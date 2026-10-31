package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"gorm.io/gorm"
)

// updateArtifactMetadata edits the descriptive record separately from file bytes.
func (s *Server) updateArtifactMetadata(c *gin.Context) {
	var req struct {
		Title       *string `json:"title" binding:"omitempty,max=500"`
		CreatorName *string `json:"creator_name" binding:"omitempty,max=255"`
		SourceURL   *string `json:"source_url" binding:"omitempty,max=4096"`
		Description *string `json:"description" binding:"omitempty,max=20000"`
		Reason      string  `json:"reason" binding:"required,min=3,max=2000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Title == nil && req.CreatorName == nil && req.SourceURL == nil && req.Description == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one valid metadata field is required"})
		return
	}
	updates := map[string]any{"updated_at": time.Now().UTC()}
	if req.Title != nil {
		if strings.TrimSpace(*req.Title) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "title cannot be empty"})
			return
		}
		updates["title"] = strings.TrimSpace(*req.Title)
	}
	if req.CreatorName != nil {
		updates["creator_name"] = trimOptional(req.CreatorName)
	}
	if req.SourceURL != nil {
		validated, err := validateUploaderURL(*req.SourceURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "source_url must be an HTTPS URL on uploader.jp"})
			return
		}
		updates["source_url"] = validated
	}
	if req.Description != nil {
		updates["description"] = trimOptional(req.Description)
	}
	db := s.db.WithContext(c.Request.Context())
	result := db.Model(&model.MArtifact{}).Where("id = ?", c.Param("id")).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update artifact metadata"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "artifact not found"})
		return
	}
	var artifact model.MArtifact
	if err := db.First(&artifact, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load updated artifact"})
		return
	}
	actor, _ := currentUser(c)
	s.audit(c, &actor, "artifact", artifact.ID, "artifact.metadata.update", "success", &req.Reason)
	c.JSON(http.StatusOK, gin.H{"artifact": artifact})
}

func (s *Server) updateFileMetadata(c *gin.Context) {
	var req struct {
		OriginalFilename *string `json:"original_filename" binding:"omitempty,max=500"`
		MediaType        *string `json:"media_type" binding:"omitempty,max=255"`
		SourceURL        *string `json:"source_url" binding:"omitempty,max=4096"`
		Reason           string  `json:"reason" binding:"required,min=3,max=2000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.OriginalFilename == nil && req.MediaType == nil && req.SourceURL == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one file metadata field and a reason are required"})
		return
	}
	updates := map[string]any{}
	if req.OriginalFilename != nil {
		if strings.TrimSpace(*req.OriginalFilename) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "original_filename cannot be empty"})
			return
		}
		updates["original_filename"] = strings.TrimSpace(*req.OriginalFilename)
	}
	if req.MediaType != nil {
		updates["media_type"] = trimOptional(req.MediaType)
	}
	if req.SourceURL != nil {
		if strings.TrimSpace(*req.SourceURL) == "" {
			updates["source_url"] = nil
		} else {
			validated, err := validateUploaderURL(*req.SourceURL)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "source_url must be an HTTPS URL on uploader.jp"})
				return
			}
			updates["source_url"] = validated
		}
	}
	var file model.MFileObject
	db := s.db.WithContext(c.Request.Context())
	if err := db.First(&file, "id = ? AND deleted_at IS NULL AND deletion_requested_at IS NULL", c.Param("id")).Error; err != nil {
		notFoundOrError(c, err, "file object")
		return
	}
	if err := db.Model(&model.MFileObject{}).Where("id = ?", file.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update file metadata"})
		return
	}
	actor, _ := currentUser(c)
	s.audit(c, &actor, "file_object", file.ID, "file_object.metadata.update", "success", &req.Reason)
	if err := db.First(&file, "id = ?", file.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load updated file"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"file": file})
}

// bulkFileAction applies one auditable action to up to 100 selected files.
func (s *Server) bulkFileAction(c *gin.Context) {
	var req struct {
		FileIDs              []string `json:"file_ids" binding:"required,min=1,max=100,dive,uuid"`
		Action               string   `json:"action" binding:"required,oneof=policy publication delete"`
		RedistributionPolicy string   `json:"redistribution_policy"`
		Enabled              *bool    `json:"enabled"`
		RequestID            *string  `json:"request_id" binding:"omitempty,uuid"`
		Reason               string   `json:"reason" binding:"required,min=3,max=2000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bulk action payload"})
		return
	}
	actor, _ := currentUser(c)
	if (req.Action == "delete" || req.Action == "publication") && actor.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "administrator role is required for this action"})
		return
	}
	if req.Action == "policy" && !redistributionPolicies[req.RedistributionPolicy] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid redistribution_policy is required"})
		return
	}
	if req.Action == "publication" && req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enabled is required for publication action"})
		return
	}
	if req.Action == "delete" && req.RequestID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deletion must be linked to an accepted deletion request"})
		return
	}
	if len(uniqueStrings(req.FileIDs)) != len(req.FileIDs) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_ids must be unique"})
		return
	}

	db := s.db.WithContext(c.Request.Context())
	var linkedRequest *model.TPublicRequest
	if req.RequestID != nil {
		var request model.TPublicRequest
		if err := db.First(&request, "id = ?", *req.RequestID).Error; err != nil {
			notFoundOrError(c, err, "request")
			return
		}
		if req.Action == "delete" && (request.RequestType != "deletion" || request.Status != "accepted") {
			c.JSON(http.StatusConflict, gin.H{"error": "delete requires an accepted deletion request"})
			return
		}
		linkedRequest = &request
	}

	changed := make([]string, 0, len(req.FileIDs))
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, id := range req.FileIDs {
			var file model.MFileObject
			if err := tx.First(&file, "id = ? AND deleted_at IS NULL", id).Error; err != nil {
				return fmt.Errorf("file %s is not available: %w", id, err)
			}
			if file.DeletionRequestedAt != nil && req.Action != "delete" {
				return fmt.Errorf("file %s is already being deleted", id)
			}
			if linkedRequest != nil {
				matches := linkedRequest.FileObjectID != nil && *linkedRequest.FileObjectID == file.ID
				if req.Action == "delete" {
					if linkedRequest.FileObjectID == nil && linkedRequest.ArtifactID != nil && *linkedRequest.ArtifactID == file.ArtifactID {
						matches = true
					}
				} else if linkedRequest.ArtifactID != nil && *linkedRequest.ArtifactID == file.ArtifactID {
					matches = true
				}
				if !matches {
					return fmt.Errorf("file %s does not match the linked request", id)
				}
			}
			switch req.Action {
			case "policy":
				updates := map[string]any{"redistribution_policy": req.RedistributionPolicy}
				if req.RedistributionPolicy != "allowed" {
					updates["public_download_enabled"] = false
				}
				if err := tx.Model(&model.MFileObject{}).Where("id = ?", file.ID).Updates(updates).Error; err != nil {
					return err
				}
				var checkedAt *time.Time
				var checkedBy *string
				if req.RedistributionPolicy == "allowed" || req.RedistributionPolicy == "restricted" {
					now := time.Now().UTC()
					checkedAt, checkedBy = &now, &actor.ID
				}
				policy := model.TPolicy{FileObjectID: file.ID, PolicyType: req.RedistributionPolicy, CheckedAt: checkedAt, CheckedBy: checkedBy}
				if err := tx.Create(&policy).Error; err != nil {
					return err
				}
			case "publication":
				if *req.Enabled {
					if file.RedistributionPolicy != "allowed" || file.PreservationStatus != "verified" || file.Sha256 == nil || !isHexDigest(*file.Sha256) {
						return fmt.Errorf("file %s cannot be published: policy must be allowed, preservation verified, and SHA-256 valid", id)
					}
					cutoff := time.Now().UTC().Add(-2 * time.Minute)
					var healthy int64
					if err := tx.Table("placements AS p").Joins("JOIN m_nodes AS n ON n.id = p.node_id").Where("p.file_object_id = ? AND p.status = 'verified' AND n.status = 'online' AND n.last_seen_at >= ?", file.ID, cutoff).Count(&healthy).Error; err != nil {
						return err
					}
					if healthy == 0 {
						return fmt.Errorf("file %s cannot be published without a healthy verified placement", id)
					}
				}
				if err := tx.Model(&model.MFileObject{}).Where("id = ?", file.ID).Update("public_download_enabled", *req.Enabled).Error; err != nil {
					return err
				}
			case "delete":
				if linkedRequest == nil {
					return errors.New("delete requires a linked request")
				}
				if err := queueFileDeletionTx(tx, file, *linkedRequest, actor.ID, req.Reason); err != nil {
					return err
				}
			}
			changed = append(changed, id)
		}
		return nil
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	reason := req.Reason
	if linkedRequest != nil {
		reason += " [request_id=" + linkedRequest.ID + "]"
	}
	for _, id := range changed {
		s.audit(c, &actor, "file_object", id, "file_object.bulk."+req.Action, "success", &reason)
	}
	c.JSON(http.StatusOK, gin.H{"changed": changed, "count": len(changed), "action": req.Action})
}

func queueFileDeletionTx(tx *gorm.DB, file model.MFileObject, request model.TPublicRequest, actorID, reason string) error {
	if request.RequestType != "deletion" || request.Status != "accepted" {
		return errors.New("file deletion requires an accepted deletion request")
	}
	if request.FileObjectID != nil {
		if *request.FileObjectID != file.ID {
			return errors.New("deletion request targets a different file")
		}
	} else if request.ArtifactID == nil || *request.ArtifactID != file.ArtifactID {
		return errors.New("deletion request must target this file or its artifact")
	}
	if value, _ := file.DeletedAt.Value(); value != nil {
		return nil
	}
	var running int64
	if err := tx.Model(&model.TTask{}).Where("file_object_id = ? AND status = 'running' AND task_type <> 'delete'", file.ID).Count(&running).Error; err != nil {
		return err
	}
	if running > 0 {
		return fmt.Errorf("file %s has a running preservation task; retry deletion after it finishes", file.ID)
	}
	if err := tx.Model(&model.TTask{}).Where("file_object_id = ? AND status = 'queued' AND task_type <> 'delete'", file.ID).Updates(map[string]any{"status": "failed", "last_error": "cancelled by accepted deletion request", "updated_at": time.Now().UTC()}).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := tx.Model(&model.MFileObject{}).Where("id = ?", file.ID).Updates(map[string]any{"public_download_enabled": false, "deletion_requested_at": now}).Error; err != nil {
		return err
	}
	var placements []model.TPlacement
	if err := tx.Where("file_object_id = ?", file.ID).Find(&placements).Error; err != nil {
		return err
	}
	pending := 0
	for _, placement := range placements {
		if placement.Status == "deleted" {
			continue
		}
		var active int64
		if err := tx.Model(&model.TTask{}).Where("file_object_id = ? AND target_node_id = ? AND task_type = 'delete' AND status IN ('queued','running')", file.ID, placement.NodeID).Count(&active).Error; err != nil {
			return err
		}
		if active == 0 {
			nodeID := placement.NodeID
			task := model.TTask{TaskType: "delete", FileObjectID: file.ID, TargetNodeID: &nodeID, Status: "queued", Priority: 5}
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.TPlacement{}).Where("id = ?", placement.ID).Update("status", "deleting").Error; err != nil {
			return err
		}
		pending++
	}
	if pending == 0 {
		if err := tx.Model(&model.MFileObject{}).Where("id = ?", file.ID).Update("deleted_at", now).Error; err != nil {
			return err
		}
	}
	auditReason := strings.TrimSpace(reason) + " [request_id=" + request.ID + "]"
	targetID := file.ID
	entry := model.TAuditLog{ActorID: &actorID, ActorLabel: "user:" + actorID, TargetType: "file_object", TargetID: &targetID, Action: "file_object.deletion.requested", Result: "success", Reason: trimOptional(&auditReason)}
	return tx.Create(&entry).Error
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

// createAdminDeletionRequests creates pending, file-linked deletion requests. It does
// not delete data; an administrator must approve the requests from the review workspace.
func (s *Server) createAdminDeletionRequests(c *gin.Context) {
	var req struct {
		FileIDs []string `json:"file_ids" binding:"required,min=1,max=100,dive,uuid"`
		Reason  string   `json:"reason" binding:"required,min=5,max=2000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_ids and a reason of at least 5 characters are required"})
		return
	}
	if len(uniqueStrings(req.FileIDs)) != len(req.FileIDs) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_ids must be unique"})
		return
	}
	actor, _ := currentUser(c)
	created := make([]model.TPublicRequest, 0, len(req.FileIDs))
	db := s.db.WithContext(c.Request.Context())
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, id := range req.FileIDs {
			var file model.MFileObject
			if err := tx.First(&file, "id = ? AND deleted_at IS NULL AND deletion_requested_at IS NULL", id).Error; err != nil {
				return fmt.Errorf("file %s is not available for deletion request: %w", id, err)
			}
			artifactID, fileID := file.ArtifactID, file.ID
			submitter := "管理者による削除申請"
			request := model.TPublicRequest{
				RequestType: "deletion", Status: "pending", ArtifactID: &artifactID,
				FileObjectID: &fileID, SubmitterName: &submitter,
				Details: strings.TrimSpace(req.Reason),
			}
			if err := tx.Create(&request).Error; err != nil {
				return err
			}
			created = append(created, request)
			targetID := request.ID
			reason := strings.TrimSpace(req.Reason) + " [file_id=" + file.ID + "]"
			entry := model.TAuditLog{ActorID: &actor.ID, ActorLabel: actor.Email, TargetType: "public_request", TargetID: &targetID, Action: "public_request.admin_deletion_request.create", Result: "success", Reason: trimOptional(&reason)}
			if err := tx.Create(&entry).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"items": created, "count": len(created), "message": "deletion requests created; approve them in the review workspace to start deletion"})
}
