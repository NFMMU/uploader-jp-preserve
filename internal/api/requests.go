package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"gorm.io/gorm"
)

var requestTypes = map[string]bool{"takedown": true, "deletion": true, "external_publication": true, "preservation": true}
var requestStatuses = map[string]bool{"pending": true, "in_review": true, "accepted": true, "rejected": true, "closed": true}

func (s *Server) submitPublicRequest(c *gin.Context) {
	var req struct {
		RequestType   string  `json:"request_type" binding:"required"`
		ArtifactID    *string `json:"artifact_id" binding:"omitempty,uuid"`
		FileObjectID  *string `json:"file_object_id" binding:"omitempty,uuid"`
		Title         *string `json:"title" binding:"omitempty,max=500"`
		CreatorName   *string `json:"creator_name" binding:"omitempty,max=255"`
		SourceURL     *string `json:"source_url" binding:"omitempty,max=4096"`
		SubmitterName *string `json:"submitter_name" binding:"omitempty,max=255"`
		Contact       *string `json:"contact" binding:"omitempty,max=500"`
		Details       string  `json:"details" binding:"required,min=5,max=10000"`
		ExternalURL   *string `json:"external_url" binding:"omitempty,max=4096"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !requestTypes[req.RequestType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid request_type and details are required"})
		return
	}
	if req.RequestType == "preservation" {
		if req.SourceURL == nil || strings.TrimSpace(*req.SourceURL) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "source_url is required for preservation requests"})
			return
		}
		validated, err := normalizeUploaderURL(*req.SourceURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "source_url must be an HTTPS uploader.jp/getuploader.com URL"})
			return
		}
		req.SourceURL = &validated
		if req.Title == nil || strings.TrimSpace(*req.Title) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "title is required for preservation requests"})
			return
		}
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
		Title: trimOptional(req.Title), CreatorName: trimOptional(req.CreatorName), SourceURL: trimOptional(req.SourceURL),
		SubmitterName: trimOptional(req.SubmitterName), Contact: trimOptional(req.Contact), Details: strings.TrimSpace(req.Details), ExternalURL: trimOptional(req.ExternalURL),
	}
	if req.RequestType == "preservation" && req.SourceURL != nil {
		var existingFile model.FileObject
		if err := s.db.WithContext(c.Request.Context()).Where("split_part(trim(source_url), '#', 1) = ? AND deleted_at IS NULL", *req.SourceURL).First(&existingFile).Error; err == nil {
			c.JSON(http.StatusOK, gin.H{"duplicate": true, "file_object_id": existingFile.ID, "message": "このURLはすでにカタログに登録されているため、重複申請を破棄しました。"})
			return
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check existing source URL"})
			return
		}
		var existingRequest model.PublicRequest
		if err := s.db.WithContext(c.Request.Context()).Where("request_type = 'preservation' AND split_part(trim(source_url), '#', 1) = ?", *req.SourceURL).Order("created_at ASC").First(&existingRequest).Error; err == nil {
			c.JSON(http.StatusOK, gin.H{"duplicate": true, "request_id": existingRequest.ID, "message": "同じURLの保全リクエストがすでに存在するため、重複リクエストを破棄しました。"})
			return
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check existing preservation requests"})
			return
		}
	}

	if err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		if request.RequestType == "preservation" && request.SourceURL != nil {
			result := tx.Exec(`INSERT INTO source_url_registry (normalized_url, source_url, origin, public_request_id, created_at, updated_at)
				VALUES (?, ?, 'public_request', ?, now(), now()) ON CONFLICT (normalized_url) DO NOTHING`, *request.SourceURL, *request.SourceURL, request.ID)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return errDuplicateSourceURL
			}
		}
		return nil
	}); err != nil {
		if errors.Is(err, errDuplicateSourceURL) {
			var existing struct {
				PublicRequestID *string `gorm:"column:public_request_id"`
			}
			_ = s.db.WithContext(c.Request.Context()).Table("source_url_registry").Select("public_request_id").Where("normalized_url = ?", *request.SourceURL).Scan(&existing).Error
			c.JSON(http.StatusOK, gin.H{"duplicate": true, "request_id": existing.PublicRequestID, "message": "同じURLの申請・収集リクエストがすでに存在するため、重複リクエストを破棄しました。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not submit request"})
		return
	}
	s.audit(c, nil, "public_request", request.ID, "public_request.submit", "success", nil)
	c.JSON(http.StatusCreated, gin.H{"id": request.ID, "request_type": request.RequestType, "status": request.Status, "created_at": request.CreatedAt})
}

type publicRequestListItem struct {
	model.PublicRequest
	LinkedFileName      string `json:"linked_file_name,omitempty"`
	LinkedArtifactTitle string `json:"linked_artifact_title,omitempty"`
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
	if err := query.Order("CASE WHEN status = 'pending' THEN 0 WHEN status = 'in_review' THEN 1 ELSE 2 END, created_at ASC").Offset((page - 1) * limit).Limit(limit).Find(&requests).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list requests"})
		return
	}
	items := make([]publicRequestListItem, 0, len(requests))
	for _, request := range requests {
		item := publicRequestListItem{PublicRequest: request}
		artifactID := request.ArtifactID
		if request.FileObjectID != nil {
			var file model.FileObject
			if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ?", *request.FileObjectID).Error; err == nil {
				item.LinkedFileName = file.OriginalFilename
				if artifactID == nil {
					artifactID = &file.ArtifactID
				}
			}
		}
		if artifactID != nil {
			var artifact model.Artifact
			if err := s.db.WithContext(c.Request.Context()).First(&artifact, "id = ?", *artifactID).Error; err == nil {
				item.LinkedArtifactTitle = artifact.Title
			}
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "page": page, "limit": limit, "total": total})
}

func (s *Server) reviewPublicRequest(c *gin.Context) {
	var req struct {
		Status       string  `json:"status" binding:"required"`
		Reason       string  `json:"reason" binding:"required,min=3,max=2000"`
		ArtifactID   *string `json:"artifact_id" binding:"omitempty,uuid"`
		FileObjectID *string `json:"file_object_id" binding:"omitempty,uuid"`
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
	if req.ArtifactID != nil {
		var count int64
		if err := s.db.WithContext(c.Request.Context()).Model(&model.Artifact{}).Where("id = ?", *req.ArtifactID).Count(&count).Error; err != nil || count == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "artifact_id does not exist"})
			return
		}
	}
	if req.FileObjectID != nil {
		query := s.db.WithContext(c.Request.Context()).Model(&model.FileObject{}).Where("id = ?", *req.FileObjectID)
		linkedArtifactID := req.ArtifactID
		if linkedArtifactID == nil {
			linkedArtifactID = publicRequest.ArtifactID
		}
		if linkedArtifactID != nil {
			query = query.Where("artifact_id = ?", *linkedArtifactID)
		}
		var count int64
		if err := query.Count(&count).Error; err != nil || count == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file_object_id does not exist or does not belong to artifact_id"})
			return
		}
	}
	if req.Status == "accepted" && publicRequest.RequestType == "deletion" && req.FileObjectID == nil && publicRequest.FileObjectID == nil && req.ArtifactID == nil && publicRequest.ArtifactID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a deletion request must be linked to a file or artifact before acceptance"})
		return
	}
	actor, _ := currentUser(c)
	if req.Status == "accepted" && publicRequest.RequestType == "deletion" && actor.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "administrator role is required to approve deletion requests"})
		return
	}
	now := time.Now().UTC()
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"status": req.Status, "reviewed_by": actor.ID, "reviewed_at": now, "updated_at": now}
		if req.ArtifactID != nil {
			updates["artifact_id"] = *req.ArtifactID
		}
		if req.FileObjectID != nil {
			updates["file_object_id"] = *req.FileObjectID
		}
		if err := tx.Model(&model.PublicRequest{}).Where("id = ?", publicRequest.ID).Updates(updates).Error; err != nil {
			return err
		}
		// Takedown requests immediately stop public access. Accepted deletion requests
		// additionally enqueue physical deletion tasks for every linked file.
		if req.Status == "accepted" && (publicRequest.RequestType == "takedown" || publicRequest.RequestType == "deletion") {
			targetFileID := publicRequest.FileObjectID
			if req.FileObjectID != nil {
				targetFileID = req.FileObjectID
			}
			targetArtifactID := publicRequest.ArtifactID
			if req.ArtifactID != nil {
				targetArtifactID = req.ArtifactID
			}
			if targetFileID != nil {
				if err := tx.Model(&model.FileObject{}).Where("id = ?", *targetFileID).Update("public_download_enabled", false).Error; err != nil {
					return err
				}
			} else if publicRequest.RequestType == "takedown" && targetArtifactID != nil {
				if err := tx.Model(&model.FileObject{}).Where("artifact_id = ? AND deleted_at IS NULL", *targetArtifactID).Update("public_download_enabled", false).Error; err != nil {
					return err
				}
			}
			if publicRequest.RequestType == "deletion" {
				nextRequest := publicRequest
				nextRequest.Status = req.Status
				if req.ArtifactID != nil {
					nextRequest.ArtifactID = req.ArtifactID
				}
				if req.FileObjectID != nil {
					nextRequest.FileObjectID = req.FileObjectID
				}
				var files []model.FileObject
				fileQuery := tx.Where("deleted_at IS NULL")
				if nextRequest.FileObjectID != nil {
					fileQuery = fileQuery.Where("id = ?", *nextRequest.FileObjectID)
				} else if nextRequest.ArtifactID != nil {
					fileQuery = fileQuery.Where("artifact_id = ?", *nextRequest.ArtifactID)
				} else {
					return nil
				}
				if err := fileQuery.Find(&files).Error; err != nil {
					return err
				}
				for _, file := range files {
					if err := queueFileDeletionTx(tx, file, nextRequest, actor.ID, req.Reason); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "running preservation task") || strings.Contains(err.Error(), "already being deleted") {
			statusCode = http.StatusConflict
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}
	publicRequest.Status = req.Status
	if req.ArtifactID != nil {
		publicRequest.ArtifactID = req.ArtifactID
	}
	if req.FileObjectID != nil {
		publicRequest.FileObjectID = req.FileObjectID
	}
	publicRequest.ReviewedBy = &actor.ID
	publicRequest.ReviewedAt = &now
	s.audit(c, &actor, "public_request", publicRequest.ID, "public_request.review", "success", &req.Reason)
	if req.Status == "accepted" && publicRequest.RequestType == "deletion" {
		s.audit(c, &actor, "public_request", publicRequest.ID, "public_request.deletion.queued", "success", &req.Reason)
	}
	c.JSON(http.StatusOK, gin.H{"request": publicRequest, "note": "Accepted takedown requests disable public downloads. Accepted deletion requests enqueue node-agent deletion tasks; the catalog entry is hidden after all placements report deletion success."})
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
