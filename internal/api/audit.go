package api

import (
	"context"
	"log/slog"

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
