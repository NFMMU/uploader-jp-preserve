package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
)

func (s *Server) audit(c *gin.Context, actor *model.User, targetType, targetID, action, result string, reason *string) {
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	entry := model.AuditLog{ActorLabel: "anonymous", TargetType: targetType, TargetID: &targetID, Action: action, Result: result, Reason: trimOptional(reason)}
	if actor != nil {
		entry.ActorID = &actor.ID
		entry.ActorLabel = actor.Email
	}
	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		s.logger.Error("audit_log_write_failed", slog.String("action", action), slog.String("target_type", targetType), slog.String("target_id", targetID), slog.Any("error", err))
	}
}

func (s *Server) listAuditLogs(c *gin.Context) {
	page, limit, isAll := pagination(c)
	var total int64
	if err := s.db.WithContext(c.Request.Context()).Model(&model.AuditLog{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not count audit logs"})
		return
	}
	type auditLogView struct {
		model.AuditLog   `gorm:"embedded"`
		ActorDisplayName *string `json:"actor_display_name,omitempty"`
	}
	var logs []auditLogView
	query := s.db.WithContext(c.Request.Context()).
		Model(&model.AuditLog{}).
		Select("audit_logs.*, users.display_name AS actor_display_name").
		Joins("LEFT JOIN users ON users.id = audit_logs.actor_id").
		Order("audit_logs.created_at DESC")

	if !isAll {
		query = query.Offset((page - 1) * limit).Limit(limit)
	}
	if err := query.Scan(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list audit logs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": logs, "page": page, "limit": limit, "total": total})
}
