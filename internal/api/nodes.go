package api

import (
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
	"github.com/uploader-jp/uploader-archive/internal/gen/model"
	"gorm.io/gorm"
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
	if s.cfg.NodeEnrollmentToken == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "node enrollment is disabled"})
		return
	}
	enrollment := c.GetHeader("X-Node-Enrollment-Token")
	if len(enrollment) != len(s.cfg.NodeEnrollmentToken) || subtle.ConstantTimeCompare([]byte(enrollment), []byte(s.cfg.NodeEnrollmentToken)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid enrollment token"})
		return
	}
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
	token, err := newNodeToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not issue node credential"})
		return
	}
	node := model.Node{
		Name: strings.TrimSpace(req.Name), NodeType: req.NodeType, Status: "offline",
		RequestedCapacityBytes: req.RequestedCapacityBytes, AgentVersion: stringPtr(strings.TrimSpace(req.AgentVersion)),
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not register node"})
		return
	}
	if err := s.db.WithContext(c.Request.Context()).Exec("UPDATE nodes SET api_token_hash = ?, updated_at = now() WHERE id = ?", hashNodeToken(token), node.ID).Error; err != nil {
		_ = s.db.Delete(&node).Error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not store node credential"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"node":       gin.H{"id": node.ID, "name": node.Name, "node_type": node.NodeType, "status": node.Status, "requested_capacity_bytes": node.RequestedCapacityBytes},
		"node_token": token,
		"warning":    "node_token is shown only once; store it securely",
	})
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
		LastSeenAt             *time.Time `json:"last_seen_at,omitempty"`
		AgentVersion           *string    `json:"agent_version,omitempty"`
		CreatedAt              time.Time  `json:"created_at"`
		UpdatedAt              time.Time  `json:"updated_at"`
	}
	items := make([]nodeView, 0, len(nodes))
	for _, node := range nodes {
		status := node.Status
		if node.LastSeenAt == nil || time.Since(*node.LastSeenAt) > 2*time.Minute {
			status = "offline"
		}
		items = append(items, nodeView{ID: node.ID, Name: node.Name, NodeType: node.NodeType, Status: status, CapacityBytes: node.CapacityBytes, UsedBytes: node.UsedBytes, AvailableBytes: node.AvailableBytes, RequestedCapacityBytes: node.RequestedCapacityBytes, LastSeenAt: node.LastSeenAt, AgentVersion: node.AgentVersion, CreatedAt: node.CreatedAt, UpdatedAt: node.UpdatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) listTasks(c *gin.Context) {
	query := s.db.WithContext(c.Request.Context()).Model(&model.Task{})
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	var tasks []model.Task
	if err := query.Order("priority DESC, created_at ASC").Limit(500).Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list tasks"})
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
		items = append(items, taskView{ID: t.ID, TaskType: t.TaskType, FileObjectID: t.FileObjectID, SourceNodeID: t.SourceNodeID, TargetNodeID: t.TargetNodeID, Status: t.Status, Priority: t.Priority, AttemptCount: t.AttemptCount, LastError: t.LastError, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) ingestFile(c *gin.Context) {
	fileID := c.Param("id")
	if _, err := uuid.Parse(fileID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}
	var file model.FileObject
	if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ?", fileID).Error; err != nil {
		notFoundOrError(c, err, "file object")
		return
	}
	if file.PreservationStatus == "verified" {
		c.JSON(http.StatusConflict, gin.H{"error": "file object is already verified; create a new file object to ingest a different version"})
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
	node, err := s.selectNodeForCapacity(c, fh.Size)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
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

	taskID := uuid.NewString()
	stagingPath := filepath.Join(s.cfg.StagingDir, taskID)
	if err := os.Rename(tmpName, stagingPath); err != nil {
		cleanupTmp()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not finalize staging file"})
		return
	}
	sha := hex.EncodeToString(hasher.Sum(nil))
	mediaType := strings.TrimSpace(fh.Header.Get("Content-Type"))
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	storageKey := "objects/" + file.ID + "/" + taskID
	task := model.Task{ID: taskID, TaskType: "store", FileObjectID: file.ID, TargetNodeID: &node.ID, Status: "queued", Priority: int16(priority)}
	placement := model.Placement{FileObjectID: file.ID, NodeID: node.ID, StorageKey: storageKey, SHA256: &sha, Status: "queued"}
	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.FileObject{}).Where("id = ?", file.ID).Updates(map[string]any{
			"original_filename": filename, "size_bytes": written, "sha256": sha, "media_type": mediaType, "preservation_status": "queued", "public_download_enabled": false,
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		var existing model.Placement
		placementErr := tx.Where("file_object_id = ? AND node_id = ?", file.ID, node.ID).First(&existing).Error
		if errors.Is(placementErr, gorm.ErrRecordNotFound) {
			if err := tx.Create(&placement).Error; err != nil {
				return err
			}
		} else if placementErr != nil {
			return placementErr
		} else {
			if err := tx.Model(&model.Placement{}).Where("id = ?", existing.ID).Updates(map[string]any{
				"storage_key": storageKey, "sha256": sha, "status": "queued", "verified_at": nil,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_ = os.Remove(stagingPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create preservation task"})
		return
	}
	actor, _ := currentUser(c)
	s.audit(c, &actor, "file_object", file.ID, "file_object.ingest.queued", "success", stringPtr("File uploaded to temporary staging and queued for node storage"))
	c.JSON(http.StatusAccepted, gin.H{"file_object_id": file.ID, "task_id": task.ID, "node_id": node.ID, "status": "queued", "size_bytes": written, "sha256": sha, "storage_key": storageKey})
}

func (s *Server) selectNodeForCapacity(c *gin.Context, incoming int64) (model.Node, error) {
	var nodes []model.Node
	cutoff := time.Now().UTC().Add(-2 * time.Minute)
	if err := s.db.WithContext(c.Request.Context()).Where("status = ? AND last_seen_at >= ?", "online", cutoff).Order("last_seen_at DESC").Limit(100).Find(&nodes).Error; err != nil {
		return model.Node{}, errors.New("could not select a storage node")
	}
	for _, node := range nodes {
		var reserved int64
		err := s.db.WithContext(c.Request.Context()).Table("tasks AS t").Select("COALESCE(SUM(f.size_bytes), 0)").Joins("JOIN file_objects AS f ON f.id = t.file_object_id").Where("t.target_node_id = ? AND t.status IN ('queued', 'running')", node.ID).Scan(&reserved).Error
		if err != nil {
			return model.Node{}, errors.New("could not calculate reserved node capacity")
		}
		if node.AvailableBytes-reserved < incoming {
			continue
		}
		if node.RequestedCapacityBytes > 0 && node.UsedBytes+reserved+incoming > node.RequestedCapacityBytes {
			continue
		}
		return node, nil
	}
	return model.Node{}, errors.New("no online node has enough free space and participant quota; start the node agent or reduce the file size")
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
	var exhaustedTaskIDs []string
	noTask := false
	leaseToken := uuid.NewString()
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		// Capture exhausted leases so their staging files can be removed after the transaction commits.
		if err := tx.Raw(`SELECT id FROM tasks WHERE target_node_id = ? AND status = 'running' AND lease_until < now() AND attempt_count >= ? FOR UPDATE`, node.ID, maxTaskAttempts).Scan(&exhaustedTaskIDs).Error; err != nil {
			return err
		}
		// Expired leases that exhausted retries are terminal rather than stuck forever.
		if err := tx.Exec(`UPDATE tasks SET status = 'failed', lease_until = NULL, lease_token = NULL,
			last_error = COALESCE(last_error, 'task lease expired after maximum attempts'), updated_at = now()
			WHERE target_node_id = ? AND status = 'running' AND lease_until < now() AND attempt_count >= ?`, node.ID, maxTaskAttempts).Error; err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE placements SET status = 'error' WHERE node_id = ? AND file_object_id IN
			(SELECT file_object_id FROM tasks WHERE target_node_id = ? AND status = 'failed' AND last_error = 'task lease expired after maximum attempts')`, node.ID, node.ID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE file_objects SET preservation_status = 'error'
			WHERE id IN (SELECT file_object_id FROM tasks WHERE target_node_id = ? AND status = 'failed' AND last_error = 'task lease expired after maximum attempts')
			AND preservation_status = 'queued'`, node.ID).Error; err != nil {
			return err
		}
		err := tx.Raw(`SELECT id, task_type, file_object_id, source_node_id, target_node_id, status, priority, attempt_count, last_error, created_at, updated_at
			FROM tasks WHERE target_node_id = ? AND attempt_count < ? AND
			(status = 'queued' OR (status = 'running' AND lease_until < now()))
			ORDER BY priority DESC, created_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED`, node.ID, maxTaskAttempts).Scan(&claimed.Task).Error
		if err != nil {
			return err
		}
		if claimed.Task.ID == "" {
			noTask = true
			return nil
		}
		if err := tx.Exec(`UPDATE tasks SET status = 'running', attempt_count = attempt_count + 1,
			lease_until = ?, lease_token = ?, last_error = NULL, updated_at = now() WHERE id = ?`, time.Now().UTC().Add(nodeLeaseDuration), leaseToken, claimed.Task.ID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Placement{}).Where("file_object_id = ? AND node_id = ?", claimed.Task.FileObjectID, node.ID).Update("status", "running").Error; err != nil {
			return err
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
	for _, exhaustedID := range exhaustedTaskIDs {
		_ = os.Remove(filepath.Join(s.cfg.StagingDir, exhaustedID))
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
	var placement model.Placement
	if err := s.db.WithContext(c.Request.Context()).First(&placement, "file_object_id = ? AND node_id = ?", file.ID, node.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load task placement"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"task_id": claimed.Task.ID, "lease_token": claimed.LeaseToken, "file_object_id": file.ID,
		"original_filename": file.OriginalFilename, "size_bytes": file.SizeBytes, "sha256": file.SHA256,
		"storage_key": placement.StorageKey, "attempt_count": claimed.Task.AttemptCount,
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
	if err := s.db.WithContext(c.Request.Context()).Model(&model.Task{}).Where("id = ? AND target_node_id = ? AND status = 'running' AND lease_token = ? AND lease_until > now()", taskID, node.ID, lease).Count(&count).Error; err != nil || count != 1 {
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
	if err := s.db.WithContext(c.Request.Context()).First(&task, "id = ? AND target_node_id = ?", taskID, node.ID).Error; err != nil {
		notFoundOrError(c, err, "task")
		return
	}
	if task.Status != "running" {
		c.JSON(http.StatusConflict, gin.H{"error": "task is not running"})
		return
	}
	var leaseToken string
	if err := s.db.WithContext(c.Request.Context()).Raw("SELECT lease_token::text FROM tasks WHERE id = ?", taskID).Row().Scan(&leaseToken); err != nil || leaseToken != req.LeaseToken {
		c.JSON(http.StatusConflict, gin.H{"error": "task lease token does not match"})
		return
	}
	var file model.FileObject
	if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ?", task.FileObjectID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load task file"})
		return
	}
	if req.Status == "success" && (file.SHA256 == nil || !strings.EqualFold(*file.SHA256, req.SHA256) || file.SizeBytes != req.SizeBytes) {
		req.Status = "failure"
		req.Error = fmt.Sprintf("integrity verification failed: expected size=%d sha256=%s, received size=%d sha256=%s", file.SizeBytes, valueOrEmpty(file.SHA256), req.SizeBytes, req.SHA256)
	}
	terminal := false
	var nextStatus string
	if req.Status == "success" {
		nextStatus = "completed"
	} else if task.AttemptCount >= maxTaskAttempts {
		nextStatus = "failed"
		terminal = true
	} else {
		nextStatus = "queued"
	}
	if req.Status == "failure" && strings.TrimSpace(req.Error) == "" {
		req.Error = "node agent reported task failure"
	}
	now := time.Now().UTC()
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
			placementUpdates = map[string]any{"status": "verified", "sha256": strings.ToLower(req.SHA256), "verified_at": now}
		} else if terminal {
			placementUpdates = map[string]any{"status": "error"}
		}
		if err := tx.Model(&model.Placement{}).Where("file_object_id = ? AND node_id = ?", file.ID, node.ID).Updates(placementUpdates).Error; err != nil {
			return err
		}
		if req.Status == "success" {
			if err := tx.Model(&model.FileObject{}).Where("id = ?", file.ID).Update("preservation_status", "verified").Error; err != nil {
				return err
			}
		} else if terminal {
			if err := tx.Model(&model.FileObject{}).Where("id = ?", file.ID).Update("preservation_status", "error").Error; err != nil {
				return err
			}
		}
		result := "success"
		action := "node.task.completed"
		if req.Status == "failure" {
			result = "failure"
			action = "node.task.failed"
		}
		return tx.Create(&model.AuditLog{ActorLabel: "node:" + node.Name, TargetType: "task", TargetID: &task.ID, Action: action, Result: result, Reason: stringPtr(req.Error)}).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not record task result"})
		return
	}
	if req.Status == "success" || terminal {
		_ = os.Remove(filepath.Join(s.cfg.StagingDir, task.ID))
	}
	preservationStatus := "queued"
	if req.Status == "success" {
		preservationStatus = "verified"
	} else if terminal {
		preservationStatus = "error"
	}
	c.JSON(http.StatusOK, gin.H{"task_id": task.ID, "status": nextStatus, "retrying": nextStatus == "queued", "preservation_status": preservationStatus})
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringPtr(value string) *string { return &value }
