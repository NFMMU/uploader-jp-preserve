package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"github.com/nfmmu/uploader-archive/internal/hooks"
)

func (s *Server) getAuthorArtifactAssignments(c *gin.Context) {
	userID := c.Param("userID")
	var user model.User
	if err := s.db.WithContext(c.Request.Context()).Select("id").First(&user, "id = ?", userID).Error; err != nil {
		notFoundOrError(c, err, "user")
		return
	}
	var artifactIDs []string
	if err := s.db.WithContext(c.Request.Context()).Table("author_artifact").
		Where("user_id = ?", userID).Order("artifact_id").Pluck("artifact_id", &artifactIDs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load author assignments"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user_id": userID, "artifact_ids": artifactIDs})
}

func (s *Server) replaceAuthorArtifactAssignments(c *gin.Context) {
	var req struct {
		ArtifactIDs []string `json:"artifact_ids" binding:"max=1000,dive,uuid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ArtifactIDs == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "artifact_ids must be an array of UUIDs"})
		return
	}
	userID := c.Param("userID")
	if _, err := uuid.Parse(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}
	unique := uniqueStrings(req.ArtifactIDs)
	if len(unique) != len(req.ArtifactIDs) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "artifact_ids must be unique"})
		return
	}
	for _, id := range unique {
		if _, err := uuid.Parse(id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "artifact_ids must contain valid UUIDs"})
			return
		}
	}

	db := s.db.WithContext(c.Request.Context())
	err := db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "role").First(&user, "id = ?", userID).Error; err != nil {
			return err
		}
		if user.Role != "author" {
			return errAssignmentTargetNotAuthor
		}
		if len(unique) > 0 {
			var count int64
			if err := tx.Model(&model.Artifact{}).Where("id IN ?", unique).Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(unique)) {
				return errAssignmentArtifactNotFound
			}
		}
		if err := tx.Exec("DELETE FROM author_artifact WHERE user_id = ?", userID).Error; err != nil {
			return err
		}
		for _, artifactID := range unique {
			if err := tx.Exec("INSERT INTO author_artifact (user_id, artifact_id) VALUES (?, ?) ON CONFLICT DO NOTHING", userID, artifactID).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		case errors.Is(err, errAssignmentTargetNotAuthor):
			c.JSON(http.StatusConflict, gin.H{"error": "target user is not an author"})
		case errors.Is(err, errAssignmentArtifactNotFound):
			c.JSON(http.StatusBadRequest, gin.H{"error": "one or more artifacts do not exist"})
		default:
			s.logger.Error("could not replace author artifact assignments", "user_id", userID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update author assignments"})
		}
		return
	}
	actor, _ := hooks.CurrentUser(c)
	s.audit(c, &actor, "user", userID, "author_artifact.assignments.replace", "success", nil)
	c.JSON(http.StatusOK, gin.H{"user_id": userID, "artifact_ids": unique})
}

var (
	errAssignmentTargetNotAuthor  = errors.New("assignment target is not an author")
	errAssignmentArtifactNotFound = errors.New("one or more assigned artifacts do not exist")
)
