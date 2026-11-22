package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"github.com/nfmmu/uploader-archive/internal/hooks"
)

var errFileAlreadyReplaced = errors.New("file object has already been replaced")

// replaceFileObject registers a new file object as the replacement of an
// existing one and stops the old file from being downloaded. The old file is
// not deleted yet: its physical deletion is requested once the replacement
// reaches the verified preservation state (see recomputeFilePreservation).
func (s *Server) replaceFileObject(c *gin.Context) {
	var req fileObjectPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file object payload"})
		return
	}
	policy, err := s.validateFileObjectPayload(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	db := s.db.WithContext(c.Request.Context())
	actor, _ := hooks.CurrentUser(c)
	var created model.FileObject
	err = db.Transaction(func(tx *gorm.DB) error {
		var oldFile model.FileObject
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&oldFile, "id = ? AND deleted_at IS NULL", c.Param("id")).Error; err != nil {
			return err
		}
		if oldFile.DeletionRequestedAt != nil {
			return errFileDeletionAlreadyRequested
		}
		if oldFile.ReplacedByFileObjectID != nil {
			return errFileAlreadyReplaced
		}
		if actor.Role == "author" {
			var assignment model.AuthorArtifact
			if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).
				Where("user_id = ? AND artifact_id = ?", actor.ID, oldFile.ArtifactID).
				Take(&assignment).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errAuthorArtifactAccessDenied
				}
				return err
			}
		}
		created = newFileObjectFromPayload(oldFile.ArtifactID, req, policy)
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.FileObject{}).Where("id = ?", oldFile.ID).Updates(map[string]any{
			"replaced_by_file_object_id": created.ID,
			"replaced_by_user_id":        actor.ID,
			"public_download_enabled":    false,
		}).Error; err != nil {
			return err
		}
		targetID := oldFile.ID
		reason := "replaced by file " + created.ID
		entry := model.AuditLog{
			ActorID: &actor.ID, ActorLabel: "user:" + actor.ID,
			TargetType: "file_object", TargetID: &targetID,
			Action: "file_object.replace", Result: "success", Reason: &reason,
		}
		return tx.Create(&entry).Error
	})
	if err != nil {
		switch {
		case errors.Is(err, errFileDeletionAlreadyRequested):
			c.JSON(http.StatusConflict, gin.H{"error": "file deletion has already been requested"})
		case errors.Is(err, errFileAlreadyReplaced):
			c.JSON(http.StatusConflict, gin.H{"error": "file object has already been replaced"})
		case errors.Is(err, errAuthorArtifactAccessDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": "artifact access denied"})
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "file object not found"})
		default:
			s.logger.Error("could not replace file object", "file_id", c.Param("id"), "actor_id", actor.ID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not replace file object"})
		}
		return
	}
	if created.SourceURL != nil {
		created.SourceURL = new(uploaderSourceURLForAccess(*created.SourceURL))
	}
	c.JSON(http.StatusCreated, created)
}
