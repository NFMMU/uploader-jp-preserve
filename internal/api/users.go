package api

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Server) updateUser(c *gin.Context) {
	var req struct {
		Role        *string `json:"role" binding:"omitempty,oneof=admin reviewer viewer author"`
		Email       *string `json:"email" binding:"omitempty,email,max=320"`
		Disabled    *bool   `json:"disabled"`
		DisplayName *string `json:"display_name" binding:"omitempty,min=1,max=120"`
		CustomID    *string `json:"custom_id" binding:"omitempty,min=3,max=16"`
		Reason      *string `json:"reason" binding:"omitempty,min=3,max=2000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Role == nil && req.Disabled == nil && req.DisplayName == nil && req.CustomID == nil && req.Email == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one valid change and a reason are required"})
		return
	}
	if (req.Role != nil && strings.TrimSpace(*req.Role) == "") || (req.DisplayName != nil && strings.TrimSpace(*req.DisplayName) == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role and display_name cannot be empty"})
		return
	}
	actor, _ := currentUser(c)
	var target model.MUser
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

	if req.DisplayName != nil {
		displayName := strings.TrimSpace(*req.DisplayName)
		if utf8.RuneCountInString(displayName) < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email, display_name, password (12+ chars), custom_id (3-15 chars) and valid role are required"})
			return
		}
	}
	if req.CustomID != nil {
		customID := strings.TrimSpace(*req.CustomID)
		if utf8.RuneCountInString(customID) < 3 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email, display_name, password (12+ chars), custom_id (3-15 chars) and valid role are required"})
			return
		}
	}

	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if target.Role == "admin" && (newRole != "admin" || disable) {
			var activeAdmins []model.MUser
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("role = ? AND disabled_at IS NULL", "admin").Order("id").Find(&activeAdmins).Error; err != nil {
				return err
			}
			if len(activeAdmins) <= 1 {
				return errLastActiveAdmin
			}
		}
		updates := model.MUser{
			UpdatedAt: time.Now(),
		}
		if req.Role != nil {
			updates.Role = *req.Role
		}
		if req.DisplayName != nil {
			updates.DisplayName = strings.TrimSpace(*req.DisplayName)
		}
		if req.CustomID != nil {
			updates.CustomID = strings.TrimSpace(*req.CustomID)
		}
		if req.Email != nil {
			updates.Email = strings.ToLower(strings.TrimSpace(*req.Email))
		}
		if req.Disabled != nil {
			if *req.Disabled {
				now := time.Now().UTC()
				updates.DisabledAt = &now
			} else {
				updates.DisabledAt = nil
			}
		}
		return tx.Where("id = ?", target.ID).Updates(updates).Error
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
	s.audit(c, &actor, "user", target.ID, "user.update", "success", req.Reason)
	c.JSON(http.StatusOK, gin.H{"id": target.ID, "email": target.Email, "display_name": target.DisplayName, "role": target.Role, "disabled": target.DisabledAt != nil})
}

var errLastActiveAdmin = errors.New("last active administrator cannot be demoted or disabled")

func (s *Server) createUser(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email,max=320"`
		Password    string `json:"password" binding:"required,min=12,max=1024"`
		DisplayName string `json:"display_name" binding:"required,min=1,max=120"`
		CustomID    string `json:"custom_id" binding:"required,min=3,max=16"`
		Role        string `json:"role" binding:"required,oneof=admin reviewer viewer author"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email, display_name, password (12+ chars), custom_id (3-15 chars) and valid role are required"})
		return
	}
	actor, _ := currentUser(c)
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not hash password"})
		return
	}
	customID := strings.TrimSpace(req.CustomID)
	displayName := strings.TrimSpace(req.DisplayName)
	if utf8.RuneCountInString(customID) < 3 || utf8.RuneCountInString(displayName) < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email, display_name, password (12+ chars), custom_id (3-15 chars) and valid role are required"})
		return
	}
	user := model.MUser{Email: strings.ToLower(strings.TrimSpace(req.Email)), PasswordHash: string(hash), CustomID: customID, DisplayName: displayName, Role: req.Role}
	if err := s.db.WithContext(c.Request.Context()).Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "user could not be created; email may already exist"})
		return
	}
	s.audit(c, &actor, "user", user.ID, "user.create", "success", nil)
	c.JSON(http.StatusCreated, gin.H{"id": user.ID, "email": user.Email, "display_name": user.DisplayName, "role": user.Role, "created_at": user.CreatedAt})
}

func (s *Server) listUsers(c *gin.Context) {
	var users []model.MUser
	if err := s.db.WithContext(c.Request.Context()).Select("id", "email", "display_name", "custom_id", "role", "disabled_at", "created_at", "updated_at").Order("created_at DESC").Limit(500).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list users"})
		return
	}
	type userView struct {
		ID          string     `json:"id"`
		Email       string     `json:"email"`
		DisplayName string     `json:"display_name"`
		CustomID    string     `json:"custom_id"`
		Role        string     `json:"role"`
		DisabledAt  *time.Time `json:"disabled_at,omitempty"`
		CreatedAt   time.Time  `json:"created_at"`
		UpdatedAt   time.Time  `json:"updated_at"`
	}
	items := make([]userView, 0, len(users))
	for _, user := range users {
		items = append(items, userView{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			CustomID:    user.CustomID,
			Role:        user.Role,
			DisabledAt:  user.DisabledAt,
			CreatedAt:   user.CreatedAt,
			UpdatedAt:   user.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
