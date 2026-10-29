package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxIngestBytes    = int64(5 << 30) // 5 GiB per request in this MVP.
	maxTaskAttempts   = 5
	nodeLeaseDuration = 30 * time.Minute
)

type nodeContext struct {
	Node model.Node
}

func hashNodeToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newNodeToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Server) registerNode(c *gin.Context) {
	var req struct {
		Name                   string `json:"name" binding:"required,min=1,max=120"`
		NodeType               string `json:"node_type" binding:"required,oneof=A B"`
		RequestedCapacityBytes int64  `json:"requested_capacity_bytes" binding:"gte=0"`
		AgentVersion           string `json:"agent_version" binding:"required,max=64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, node_type (A or B), requested_capacity_bytes, and agent_version are required"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.AgentVersion = strings.TrimSpace(req.AgentVersion)
	enrollment := strings.TrimSpace(c.GetHeader("X-Node-Enrollment-Token"))
	if enrollment == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "one-time enrollment token required"})
		return
	}
	// Shared-token registration exists only for local development. Production nodes
	// must use an approved, one-time token issued for a specific enrollment request.
	if s.cfg.Environment == "development" && s.cfg.DevNodeEnrollmentToken != "" && len(enrollment) == len(s.cfg.DevNodeEnrollmentToken) && subtle.ConstantTimeCompare([]byte(enrollment), []byte(s.cfg.DevNodeEnrollmentToken)) == 1 {
		s.registerDevelopmentNode(c, req)
		return
	}
	s.registerApprovedNode(c, req)
}

func (s *Server) registerDevelopmentNode(c *gin.Context, req struct {
	Name                   string `json:"name" binding:"required,min=1,max=120"`
	NodeType               string `json:"node_type" binding:"required,oneof=A B"`
	RequestedCapacityBytes int64  `json:"requested_capacity_bytes" binding:"gte=0"`
	AgentVersion           string `json:"agent_version" binding:"required,max=64"`
}) {
	token, err := newNodeToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not issue node credential"})
		return
	}
	node := model.Node{Name: req.Name, NodeType: req.NodeType, Status: "offline", RequestedCapacityBytes: req.RequestedCapacityBytes, AgentVersion: stringPtr(req.AgentVersion)}
	if err := s.db.WithContext(c.Request.Context()).Create(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not register development node"})
		return
	}
	if err := s.db.WithContext(c.Request.Context()).Exec("UPDATE nodes SET api_token_hash = ?, updated_at = now() WHERE id = ?", hashNodeToken(token), node.ID).Error; err != nil {
		_ = s.db.Delete(&node).Error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not store node credential"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"node": gin.H{"id": node.ID, "name": node.Name, "node_type": node.NodeType, "status": node.Status, "requested_capacity_bytes": node.RequestedCapacityBytes}, "node_token": token, "warning": "development-only node registration; store credentials securely"})
}

func (s *Server) registerApprovedNode(c *gin.Context, req struct {
	Name                   string `json:"name" binding:"required,min=1,max=120"`
	NodeType               string `json:"node_type" binding:"required,oneof=A B"`
	RequestedCapacityBytes int64  `json:"requested_capacity_bytes" binding:"gte=0"`
	AgentVersion           string `json:"agent_version" binding:"required,max=64"`
}) {
	var responseNode model.Node
	var nodeToken string
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var enrollment struct {
			ID                     string
			Name                   string
			NodeType               string
			RequestedCapacityBytes int64
		}
		err := tx.Raw(`SELECT id::text, name, node_type, requested_capacity_bytes
			FROM node_enrollment_requests
			WHERE enrollment_token_hash = ? AND status = 'approved' AND token_expires_at > now()
			FOR UPDATE`, hashNodeToken(strings.TrimSpace(c.GetHeader("X-Node-Enrollment-Token")))).Scan(&enrollment).Error
		if err != nil {
			return err
		}
		if enrollment.ID == "" {
			return gorm.ErrRecordNotFound
		}
		if req.Name != enrollment.Name || req.NodeType != enrollment.NodeType {
			return errors.New("registration details do not match the approved request")
		}
		nodeToken, err = newNodeToken()
		if err != nil {
			return err
		}
		responseNode = model.Node{Name: req.Name, NodeType: req.NodeType, Status: "offline", RequestedCapacityBytes: req.RequestedCapacityBytes, AgentVersion: stringPtr(req.AgentVersion)}
		if err := tx.Create(&responseNode).Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE nodes SET api_token_hash = ?, updated_at = now() WHERE id = ?", hashNodeToken(nodeToken), responseNode.ID).Error; err != nil {
			return err
		}
		result := tx.Exec(`UPDATE node_enrollment_requests SET status = 'enrolled', node_id = ?, enrollment_token_hash = NULL, token_expires_at = NULL, updated_at = now() WHERE id = ? AND status = 'approved'`, responseNode.ID, enrollment.ID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("enrollment token has already been used")
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid, expired, or already used enrollment token"})
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "do not match") || strings.Contains(err.Error(), "already been used") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not complete node enrollment"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"node": gin.H{"id": responseNode.ID, "name": responseNode.Name, "node_type": responseNode.NodeType, "status": responseNode.Status, "requested_capacity_bytes": responseNode.RequestedCapacityBytes}, "node_token": nodeToken, "warning": "node_token is shown only once; store it securely"})
}

func (s *Server) authenticateNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeID := c.Param("id")
		if _, err := uuid.Parse(nodeID); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid node id"})
			return
		}
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "node bearer token required"})
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "node bearer token required"})
			return
		}
		var storedHash string
		err := s.db.WithContext(c.Request.Context()).Raw("SELECT api_token_hash FROM nodes WHERE id = ?", nodeID).Row().Scan(&storedHash)
		if err != nil || storedHash == "" || subtle.ConstantTimeCompare([]byte(storedHash), []byte(hashNodeToken(token))) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid node credentials"})
			return
		}
		var node model.Node
		if err := s.db.WithContext(c.Request.Context()).First(&node, "id = ?", nodeID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "node unavailable"})
			return
		}
		c.Set("currentNode", nodeContext{Node: node})
		c.Next()
	}
}

func currentNode(c *gin.Context) (model.Node, bool) {
	v, ok := c.Get("currentNode")
	if !ok {
		return model.Node{}, false
	}
	ctx, ok := v.(nodeContext)
	return ctx.Node, ok
}

func (s *Server) nodeHeartbeat(c *gin.Context) {
	node, ok := currentNode(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "node authentication required"})
		return
	}
	var req struct {
		CapacityBytes          int64  `json:"capacity_bytes" binding:"gte=0"`
		UsedBytes              int64  `json:"used_bytes" binding:"gte=0"`
		AvailableBytes         int64  `json:"available_bytes" binding:"gte=0"`
		RequestedCapacityBytes int64  `json:"requested_capacity_bytes" binding:"gte=0"`
		AgentVersion           string `json:"agent_version" binding:"required,max=64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid capacity report"})
		return
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status": "online", "capacity_bytes": req.CapacityBytes, "used_bytes": req.UsedBytes,
		"available_bytes": req.AvailableBytes, "requested_capacity_bytes": req.RequestedCapacityBytes,
		"agent_version": strings.TrimSpace(req.AgentVersion), "last_seen_at": now, "updated_at": now,
	}
	if node.Status != "online" || node.LastSeenAt == nil || time.Since(*node.LastSeenAt) > 2*time.Minute {
		updates["online_since"] = now
	}
	if err := s.db.WithContext(c.Request.Context()).Model(&model.Node{}).Where("id = ?", node.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update node heartbeat"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "online", "last_seen_at": now})
}

func (s *Server) listNodes(c *gin.Context) {
	var nodes []model.Node
	if err := s.db.WithContext(c.Request.Context()).Order("created_at DESC").Limit(500).Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list nodes"})
		return
	}

	type nodeView struct {
		ID                     string     `json:"id"`
		Name                   string     `json:"name"`
		NodeType               string     `json:"node_type"`
		Status                 string     `json:"status"`
		CapacityBytes          int64      `json:"capacity_bytes"`
		UsedBytes              int64      `json:"used_bytes"`
		AvailableBytes         int64      `json:"available_bytes"`
		RequestedCapacityBytes int64      `json:"requested_capacity_bytes"`
		ContactEmail           *string    `json:"contact_email,omitempty"`
		LastSeenAt             *time.Time `json:"last_seen_at,omitempty"`
		AgentVersion           *string    `json:"agent_version,omitempty"`
		CreatedAt              time.Time  `json:"created_at"`
		UpdatedAt              time.Time  `json:"updated_at"`
	}

	type enrollmentContact struct {
		NodeID       string `gorm:"column:node_id"`
		ContactEmail string `gorm:"column:contact_email"`
	}
	nodeIDs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		nodeIDs = append(nodeIDs, node.ID)
	}
	contacts := make(map[string]*string, len(nodes))
	if len(nodeIDs) > 0 {
		var latestContacts []enrollmentContact
		if err := s.db.WithContext(c.Request.Context()).
			Table("node_enrollment_requests").
			Select("DISTINCT ON (node_id) node_id, contact_email").
			Where("node_id IN ? AND status = ?", nodeIDs, "enrolled").
			Order("node_id, created_at DESC").
			Scan(&latestContacts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list node contacts"})
			return
		}
		for _, contact := range latestContacts {
			email := contact.ContactEmail
			contacts[contact.NodeID] = &email
		}
	}

	items := make([]nodeView, 0, len(nodes))
	for _, node := range nodes {
		status := node.Status
		if node.LastSeenAt == nil || time.Since(*node.LastSeenAt) > 2*time.Minute {
			status = "offline"
		}

		items = append(items, nodeView{
			ID:                     node.ID,
			Name:                   node.Name,
			NodeType:               node.NodeType,
			Status:                 status,
			CapacityBytes:          node.CapacityBytes,
			UsedBytes:              node.UsedBytes,
			AvailableBytes:         node.AvailableBytes,
			RequestedCapacityBytes: node.RequestedCapacityBytes,
			ContactEmail:           contacts[node.ID],
			LastSeenAt:             node.LastSeenAt,
			AgentVersion:           node.AgentVersion,
			CreatedAt:              node.CreatedAt,
			UpdatedAt:              node.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) deleteNodes(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node id"})
		return
	}
	actor, _ := currentUser(c)
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("node_enrollment_requests").Where("node_id = ?", id).Delete(nil)
		if result.Error != nil {
			return result.Error
		}
		result = tx.Delete(&model.Node{ID: id})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("node has been aleady deleted")
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete node"})
		return
	}
	s.audit(c, &actor, "node", id, "node.delete", "success", nil)
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *Server) listTasks(c *gin.Context) {
	page, limit, isAll := pagination(c)

	query := s.db.WithContext(c.Request.Context()).
		Model(&model.Task{}).
		Where(`NOT (
            tasks.status = 'failed'
            AND tasks.task_type IN ('source_fetch', 'store')
            AND EXISTS (
                SELECT 1 FROM file_objects
                WHERE file_objects.id = tasks.file_object_id
                AND file_objects.preservation_status = 'verified'
            )
        )`)

	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("tasks.status = ?", status)
	}

	// フィルター適用後の全件数を取得
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not count tasks",
		})
		return
	}

	// タスクを取得
	var tasks []model.Task
	taskQuery := query.Order("priority DESC, created_at ASC, tasks.id ASC")
	if !isAll {
		taskQuery = taskQuery.Limit(limit).Offset((page - 1) * limit)
	}
	if err := taskQuery.Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not list tasks",
		})
		return
	}

	type taskView struct {
		ID           string    `json:"id"`
		TaskType     string    `json:"task_type"`
		FileObjectID string    `json:"file_object_id"`
		SourceNodeID *string   `json:"source_node_id,omitempty"`
		TargetNodeID *string   `json:"target_node_id,omitempty"`
		Status       string    `json:"status"`
		Priority     int16     `json:"priority"`
		AttemptCount int32     `json:"attempt_count"`
		LastError    *string   `json:"last_error,omitempty"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
	}

	items := make([]taskView, 0, len(tasks))
	for _, t := range tasks {
		items = append(items, taskView{
			ID:           t.ID,
			TaskType:     t.TaskType,
			FileObjectID: t.FileObjectID,
			SourceNodeID: t.SourceNodeID,
			TargetNodeID: t.TargetNodeID,
			Status:       t.Status,
			Priority:     t.Priority,
			AttemptCount: t.AttemptCount,
			LastError:    t.LastError,
			CreatedAt:    t.CreatedAt,
			UpdatedAt:    t.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (s *Server) ingestFile(c *gin.Context) {
	fileID := c.Param("id")
	if _, err := uuid.Parse(fileID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}
	var file model.FileObject
	if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ? AND deleted_at IS NULL AND deletion_requested_at IS NULL", fileID).Error; err != nil {
		notFoundOrError(c, err, "file object")
		return
	}
	if file.PreservationStatus == "verified" {
		c.JSON(http.StatusConflict, gin.H{"error": "file object is already verified; create a new file object to ingest a different version"})
		return
	}
	if isUnconfirmedRedistributionPolicy(file.RedistributionPolicy) {
		c.JSON(http.StatusConflict, gin.H{"error": "redistribution policy is unconfirmed; set a policy other than pending before preservation"})
		return
	}
	var activeTasks int64
	if err := s.db.WithContext(c.Request.Context()).Model(&model.Task{}).Where("file_object_id = ? AND status IN ('queued', 'running')", file.ID).Count(&activeTasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check existing tasks"})
		return
	}
	if activeTasks > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "an ingestion task is already active for this file object"})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxIngestBytes+(1<<20))
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected multipart form with a file field; upload limit is 5 GiB"})
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	fh, headerErr := c.FormFile("file")
	if headerErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart field 'file' is required"})
		return
	}
	priority := 3
	if rawPriority := strings.TrimSpace(c.PostForm("priority")); rawPriority != "" {
		parsedPriority, err := strconv.Atoi(rawPriority)
		if err != nil || parsedPriority < 0 || parsedPriority > 5 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "priority must be an integer between 0 and 5"})
			return
		}
		priority = parsedPriority
	}
	if fh.Size < 0 || fh.Size > maxIngestBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file exceeds the 5 GiB MVP upload limit"})
		return
	}
	filename := strings.TrimSpace(filepath.Base(strings.ReplaceAll(fh.Filename, "\\", "/")))
	if filename == "" || len(filename) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filename"})
		return
	}
	if err := os.MkdirAll(s.cfg.StagingDir, 0o750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "staging storage is unavailable"})
		return
	}
	source, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not open uploaded file"})
		return
	}
	defer source.Close()
	tmp, err := os.CreateTemp(s.cfg.StagingDir, ".upload-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create staging file"})
		return
	}
	tmpName := tmp.Name()
	cleanupTmp := func() { _ = os.Remove(tmpName) }
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(source, maxIngestBytes+1))
	if copyErr != nil || written > maxIngestBytes || written != fh.Size {
		_ = tmp.Close()
		cleanupTmp()
		c.JSON(http.StatusBadRequest, gin.H{"error": "uploaded file could not be read completely"})
		return
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanupTmp()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not sync staging file"})
		return
	}
	if err := tmp.Close(); err != nil {
		cleanupTmp()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not close staging file"})
		return
	}

	sha := hex.EncodeToString(hasher.Sum(nil))
	mediaType := strings.TrimSpace(fh.Header.Get("Content-Type"))
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	s.queueStagedIngest(c, file, tmpName, written, sha, mediaType, filename, priority, file.SourceURL, "", "", "")

}

// queueStagedIngest takes a verified temporary file and queues its storage on two nodes.
// The temporary file is removed on every exit path.
func (s *Server) queueStagedIngest(c *gin.Context, file model.FileObject, tmpName string, written int64, sha, mediaType, filename string, priority int, sourceURL *string, sourceFetchTaskID, sourceFetchNodeID, sourceFetchLease string) bool {
	defer os.Remove(tmpName)
	nodes, err := s.selectNodesForCapacity(c, written, 2)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "required_placements": 2})
		return false
	}
	if err := os.MkdirAll(s.cfg.StagingDir, 0o750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "staging storage is unavailable"})
		return false
	}
	tasks := make([]model.Task, 0, len(nodes))
	placements := make([]model.Placement, 0, len(nodes))
	stagePaths := make([]string, 0, len(nodes))
	for i, node := range nodes {
		taskID := uuid.NewString()
		stagePath := filepath.Join(s.cfg.StagingDir, taskID)
		if i == 0 {
			if err := os.Rename(tmpName, stagePath); err != nil {
				_ = os.Remove(tmpName)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not finalize staging file"})
				return false
			}
		} else if err := copyLocalFile(stagePaths[0], stagePath); err != nil {
			for _, existing := range stagePaths {
				_ = os.Remove(existing)
			}
			_ = os.Remove(tmpName)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not prepare staged replica"})
			return false
		}
		stagePaths = append(stagePaths, stagePath)
		task := model.Task{ID: taskID, TaskType: "store", FileObjectID: file.ID, TargetNodeID: &node.ID, Status: "queued", Priority: int16(priority)}
		storageKey := "objects/" + file.ID + "/" + taskID
		placement := model.Placement{FileObjectID: file.ID, NodeID: node.ID, StorageKey: storageKey, SHA256: &sha, Status: "queued"}
		tasks = append(tasks, task)
		placements = append(placements, placement)
	}
	now := time.Now().UTC()
	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.FileObject{}).Where("id = ?", file.ID).Updates(map[string]any{
			"original_filename": filename, "size_bytes": written, "sha256": sha, "media_type": mediaType, "preservation_status": "queued", "public_download_enabled": false,
		}).Error; err != nil {
			return err
		}
		if sourceURL != nil {
			if err := tx.Model(&model.FileObject{}).Where("id = ?", file.ID).Update("source_url", *sourceURL).Error; err != nil {
				return err
			}
		}
		if sourceFetchTaskID != "" {
			if err := tx.Model(&model.FileObject{}).Where("id = ?", file.ID).Update("source_password_encrypted", nil).Error; err != nil {
				return err
			}
		}
		for i, task := range tasks {
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
			var existing model.Placement
			placementErr := tx.Where("file_object_id = ? AND node_id = ?", file.ID, nodes[i].ID).First(&existing).Error
			if errors.Is(placementErr, gorm.ErrRecordNotFound) {
				if err := tx.Create(&placements[i]).Error; err != nil {
					return err
				}
			} else if placementErr != nil {
				return placementErr
			} else if err := tx.Model(&model.Placement{}).Where("id = ?", existing.ID).Updates(map[string]any{
				"sha256": sha, "status": "queued", "verified_at": nil,
			}).Error; err != nil {
				return err
			}
		}
		if sourceFetchTaskID != "" {
			var sourceTask model.Task
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND task_type = 'source_fetch' AND status = 'running' AND target_node_id = ? AND lease_token = ? AND lease_until > now()", sourceFetchTaskID, sourceFetchNodeID, sourceFetchLease).First(&sourceTask).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Task{}).Where("id = ?", sourceFetchTaskID).Updates(map[string]any{"status": "completed", "lease_until": nil, "lease_token": nil, "last_error": nil, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		for _, path := range stagePaths {
			_ = os.Remove(path)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create preservation tasks"})
		return false
	}
	actor, hasActor := currentUser(c)
	var actorPtr *model.User
	if hasActor {
		actorPtr = &actor
	}
	s.audit(c, actorPtr, "file_object", file.ID, "file_object.ingest.queued", "success", stringPtr("File staged and queued for two-node storage"))
	taskViews := make([]gin.H, 0, len(tasks))
	nodeViews := make([]gin.H, 0, len(nodes))
	for i := range tasks {
		taskViews = append(taskViews, gin.H{"task_id": tasks[i].ID, "node_id": nodes[i].ID, "status": "queued"})
		nodeViews = append(nodeViews, gin.H{"id": nodes[i].ID, "name": nodes[i].Name, "node_type": nodes[i].NodeType})
	}
	c.JSON(http.StatusAccepted, gin.H{"file_object_id": file.ID, "task_id": tasks[0].ID, "node_id": nodes[0].ID, "tasks": taskViews, "target_nodes": nodeViews, "replication_target": 2, "status": "queued", "size_bytes": written, "sha256": sha})
	return true
}

func copyLocalFile(sourcePath, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(target, source)
	syncErr := target.Sync()
	closeErr := target.Close()
	if copyErr != nil {
		_ = os.Remove(targetPath)
		return copyErr
	}
	if syncErr != nil {
		_ = os.Remove(targetPath)
		return syncErr
	}
	if closeErr != nil {
		_ = os.Remove(targetPath)
		return closeErr
	}
	return nil
}

func (s *Server) retryAll(c *gin.Context) {
	ctx := c.Request.Context()

	actor, ok := currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	var tasks []model.Task
	if err := s.db.WithContext(ctx).
		Where("status = ?", "failed").
		Order("created_at ASC").
		Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not load failed tasks",
		})
		return
	}

	results := make([]gin.H, 0, len(tasks))

	succeeded := 0
	failed := 0
	skipped := 0

	for _, task := range tasks {
		if err := s.retryFailedTask(ctx, task.ID, actor); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				skipped++
				results = append(results, gin.H{
					"task_id": task.ID,
					"status":  "skipped",
					"reason":  "task or related file no longer exists",
				})
				continue
			}

			skipped++
			results = append(results, gin.H{
				"task_id": task.ID,
				"status":  "skipped",
				"reason":  err.Error(),
			})
			continue
		}

		succeeded++
		results = append(results, gin.H{
			"task_id": task.ID,
			"status":  "queued",
		})
	}

	c.JSON(http.StatusAccepted, gin.H{
		"total":     len(tasks),
		"succeeded": succeeded,
		"failed":    failed,
		"skipped":   skipped,
		"results":   results,
	})
}

func (s *Server) retryFailedTask(
	ctx context.Context,
	taskID string,
	actor model.User,
) error {
	now := time.Now().UTC()

	var errTaskNotRetryable = errors.New("task is no longer failed or cannot be retried")
	var errActiveTaskExists = errors.New("another active task already exists for this file object")
	var errNoAlternativeNode = errors.New("no online storage node available")

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.Task

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&task, "id = ?", taskID).Error; err != nil {
			return err
		}

		if task.Status != "failed" {
			return errTaskNotRetryable
		}

		if task.TaskType != "store" && task.TaskType != "source_fetch" {
			return errTaskNotRetryable
		}

		var file model.FileObject
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&file, "id = ?", task.FileObjectID).Error; err != nil {
			return err
		}

		if file.PreservationStatus == "verified" {
			return errors.New("file object is already verified")
		}

		if isUnconfirmedRedistributionPolicy(file.RedistributionPolicy) {
			return errors.New("redistribution policy is unconfirmed")
		}

		// 同じファイルに対する別の実行中・待機中タスクを確認する。
		var activeCount int64
		if err := tx.Model(&model.Task{}).
			Where(
				"file_object_id = ? AND id <> ? AND status IN ('queued', 'running')",
				file.ID,
				task.ID,
			).
			Count(&activeCount).Error; err != nil {
			return err
		}

		if activeCount > 0 {
			return errActiveTaskExists
		}

		// source_fetch の再試行
		if task.TaskType == "source_fetch" {
			if file.SourceURL == nil || strings.TrimSpace(*file.SourceURL) == "" {
				return errors.New("source-fetch task has no source URL")
			}

			// 未割り当てに戻し、claimNodeTaskで取得ノードを決定する。
			result := tx.Model(&model.Task{}).
				Where("id = ? AND status = 'failed'", task.ID).
				Updates(map[string]any{
					"status":         "queued",
					"target_node_id": nil,
					"attempt_count":  0,
					"last_error":     nil,
					"lease_token":    nil,
					"lease_until":    nil,
					"updated_at":     now,
				})

			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errTaskNotRetryable
			}

			if err := tx.Model(&model.FileObject{}).
				Where("id = ?", file.ID).
				Update("preservation_status", "queued").Error; err != nil {
				return err
			}

			reason := "Administrator retried failed source-fetch task; target node cleared for automatic assignment"

			if err := tx.Create(&model.AuditLog{
				ActorID:    &actor.ID,
				ActorLabel: actor.Email,
				TargetType: "task",
				TargetID:   &task.ID,
				Action:     "task.source_fetch.retry",
				Result:     "success",
				Reason:     &reason,
			}).Error; err != nil {
				return err
			}

			return nil
		}

		// store の再試行
		if task.TargetNodeID == nil {
			return errTaskNotRetryable
		}

		if file.SHA256 == nil || len(*file.SHA256) != 64 {
			return errors.New("file has no valid expected SHA-256")
		}

		stagingPath := filepath.Join(s.cfg.StagingDir, task.ID)

		gotSHA, gotSize, err := hashStagedFile(stagingPath)
		if err != nil {
			return errors.New("staged source is missing or unreadable")
		}

		if gotSize != file.SizeBytes || !strings.EqualFold(gotSHA, *file.SHA256) {
			return errors.New("staged source failed integrity verification")
		}

		// 保存先ノードを選択する。
		// 検証済みコピーがあるノードは候補から除外する。
		var targetNode model.Node

		if err := tx.Raw(`
			SELECT n.id, n.name
			FROM nodes AS n
			WHERE n.status = 'online'
			  AND NOT EXISTS (
			      SELECT 1
			      FROM placements AS p
			      WHERE p.node_id = n.id
			        AND p.file_object_id = ?
			        AND p.status = 'verified'
			  )
			ORDER BY (n.id = ?), random()
			LIMIT 1
			FOR UPDATE OF n
		`, file.ID, *task.TargetNodeID).Scan(&targetNode).Error; err != nil {
			return err
		}

		if targetNode.ID == "" {
			return errNoAlternativeNode
		}

		// 新しいノードのPlacementを取得する。
		var placement model.Placement

		placementErr := tx.
			Where(
				"file_object_id = ? AND node_id = ?",
				file.ID,
				targetNode.ID,
			).
			First(&placement).Error

		if errors.Is(placementErr, gorm.ErrRecordNotFound) {
			placement = model.Placement{
				FileObjectID: file.ID,
				NodeID:       targetNode.ID,
				StorageKey:   "objects/" + file.ID + "/" + task.ID,
				SHA256:       file.SHA256,
				Status:       "queued",
			}

			if err := tx.Create(&placement).Error; err != nil {
				return err
			}
		} else if placementErr != nil {
			return placementErr
		} else {
			if placement.Status == "verified" {
				return errors.New("target node already has a verified placement")
			}

			if err := tx.Model(&model.Placement{}).
				Where("id = ?", placement.ID).
				Updates(map[string]any{
					"status":      "queued",
					"sha256":      file.SHA256,
					"verified_at": nil,
				}).Error; err != nil {
				return err
			}
		}

		// タスクを再キューイングする。
		result := tx.Model(&model.Task{}).
			Where("id = ? AND status = 'failed'", task.ID).
			Updates(map[string]any{
				"target_node_id": targetNode.ID,
				"status":         "queued",
				"attempt_count":  0,
				"last_error":     nil,
				"lease_until":    nil,
				"lease_token":    nil,
				"updated_at":     now,
			})

		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errTaskNotRetryable
		}

		// ファイルの保存状態を更新する。
		if err := tx.Model(&model.FileObject{}).
			Where("id = ?", file.ID).
			Updates(map[string]any{
				"preservation_status":     "queued",
				"public_download_enabled": false,
			}).Error; err != nil {
			return err
		}

		// 元のPlacementが未検証ならエラー状態にする。
		if targetNode.ID != *task.TargetNodeID {
			if err := tx.Model(&model.Placement{}).
				Where(
					"file_object_id = ? AND node_id = ? AND status <> 'verified'",
					file.ID,
					*task.TargetNodeID,
				).
				Update("status", "error").Error; err != nil {
				return err
			}
		}

		reason := fmt.Sprintf(
			"Administrator retried failed storage task; reassigned from node %s to node %s",
			*task.TargetNodeID,
			targetNode.ID,
		)

		return tx.Create(&model.AuditLog{
			ActorID:    &actor.ID,
			ActorLabel: actor.Email,
			TargetType: "task",
			TargetID:   &task.ID,
			Action:     "task.retry",
			Result:     "success",
			Reason:     &reason,
		}).Error
	})
}

func (s *Server) retryTask(c *gin.Context) {
	taskID := c.Param("id")

	if _, err := uuid.Parse(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	actor, _ := currentUser(c)

	if err := s.retryFailedTask(c.Request.Context(), taskID, actor); err != nil {
		// エラーに応じたHTTPレスポンス
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"task_id": taskID,
		"status":  "queued",
	})
}

func hashStagedFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	hasher := sha256.New()
	size, err := io.Copy(hasher, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hasher.Sum(nil)), size, nil
}

func (s *Server) selectNodesForCapacity(c *gin.Context, incoming int64, count int) ([]model.Node, error) {
	var candidates []model.Node
	cutoff := time.Now().UTC().Add(-2 * time.Minute)
	if err := s.db.WithContext(c.Request.Context()).Where("status = ? AND last_seen_at >= ?", "online", cutoff).Order("available_bytes DESC, last_seen_at DESC").Limit(500).Find(&candidates).Error; err != nil {
		return nil, errors.New("could not select storage nodes")
	}
	selected := make([]model.Node, 0, count)
	for _, node := range candidates {
		var reserved int64
		err := s.db.WithContext(c.Request.Context()).Table("tasks AS t").Select("COALESCE(SUM(f.size_bytes), 0)").Joins("JOIN file_objects AS f ON f.id = t.file_object_id").Where("t.target_node_id = ? AND t.status IN ('queued', 'running')", node.ID).Scan(&reserved).Error
		if err != nil {
			return nil, errors.New("could not calculate reserved node capacity")
		}
		if node.AvailableBytes-reserved < incoming {
			continue
		}
		if node.RequestedCapacityBytes > 0 && node.UsedBytes+reserved+incoming > node.RequestedCapacityBytes {
			continue
		}
		selected = append(selected, node)
		if len(selected) == count {
			return selected, nil
		}
	}
	return nil, fmt.Errorf("need %d online nodes with enough free space and participant quota; found %d eligible nodes", count, len(selected))
}

func (s *Server) claimNodeTask(c *gin.Context) {
	node, ok := currentNode(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "node authentication required"})
		return
	}
	var claimed struct {
		Task       model.Task
		LeaseToken string
	}
	noTask := false
	leaseToken := uuid.NewString()
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`((task_type IN ('store', 'delete') AND target_node_id = ?)
		  OR (task_type = 'replicate' AND source_node_id = ?)
		  OR (task_type = 'source_fetch' AND (target_node_id = ? OR target_node_id IS NULL)))
			AND status = 'running' AND lease_until < now() AND attempt_count >= ?`,
			node.ID, node.ID, node.ID, maxTaskAttempts).Error; err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE placements SET status = 'error' WHERE node_id = ? AND file_object_id IN
			(SELECT file_object_id FROM tasks WHERE task_type IN ('store','delete') AND target_node_id = ? AND status = 'failed' AND last_error = 'task lease expired after maximum attempts')`, node.ID, node.ID).Error; err != nil {
			return err
		}
		err := tx.Raw(`((task_type IN ('store', 'delete') AND target_node_id = ?)
  		OR (task_type = 'source_fetch' AND (target_node_id = ? OR target_node_id IS NULL))
  		OR (task_type = 'replicate' AND source_node_id = ?))
			AND attempt_count < ?
			AND (status = 'queued' OR (status = 'running' AND lease_until < now()))
			ORDER BY priority DESC, created_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED`,
			node.ID, node.ID, node.ID, maxTaskAttempts).Scan(&claimed.Task).Error
		if err != nil {
			return err
		}
		if claimed.Task.ID == "" {
			noTask = true
			return nil
		}
		if claimed.Task.TaskType == "source_fetch" {
			claimed.Task.TargetNodeID = &node.ID
		}
		if err := tx.Exec(`UPDATE tasks SET status = 'running', attempt_count = attempt_count + 1,
    target_node_id = CASE WHEN task_type = 'source_fetch' AND target_node_id IS NULL THEN ? ELSE target_node_id END,
    lease_until = ?, lease_token = ?, last_error = NULL, updated_at = now() WHERE id = ?`,
			node.ID, time.Now().UTC().Add(nodeLeaseDuration), leaseToken, claimed.Task.ID).Error; err != nil {
			return err
		}
		if claimed.Task.TaskType == "store" {
			if err := tx.Model(&model.Placement{}).Where("file_object_id = ? AND node_id = ?", claimed.Task.FileObjectID, node.ID).Update("status", "running").Error; err != nil {
				return err
			}
		}
		claimed.Task.Status = "running"
		claimed.Task.AttemptCount++
		claimed.LeaseToken = leaseToken
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not claim task"})
		return
	}
	if noTask {
		c.Status(http.StatusNoContent)
		return
	}
	var file model.FileObject
	if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ?", claimed.Task.FileObjectID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load task file"})
		return
	}
	if claimed.Task.TaskType == "source_fetch" {
		if file.SourceURL == nil || strings.TrimSpace(*file.SourceURL) == "" {
			c.JSON(http.StatusConflict, gin.H{"error": "source-fetch task has no source URL"})
			return
		}
		sourcePassword, passwordErr := s.decryptSourcePassword(file.SourcePasswordEncrypted)
		if passwordErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": passwordErr.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"task_id": claimed.Task.ID, "task_type": claimed.Task.TaskType, "lease_token": claimed.LeaseToken,
			"file_object_id": file.ID, "source_url": *file.SourceURL, "source_password": sourcePassword, "attempt_count": claimed.Task.AttemptCount,
			"target_node_id": claimed.Task.TargetNodeID,
		})
		return
	}
	placementNodeID := node.ID
	if claimed.Task.TaskType == "replicate" {
		placementNodeID = valueOrEmpty(claimed.Task.SourceNodeID)
	}
	var placement model.Placement
	if err := s.db.WithContext(c.Request.Context()).First(&placement, "file_object_id = ? AND node_id = ?", file.ID, placementNodeID).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "could not load source or target placement"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"task_id": claimed.Task.ID, "task_type": claimed.Task.TaskType, "lease_token": claimed.LeaseToken, "file_object_id": file.ID,
		"original_filename": file.OriginalFilename, "size_bytes": file.SizeBytes, "sha256": file.SHA256,
		"storage_key": placement.StorageKey, "attempt_count": claimed.Task.AttemptCount,
		"source_node_id": claimed.Task.SourceNodeID, "target_node_id": claimed.Task.TargetNodeID,
	})
}

func (s *Server) nodeTaskContent(c *gin.Context) {
	node, ok := currentNode(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "node authentication required"})
		return
	}
	taskID := c.Param("taskID")
	if _, err := uuid.Parse(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}
	lease := c.Query("lease_token")
	if _, err := uuid.Parse(lease); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid lease_token is required"})
		return
	}
	var count int64
	if err := s.db.WithContext(c.Request.Context()).Model(&model.Task{}).Where("id = ? AND task_type = 'store' AND target_node_id = ? AND status = 'running' AND lease_token = ? AND lease_until > now()", taskID, node.ID, lease).Count(&count).Error; err != nil || count != 1 {
		c.JSON(http.StatusConflict, gin.H{"error": "task lease is no longer valid"})
		return
	}
	path := filepath.Join(s.cfg.StagingDir, taskID)
	f, err := os.Open(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "staged content is no longer available"})
		return
	}
	defer f.Close()
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, f)
}

func (s *Server) nodeTaskResult(c *gin.Context) {
	node, ok := currentNode(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "node authentication required"})
		return
	}
	taskID := c.Param("taskID")
	if _, err := uuid.Parse(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}
	var req struct {
		LeaseToken string `json:"lease_token" binding:"required,uuid"`
		Status     string `json:"status" binding:"required,oneof=success failure"`
		SHA256     string `json:"sha256" binding:"omitempty,len=64,hexadecimal"`
		SizeBytes  int64  `json:"size_bytes" binding:"gte=0"`
		Error      string `json:"error" binding:"omitempty,max=2000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task result"})
		return
	}
	var task model.Task
	if err := s.db.WithContext(c.Request.Context()).Where("id = ? AND ((task_type IN ('store','delete','source_fetch') AND target_node_id = ?) OR (task_type = 'replicate' AND source_node_id = ?))", taskID, node.ID, node.ID).First(&task).Error; err != nil {
		notFoundOrError(c, err, "task")
		return
	}
	if task.Status != "running" {
		c.JSON(http.StatusConflict, gin.H{"error": "task is not running"})
		return
	}
	if task.TaskType == "source_fetch" && req.Status == "success" {
		c.JSON(http.StatusConflict, gin.H{"error": "source-fetch tasks must upload the fetched content to the source-content endpoint"})
		return
	}
	var leaseToken string
	if err := s.db.WithContext(c.Request.Context()).Raw("SELECT lease_token::text FROM tasks WHERE id = ?", taskID).Row().Scan(&leaseToken); err != nil || leaseToken != req.LeaseToken {
		c.JSON(http.StatusConflict, gin.H{"error": "task lease token does not match"})
		return
	}
	if task.TaskType == "replicate" && req.Status == "success" {
		c.JSON(http.StatusConflict, gin.H{"error": "replicate tasks must upload source content to the replica-content endpoint"})
		return
	}
	var file model.FileObject
	if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ?", task.FileObjectID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load task file"})
		return
	}
	if req.Status == "success" && task.TaskType != "delete" && (file.SHA256 == nil || !strings.EqualFold(*file.SHA256, req.SHA256) || file.SizeBytes != req.SizeBytes) {
		req.Status = "failure"
		req.Error = fmt.Sprintf("integrity verification failed: expected size=%d sha256=%s, received size=%d sha256=%s", file.SizeBytes, valueOrEmpty(file.SHA256), req.SizeBytes, req.SHA256)
	}
	terminal := req.Status == "failure" && (task.AttemptCount >= maxTaskAttempts || (task.TaskType == "source_fetch" && sourceFetchFailureIsPermanent(req.Error)))
	nextStatus := "queued"
	if req.Status == "success" {
		nextStatus = "completed"
	} else if terminal {
		nextStatus = "failed"
	}
	if req.Status == "failure" && strings.TrimSpace(req.Error) == "" {
		req.Error = "node agent reported task failure"
	}
	now := time.Now().UTC()
	placementNodeID := node.ID
	if task.TaskType == "replicate" {
		placementNodeID = valueOrEmpty(task.TargetNodeID)
	}
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"status": nextStatus, "lease_until": nil, "lease_token": nil, "updated_at": now}
		if req.Status == "failure" {
			updates["last_error"] = req.Error
		}
		if err := tx.Model(&model.Task{}).Where("id = ?", task.ID).Updates(updates).Error; err != nil {
			return err
		}
		placementUpdates := map[string]any{"status": "queued"}
		if req.Status == "success" {
			if task.TaskType == "delete" {
				placementUpdates = map[string]any{"status": "deleted"}
			} else {
				placementUpdates = map[string]any{"status": "verified", "sha256": strings.ToLower(req.SHA256), "verified_at": now}
			}
		} else if terminal {
			placementUpdates = map[string]any{"status": "error"}
		}
		if task.TaskType == "store" || task.TaskType == "delete" || terminal {
			if err := tx.Model(&model.Placement{}).Where("file_object_id = ? AND node_id = ?", file.ID, placementNodeID).Updates(placementUpdates).Error; err != nil {
				return err
			}
		}
		if task.TaskType == "delete" && req.Status == "success" && file.DeletionRequestedAt != nil {
			var remaining int64
			if err := tx.Model(&model.Placement{}).Where("file_object_id = ? AND status <> 'deleted'", file.ID).Count(&remaining).Error; err != nil {
				return err
			}
			if remaining == 0 {
				if err := tx.Model(&model.FileObject{}).Where("id = ? AND deleted_at IS NULL", file.ID).Update("deleted_at", now).Error; err != nil {
					return err
				}
				entry := model.AuditLog{ActorLabel: "node:" + node.Name, TargetType: "file_object", TargetID: &file.ID, Action: "file_object.deletion.completed", Result: "success", Reason: stringPtr("All known placements reported successful deletion")}
				if err := tx.Create(&entry).Error; err != nil {
					return err
				}
			}
		}
		if task.TaskType == "replicate" && terminal && (strings.Contains(req.Error, "source placement failed verification") || strings.Contains(req.Error, "could not read source placement") || strings.Contains(req.Error, "source replica failed size or SHA-256 validation")) {
			if err := tx.Model(&model.Placement{}).Where("file_object_id = ? AND node_id = ?", file.ID, node.ID).Update("status", "error").Error; err != nil {
				return err
			}
		}
		if req.Status == "success" && task.TaskType == "store" {
			if err := tx.Model(&model.Placement{}).Where("file_object_id = ? AND node_id = ?", file.ID, node.ID).Updates(map[string]any{"status": "verified", "sha256": strings.ToLower(req.SHA256), "verified_at": now}).Error; err != nil {
				return err
			}
		}
		result, action := "success", "node.task.completed"
		if req.Status == "failure" {
			result, action = "failure", "node.task.failed"
		}
		if err := tx.Create(&model.AuditLog{ActorLabel: "node:" + node.Name, TargetType: "task", TargetID: &task.ID, Action: action, Result: result, Reason: stringPtr(req.Error)}).Error; err != nil {
			return err
		}
		return recomputeFilePreservation(tx, file.ID)
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not record task result"})
		return
	}
	if req.Status == "success" && task.TaskType == "store" {
		_ = os.Remove(filepath.Join(s.cfg.StagingDir, task.ID))
	}
	var updatedFile model.FileObject
	_ = s.db.WithContext(c.Request.Context()).Select("preservation_status").First(&updatedFile, "id = ?", file.ID).Error
	c.JSON(http.StatusOK, gin.H{"task_id": task.ID, "status": nextStatus, "retrying": nextStatus == "queued", "preservation_status": updatedFile.PreservationStatus})
}

func (s *Server) receiveSourceFetchContent(c *gin.Context) {
	node, ok := currentNode(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "node authentication required"})
		return
	}
	taskID := c.Param("taskID")
	if _, err := uuid.Parse(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}
	lease := c.Query("lease_token")
	if _, err := uuid.Parse(lease); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid lease_token is required"})
		return
	}
	var task model.Task
	if err := s.db.WithContext(c.Request.Context()).Where("id = ? AND task_type = 'source_fetch' AND target_node_id = ? AND status = 'running' AND lease_token = ? AND lease_until > now()", taskID, node.ID, lease).First(&task).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "source-fetch task lease is no longer valid"})
		return
	}
	var file model.FileObject
	if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ?", task.FileObjectID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load source file"})
		return
	}
	if isUnconfirmedRedistributionPolicy(file.RedistributionPolicy) {
		c.JSON(http.StatusConflict, gin.H{"error": "redistribution policy is unconfirmed; source content was not accepted"})
		return
	}
	if strings.Contains(strings.ToLower(c.GetHeader("Content-Type")), "text/html") {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "agent uploaded an HTML page instead of file content"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxIngestBytes+1)
	if err := os.MkdirAll(s.cfg.StagingDir, 0o750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "staging storage is unavailable"})
		return
	}
	tmp, err := os.CreateTemp(s.cfg.StagingDir, ".source-agent-")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create source staging file"})
		return
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(c.Request.Body, maxIngestBytes+1))
	if copyErr != nil || written > maxIngestBytes {
		_ = tmp.Close()
		cleanup()
		c.JSON(http.StatusBadRequest, gin.H{"error": "source content upload failed, was empty, or exceeded the 5 GiB limit"})
		return
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not sync source staging file"})
		return
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not close source staging file"})
		return
	}
	filename := safeSourceFilename(c.GetHeader("X-Source-Filename"))
	if filename == "" {
		filename = safeSourceFilename(file.OriginalFilename)
	}
	if filename == "" || filename == "取得待ち" {
		filename = "uploader-download.bin"
	}
	mediaType := strings.TrimSpace(strings.ToLower(c.GetHeader("X-Source-Media-Type")))
	if mediaType == "" || strings.Contains(mediaType, "text/html") {
		mediaType = "application/octet-stream"
	}
	sha := hex.EncodeToString(hasher.Sum(nil))
	sourceURL := file.SourceURL
	s.queueStagedIngest(c, file, tmpName, written, sha, mediaType, filename, int(task.Priority), sourceURL, task.ID, node.ID, lease)
}

func (s *Server) replicaContent(c *gin.Context) {
	node, ok := currentNode(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "node authentication required"})
		return
	}
	taskID := c.Param("taskID")
	if _, err := uuid.Parse(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}
	lease := c.Query("lease_token")
	if _, err := uuid.Parse(lease); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid lease_token is required"})
		return
	}
	var task model.Task
	if err := s.db.WithContext(c.Request.Context()).Where("id = ? AND task_type = 'replicate' AND source_node_id = ? AND status = 'running' AND lease_token = ? AND lease_until > now()", taskID, node.ID, lease).First(&task).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "replication task lease is no longer valid"})
		return
	}
	var file model.FileObject
	if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ?", task.FileObjectID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load replica file"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxIngestBytes+1)
	if err := os.MkdirAll(s.cfg.StagingDir, 0o750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "staging storage is unavailable"})
		return
	}
	tmp, err := os.CreateTemp(s.cfg.StagingDir, ".replica-upload-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create replica staging file"})
		return
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, hasher), c.Request.Body)
	if copyErr != nil || written > maxIngestBytes {
		_ = tmp.Close()
		cleanup()
		c.JSON(http.StatusBadRequest, gin.H{"error": "replica content upload failed or exceeded size limit"})
		return
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not sync replica staging file"})
		return
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not close replica staging file"})
		return
	}
	gotSHA := hex.EncodeToString(hasher.Sum(nil))
	if written != file.SizeBytes || file.SHA256 == nil || !strings.EqualFold(gotSHA, *file.SHA256) {
		cleanup()
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "source replica failed size or SHA-256 validation"})
		return
	}
	stagingPath := filepath.Join(s.cfg.StagingDir, task.ID)
	if err := os.Rename(tmpName, stagingPath); err != nil {
		cleanup()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not finalize replica staging file"})
		return
	}
	now := time.Now().UTC()
	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var locked model.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND task_type = 'replicate' AND source_node_id = ? AND status = 'running' AND lease_token = ? AND lease_until > now()", taskID, node.ID, lease).First(&locked).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Task{}).Where("id = ?", task.ID).Updates(map[string]any{"task_type": "store", "source_node_id": nil, "status": "queued", "attempt_count": 0, "last_error": nil, "lease_until": nil, "lease_token": nil, "updated_at": now}).Error; err != nil {
			return err
		}
		storageKey := "objects/" + file.ID + "/" + task.ID
		var placement model.Placement
		placementErr := tx.Where("file_object_id = ? AND node_id = ?", file.ID, valueOrEmpty(task.TargetNodeID)).First(&placement).Error
		if errors.Is(placementErr, gorm.ErrRecordNotFound) {
			if err := tx.Create(&model.Placement{FileObjectID: file.ID, NodeID: valueOrEmpty(task.TargetNodeID), StorageKey: storageKey, SHA256: file.SHA256, Status: "queued"}).Error; err != nil {
				return err
			}
		} else if placementErr != nil {
			return placementErr
		} else {
			storageKey = placement.StorageKey
			if err := tx.Model(&model.Placement{}).Where("id = ?", placement.ID).Updates(map[string]any{"storage_key": storageKey, "sha256": file.SHA256, "status": "queued", "verified_at": nil}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.FileObject{}).Where("id = ?", file.ID).Update("preservation_status", "queued").Error; err != nil {
			return err
		}
		return tx.Create(&model.AuditLog{ActorLabel: "node:" + node.Name, TargetType: "task", TargetID: &task.ID, Action: "node.replication.source-uploaded", Result: "success", Reason: stringPtr("Verified source-node bytes staged for target-node storage")}).Error
	})
	if err != nil {
		_ = os.Remove(stagingPath)
		c.JSON(http.StatusConflict, gin.H{"error": "could not queue target-node storage after replication upload"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"task_id": task.ID, "task_type": "store", "target_node_id": task.TargetNodeID, "status": "queued", "sha256": gotSHA, "size_bytes": written})
}

func sourceFetchFailureIsPermanent(message string) bool {
	value := strings.ToLower(strings.TrimSpace(message))
	for _, marker := range []string{
		"パスワード", "画像認証", "追加の確認操作", "自動判定できません", "ログインしてください",
		"source file exceeds", "exceeded the 5 gib limit", "returned http 400", "returned http 401",
		"returned http 403", "returned http 404", "returned http 410", "http 400", "http 401", "http 403", "http 404", "http 410", "source content upload returned 409",
		"need 2 online nodes", "no online nodes", "容量", "保存先として使えるオンラインノード",
	} {
		if strings.Contains(value, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func recomputeFilePreservation(tx *gorm.DB, fileID string) error {
	var healthy int64
	if err := tx.Table("placements AS p").Joins("JOIN nodes AS n ON n.id = p.node_id").Where("p.file_object_id = ? AND p.status = 'verified' AND n.status = 'online' AND n.last_seen_at >= now() - interval '2 minutes'", fileID).Count(&healthy).Error; err != nil {
		return err
	}
	var active int64
	if err := tx.Model(&model.Task{}).Where("file_object_id = ? AND status IN ('queued', 'running')", fileID).Count(&active).Error; err != nil {
		return err
	}
	var missing int64
	if err := tx.Model(&model.Placement{}).Where("file_object_id = ? AND status = 'missing'", fileID).Count(&missing).Error; err != nil {
		return err
	}
	var offlineVerified int64
	if err := tx.Table("placements AS p").Joins("JOIN nodes AS n ON n.id = p.node_id").Where("p.file_object_id = ? AND p.status = 'verified' AND (n.status <> 'online' OR n.last_seen_at IS NULL OR n.last_seen_at < now() - interval '2 minutes')", fileID).Count(&offlineVerified).Error; err != nil {
		return err
	}
	status := "queued"
	if healthy >= 2 {
		status = "verified"
	} else if active == 0 && healthy == 1 {
		status = "degraded"
	} else if active == 0 && healthy == 0 && (missing > 0 || offlineVerified > 0) {
		status = "missing"
	} else if active == 0 && healthy == 0 {
		status = "error"
	}
	return tx.Model(&model.FileObject{}).Where("id = ?", fileID).Update("preservation_status", status).Error
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringPtr(value string) *string { return &value }
