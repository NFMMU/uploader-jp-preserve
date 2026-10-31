package api

import (
	"errors"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var customIdRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,16}$`)

func (s *Server) updateUser(c *gin.Context) {
	var req struct {
		Role        *string `json:"role" binding:"omitempty,oneof=admin reviewer viewer author"`
		Email       *string `json:"email" binding:"omitempty,email,max=320"`
		Disabled    *bool   `json:"disabled"`
		DisplayName *string `json:"display_name" binding:"omitempty,min=1,max=120"`
		CustomID    *string `json:"custom_id" binding:"omitempty,min=3,max=16"`
		Password    *string `json:"password" binding:"omitempty,min=12,max=1024"`
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
		if !customIdRegex.MatchString(customID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Custom ID must be 3–16 characters long and contain only lowercase letters, numbers, hyphens (-), and underscores (_)"})
			return
		}
	}

	var hash *string
	if req.Password != nil {
		hashRaw, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not hash password"})
			return
		}
		hashRawStrng := string(hashRaw)
		hash = &hashRawStrng
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
		updates := map[string]interface{}{
			"updated_at": time.Now(),
		}
		if req.Role != nil {
			updates["role"] = *req.Role
		}
		if req.DisplayName != nil {
			updates["display_name"] = strings.TrimSpace(*req.DisplayName)
		}
		if req.CustomID != nil {
			updates["custom_id"] = strings.TrimSpace(*req.CustomID)
		}
		if req.Email != nil {
			updates["email"] = strings.ToLower(strings.TrimSpace(*req.Email))
		}
		if hash != nil {
			updates["password_hash"] = *hash
		}
		if req.Disabled != nil {
			if *req.Disabled {
				updates["disabled_at"] = time.Now().UTC()
			} else {
				updates["disabled_at"] = nil
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
	displayName := strings.TrimSpace(req.DisplayName)
	if utf8.RuneCountInString(displayName) < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email, display_name, password (12+ chars), custom_id (3-15 chars) and valid role are required"})
		return
	}
	customID := strings.TrimSpace(req.CustomID)
	if !customIdRegex.MatchString(customID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Custom ID must be 3–16 characters long and contain only lowercase letters, numbers, hyphens (-), and underscores (_)"})
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
	page, limit, all := pagination(c)

	ctx := s.db.WithContext(c.Request.Context())

	var users []model.MUser
	var total int64

	if err := ctx.Model(&model.MUser{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not count users"})
		return
	}

	query := ctx.
		Select(
			"id",
			"email",
			"display_name",
			"custom_id",
			"role",
			"disabled_at",
			"created_at",
			"updated_at",
		).
		Order("created_at DESC")

	if !all {
		offset := (page - 1) * limit
		query = query.Offset(offset).Limit(limit)
	}

	if err := query.Find(&users).Error; err != nil {
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

	response := gin.H{
		"items": items,
		"total": total,
	}

	if !all {
		totalPages := int(math.Ceil(float64(total) / float64(limit)))

		response["page"] = page
		response["limit"] = limit
		response["total_pages"] = totalPages
	} else {
		response["all"] = true
	}

	c.JSON(http.StatusOK, response)
}
