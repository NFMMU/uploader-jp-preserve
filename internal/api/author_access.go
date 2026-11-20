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

var (
	errAuthorArtifactAccessDenied   = errors.New("author artifact access denied")
	errFileDeletionAlreadyRequested = errors.New("file deletion has already been requested")
)

// authorArtifactScope returns a query scope that limits authors to artifacts
// explicitly assigned by an administrator. Other roles are not filtered here.
func authorArtifactScope(db *gorm.DB, userID string) *gorm.DB {
	return db.Where("artifacts.id IN (SELECT artifact_id FROM author_artifact WHERE user_id = ?)", userID)
}

func (s *Server) authorHasArtifact(ctx *gin.Context, userID, artifactID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx.Request.Context()).Table("author_artifact").
		Where("user_id = ? AND artifact_id = ?", userID, artifactID).
		Count(&count).Error
	return count > 0, err
}

// requireAuthorFileArtifactAccess limits author file mutations to files belonging
// to artifacts explicitly assigned to the current author.
func (s *Server) requireAuthorFileArtifactAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := hooks.CurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if user.Role != "author" {
			c.Next()
			return
		}
		var file model.FileObject
		if err := s.db.WithContext(c.Request.Context()).
			Select("id", "artifact_id").
			First(&file, "id = ? AND deleted_at IS NULL", c.Param("id")).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "file object not found"})
			} else {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "could not verify file access"})
			}
			return
		}
		allowed, err := s.authorHasArtifact(c, user.ID, file.ArtifactID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "could not verify artifact access"})
			return
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "artifact access denied"})
			return
		}
		c.Next()
	}
}

func (s *Server) requireAuthorArtifactAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := hooks.CurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if user.Role != "author" {
			c.Next()
			return
		}
		allowed, err := s.authorHasArtifact(c, user.ID, c.Param("id"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "could not verify artifact access"})
			return
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "artifact access denied"})
			return
		}
		c.Next()
	}
}

// deleteFileObject queues physical deletion on every node that stores the file.
// The file remains auditable until every known placement reports successful deletion.
func (s *Server) deleteFileObject(c *gin.Context) {
	actor, ok := hooks.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	if actor.Role != "admin" && actor.Role != "author" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient role"})
		return
	}

	db := s.db.WithContext(c.Request.Context())
	err := db.Transaction(func(tx *gorm.DB) error {
		var file model.FileObject
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&file, "id = ? AND deleted_at IS NULL", c.Param("id")).Error; err != nil {
			return err
		}
		if actor.Role == "author" {
			var assignment model.AuthorArtifact
			if err := tx.Table("author_artifact").
				Clauses(clause.Locking{Strength: "SHARE"}).
				Where("user_id = ? AND artifact_id = ?", actor.ID, file.ArtifactID).
				Take(&assignment).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errAuthorArtifactAccessDenied
				}
				return err
			}
		}
		if file.DeletionRequestedAt != nil {
			return errFileDeletionAlreadyRequested
		}
		return queueFileDeletionTx(tx, file, nil, actor.ID, "Manual deletion from the catalog")
	})
	if err != nil {
		switch {
		case errors.Is(err, errAuthorArtifactAccessDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": "artifact access denied"})
		case errors.Is(err, errFileDeletionAlreadyRequested):
			c.JSON(http.StatusConflict, gin.H{"error": "file deletion has already been requested"})
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "file object not found"})
		default:
			s.logger.Error("could not delete file object", "file_id", c.Param("id"), "actor_id", actor.ID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete file object"})
		}
		return
	}

	s.audit(c, &actor, "file_object", c.Param("id"), "file_object.delete", "success", nil)
	c.Status(http.StatusAccepted)
}
