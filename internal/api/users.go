package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"github.com/nfmmu/uploader-archive/internal/hooks"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Server) listUsers(c *gin.Context) {
	var users []model.User
	if err := s.db.WithContext(c.Request.Context()).Select("id", "email", "display_name", "role", "disabled_at", "created_at", "updated_at").Order("created_at DESC").Limit(500).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list users"})
		return
	}
	type userView struct {
		ID          string     `json:"id"`
		Email       string     `json:"email"`
		DisplayName string     `json:"display_name"`
		Role        string     `json:"role"`
		DisabledAt  *time.Time `json:"disabled_at,omitempty"`
		CreatedAt   time.Time  `json:"created_at"`
		UpdatedAt   time.Time  `json:"updated_at"`
	}
	items := make([]userView, 0, len(users))
	for _, user := range users {
		items = append(items, userView{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName, Role: user.Role, DisabledAt: user.DisabledAt, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) createUser(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email,max=320"`
		Password    string `json:"password" binding:"required,min=12,max=1024"`
		DisplayName string `json:"display_name" binding:"required,min=1,max=120"`
		Role        string `json:"role" binding:"required,oneof=admin reviewer viewer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email, display_name, password (12+ chars), and valid role are required"})
		return
	}
	actor, _ := hooks.CurrentUser(c)
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not hash password"})
		return
	}
	user := model.User{Email: strings.ToLower(strings.TrimSpace(req.Email)), PasswordHash: string(hash), DisplayName: strings.TrimSpace(req.DisplayName), Role: req.Role}
	if err := s.db.WithContext(c.Request.Context()).Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "user could not be created; email may already exist"})
		return
	}
	s.audit(c, &actor, "user", user.ID, "user.create", "success", nil)
	c.JSON(http.StatusCreated, gin.H{"id": user.ID, "email": user.Email, "display_name": user.DisplayName, "role": user.Role, "created_at": user.CreatedAt})
}

func (s *Server) updateUser(c *gin.Context) {
	var req struct {
		Role        string `json:"role" binding:"required,oneof=admin reviewer viewer"`
		Disabled    *bool  `json:"disabled" binding:"required"`
		DisplayName string `json:"display_name" binding:"required,min=1,max=120"`
		Reason      string `json:"reason" binding:"required,min=3,max=2000"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role, disabled, display_name, and reason are required and must be valid"})
		return
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "display_name cannot be empty"})
		return
	}

	user, ok := hooks.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var target model.User
	if err := s.db.WithContext(c.Request.Context()).First(&target, "id = ?", c.Param("id")).Error; err != nil {
		notFoundOrError(c, err, "user")
		return
	}

	// Role と Disabled の変更は admin のみ許可
	if user.Role != "admin" {
		if req.Role != target.Role || *req.Disabled != (target.DisabledAt != nil) {
			c.JSON(http.StatusForbidden, gin.H{"error": "only administrators can change role or disabled status"})
			return
		}
	}

	// 自分自身の管理者権限剥奪・無効化を禁止
	if target.ID == user.ID && (req.Role != "admin" || *req.Disabled) {
		c.JSON(http.StatusConflict, gin.H{"error": "cannot remove administrator access from or disable the current account"})
		return
	}

	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if target.Role == "admin" && (req.Role != "admin" || *req.Disabled) {
			var activeAdmins []model.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Select("id").
				Where("role = ? AND disabled_at IS NULL", "admin").
				Order("id").
				Find(&activeAdmins).Error; err != nil {
				return err
			}

			if len(activeAdmins) <= 1 {
				return errLastActiveAdmin
			}
		}

		now := time.Now().UTC()
		updates := map[string]any{
			"role":         req.Role,
			"display_name": displayName,
			"disabled_at":  nil,
			"updated_at":   now,
		}

		if *req.Disabled {
			updates["disabled_at"] = now
		}

		return tx.Model(&model.User{}).
			Where("id = ?", target.ID).
			Updates(updates).Error
	})

	if err != nil {
		if errors.Is(err, errLastActiveAdmin) {
			c.JSON(http.StatusConflict, gin.H{"error": "cannot disable or demote the last active administrator"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update user"})
		return
	}

	target.Role = req.Role
	target.DisplayName = displayName
	target.DisabledAt = nil
	if *req.Disabled {
		now := time.Now().UTC()
		target.DisabledAt = &now
	}

	s.audit(c, &user, "user", target.ID, "user.update", "success", &req.Reason)

	c.JSON(http.StatusOK, gin.H{
		"id":           target.ID,
		"email":        target.Email,
		"display_name": target.DisplayName,
		"role":         target.Role,
		"disabled":     target.DisabledAt != nil,
	})
}

func (s *Server) updatePassword(c *gin.Context) {
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
		Reason          string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	actor, ok := hooks.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	// パスワードの長さを検証する。
	// bcrypt は UTF-8 バイト列で72バイトを超えるパスワードを扱えない。
	if len([]rune(req.NewPassword)) < 12 || len([]byte(req.NewPassword)) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "new password must be at least 12 characters and at most 72 bytes",
		})
		return
	}

	if req.NewPassword != req.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "new password and confirmation do not match",
		})
		return
	}

	targetID := c.Param("id")
	isSelf := actor.ID == targetID

	// 他人のパスワードを変更できるのは admin のみ。
	if !isSelf && actor.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "insufficient permissions",
		})
		return
	}

	// 本人による変更では現在のパスワードを要求する。
	// 他人への管理者リセットでは理由を必須とする。
	if isSelf {
		if req.CurrentPassword == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "current_password is required",
			})
			return
		}
	} else {
		req.Reason = strings.TrimSpace(req.Reason)
		if len(req.Reason) < 3 || len(req.Reason) > 2000 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "reason must be between 3 and 2000 characters",
			})
			return
		}
	}

	// 対象ユーザーを取得する。
	var target model.User
	if err := s.db.WithContext(c.Request.Context()).
		Select("id", "password_hash").
		First(&target, "id = ?", targetID).Error; err != nil {
		notFoundOrError(c, err, "user")
		return
	}

	// 本人による変更の場合は現在のパスワードを検証する。
	if isSelf {
		if err := bcrypt.CompareHashAndPassword(
			[]byte(target.PasswordHash),
			[]byte(req.CurrentPassword),
		); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "current password is incorrect",
			})
			return
		}
	}

	// 現在のパスワードと同じものへの変更を禁止する。
	if isSelf {
		if err := bcrypt.CompareHashAndPassword(
			[]byte(target.PasswordHash),
			[]byte(req.NewPassword),
		); err == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "new password must be different from current password",
			})
			return
		}
	}

	// 新しいパスワードをハッシュ化する。
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(req.NewPassword),
		bcrypt.DefaultCost,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to hash password",
		})
		return
	}

	// パスワードを更新する。
	result := s.db.WithContext(c.Request.Context()).
		Model(&model.User{}).
		Where("id = ?", target.ID).
		Updates(map[string]any{
			"password_hash": string(hash),
			"updated_at":    time.Now().UTC(),
		})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update password",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "user not found",
		})
		return
	}

	// 操作を監査ログに記録する。
	action := "user.password.change"
	var reason *string

	if !isSelf {
		action = "user.password.reset"
		reason = &req.Reason
	}

	s.audit(
		c,
		&actor,
		"user",
		target.ID,
		action,
		"success",
		reason,
	)

	c.JSON(http.StatusOK, gin.H{
		"id":      target.ID,
		"message": "password updated successfully",
	})
}

var errLastActiveAdmin = errors.New("last active administrator cannot be demoted or disabled")
