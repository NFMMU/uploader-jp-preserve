package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/uploader-jp/uploader-archive/internal/gen/model"
	"gorm.io/gorm"
)

var requestTypes = map[string]bool{"takedown": true, "deletion": true, "external_publication": true}
var requestStatuses = map[string]bool{"pending": true, "in_review": true, "accepted": true, "rejected": true, "closed": true}

func (s *Server) submitPublicRequest(c *gin.Context) {
	var req struct {
		RequestType   string  `json:"request_type" binding:"required"`
		ArtifactID    *string `json:"artifact_id" binding:"omitempty,uuid"`
		FileObjectID  *string `json:"file_object_id" binding:"omitempty,uuid"`
		SubmitterName *string `json:"submitter_name" binding:"omitempty,max=255"`
		Contact       *string `json:"contact" binding:"omitempty,max=500"`
		Details       string  `json:"details" binding:"required,min=5,max=10000"`
		ExternalURL   *string `json:"external_url" binding:"omitempty,max=4096"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !requestTypes[req.RequestType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid request_type and details are required"})
		return
	}
	if req.RequestType == "external_publication" {
		if req.ExternalURL == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "external_url is required for external_publication requests"})
			return
		}
		validated, err := validateHTTPURL(*req.ExternalURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "external_url must be an http(s) URL"})
			return
		}
		req.ExternalURL = &validated
	}
	if req.ArtifactID != nil {
		var count int64
		if err := s.db.WithContext(c.Request.Context()).Model(&model.Artifact{}).Where("id = ?", *req.ArtifactID).Count(&count).Error; err != nil || count == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "artifact_id does not exist"})
			return
		}
	}
	if req.FileObjectID != nil {
		var count int64
		if err := s.db.WithContext(c.Request.Context()).Model(&model.FileObject{}).Where("id = ?", *req.FileObjectID).Count(&count).Error; err != nil || count == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file_object_id does not exist"})
			return
		}
	}
	request := model.PublicRequest{
		RequestType: req.RequestType, Status: "pending", ArtifactID: req.ArtifactID, FileObjectID: req.FileObjectID,
		SubmitterName: trimOptional(req.SubmitterName), Contact: trimOptional(req.Contact), Details: strings.TrimSpace(req.Details), ExternalURL: trimOptional(req.ExternalURL),
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&request).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not submit request"})
		return
	}
	s.audit(c, nil, "public_request", request.ID, "public_request.submit", "success", nil)
	c.JSON(http.StatusCreated, gin.H{"id": request.ID, "request_type": request.RequestType, "status": request.Status, "created_at": request.CreatedAt})
}

func (s *Server) listPublicRequests(c *gin.Context) {
	page, limit := pagination(c)
	query := s.db.WithContext(c.Request.Context()).Model(&model.PublicRequest{})
	if status := c.Query("status"); status != "" {
		if !requestStatuses[status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not count requests"})
		return
	}
	var requests []model.PublicRequest
	if err := query.Order("created_at ASC").Offset((page - 1) * limit).Limit(limit).Find(&requests).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list requests"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": requests, "page": page, "limit": limit, "total": total})
}

func (s *Server) reviewPublicRequest(c *gin.Context) {
	var req struct {
		Status string `json:"status" binding:"required"`
		Reason string `json:"reason" binding:"required,min=3,max=2000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !requestStatuses[req.Status] || req.Status == "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid review status and reason are required"})
		return
	}
	var publicRequest model.PublicRequest
	if err := s.db.WithContext(c.Request.Context()).First(&publicRequest, "id = ?", c.Param("id")).Error; err != nil {
		notFoundOrError(c, err, "public request")
		return
	}
	actor, _ := currentUser(c)
	now := time.Now().UTC()
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"status": req.Status, "reviewed_by": actor.ID, "reviewed_at": now, "updated_at": now}
		if err := tx.Model(&model.PublicRequest{}).Where("id = ?", publicRequest.ID).Updates(updates).Error; err != nil {
			return err
		}
		// Stop serving immediately when a takedown/deletion request is accepted.
		if req.Status == "accepted" && publicRequest.FileObjectID != nil && (publicRequest.RequestType == "takedown" || publicRequest.RequestType == "deletion") {
			return tx.Model(&model.FileObject{}).Where("id = ?", *publicRequest.FileObjectID).Update("public_download_enabled", false).Error
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not review request"})
		return
	}
	publicRequest.Status = req.Status
	publicRequest.ReviewedBy = &actor.ID
	publicRequest.ReviewedAt = &now
	s.audit(c, &actor, "public_request", publicRequest.ID, "public_request.review", "success", &req.Reason)
	c.JSON(http.StatusOK, gin.H{"request": publicRequest, "note": "Accepted takedown/deletion requests disable the public-download flag immediately; node deletion tasks are not implemented in MVP-1."})
}

func (s *Server) listAuditLogs(c *gin.Context) {
	page, limit := pagination(c)
	var total int64
	if err := s.db.WithContext(c.Request.Context()).Model(&model.AuditLog{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not count audit logs"})
		return
	}
	var logs []model.AuditLog
	if err := s.db.WithContext(c.Request.Context()).Order("created_at DESC").Offset((page - 1) * limit).Limit(limit).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list audit logs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": logs, "page": page, "limit": limit, "total": total})
}
