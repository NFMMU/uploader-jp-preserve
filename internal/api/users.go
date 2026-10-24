package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Server) updateUser(c *gin.Context) {
	var req struct {
		Role        *string `json:"role" binding:"omitempty,oneof=admin reviewer viewer"`
		Disabled    *bool   `json:"disabled"`
		DisplayName *string `json:"display_name" binding:"omitempty,min=1,max=120"`
		Reason      string  `json:"reason" binding:"required,min=3,max=2000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Role == nil && req.Disabled == nil && req.DisplayName == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one valid change and a reason are required"})
		return
	}
	if (req.Role != nil && strings.TrimSpace(*req.Role) == "") || (req.DisplayName != nil && strings.TrimSpace(*req.DisplayName) == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role and display_name cannot be empty"})
		return
	}
	actor, _ := currentUser(c)
	var target model.User
	if err := s.db.WithContext(c.Request.Context()).First(&target, "id = ?", c.Param("id")).Error; err != nil {
		notFoundOrError(c, err, "user")
		return
	}
	if target.ID == actor.ID && ((req.Role != nil && *req.Role != "admin") || (req.Disabled != nil && *req.Disabled)) {
		c.JSON(http.StatusConflict, gin.H{"error": "cannot remove administrator access from or disable the current account"})
		return
	}
	newRole := target.Role
	if req.Role != nil {
		newRole = *req.Role
	}
	disable := target.DisabledAt != nil
	if req.Disabled != nil {
		disable = *req.Disabled
	}
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if target.Role == "admin" && (newRole != "admin" || disable) {
			var activeAdmins []model.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("role = ? AND disabled_at IS NULL", "admin").Order("id").Find(&activeAdmins).Error; err != nil {
				return err
			}
			if len(activeAdmins) <= 1 {
				return errLastActiveAdmin
			}
		}
		updates := map[string]any{"updated_at": time.Now().UTC()}
		if req.Role != nil {
			updates["role"] = *req.Role
		}
		if req.DisplayName != nil {
			updates["display_name"] = strings.TrimSpace(*req.DisplayName)
		}
		if req.Disabled != nil {
			if *req.Disabled {
				now := time.Now().UTC()
				updates["disabled_at"] = &now
			} else {
				updates["disabled_at"] = nil
			}
		}
		return tx.Model(&model.User{}).Where("id = ?", target.ID).Updates(updates).Error
	})
	if err != nil {
		if err == errLastActiveAdmin {
			c.JSON(http.StatusConflict, gin.H{"error": "cannot disable or demote the last active administrator"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update user"})
		return
	}
	if req.Role != nil {
		target.Role = *req.Role
	}
	if req.DisplayName != nil {
		target.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.Disabled != nil {
		if *req.Disabled {
			now := time.Now().UTC()
			target.DisabledAt = &now
		} else {
			target.DisabledAt = nil
		}
	}
	s.audit(c, &actor, "user", target.ID, "user.update", "success", &req.Reason)
	c.JSON(http.StatusOK, gin.H{"id": target.ID, "email": target.Email, "display_name": target.DisplayName, "role": target.Role, "disabled": target.DisabledAt != nil})
}

var errLastActiveAdmin = errors.New("last active administrator cannot be demoted or disabled")
