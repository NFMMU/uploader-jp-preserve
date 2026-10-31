package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	requiredReplicaCount   = 2
	maxHealthyReplicaCount = requiredReplicaCount + 1
	surplusReplicaCooldown = 5 * time.Minute
)

type placementStatusView struct {
	ID         string     `json:"id"`
	NodeID     string     `json:"node_id"`
	NodeName   string     `json:"node_name"`
	NodeType   string     `json:"node_type"`
	NodeStatus string     `json:"node_status"`
	Status     string     `json:"status"`
	SHA256     *string    `json:"sha256,omitempty"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (s *Server) listPlacements(c *gin.Context) {
	fileID := c.Param("id")
	if _, err := uuid.Parse(fileID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}
	var file model.MFileObject
	if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ?", fileID).Error; err != nil {
		notFoundOrError(c, err, "file object")
		return
	}
	var placements []model.TPlacement
	if err := s.db.WithContext(c.Request.Context()).Where("file_object_id = ?", fileID).Order("created_at ASC").Find(&placements).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list placements"})
		return
	}
	items := make([]placementStatusView, 0, len(placements))
	healthy := 0
	for _, placement := range placements {
		var node model.MNode
		if err := s.db.WithContext(c.Request.Context()).First(&node, "id = ?", placement.NodeID).Error; err != nil {
			continue
		}
		status := node.Status
		if node.LastSeenAt == nil || time.Since(*node.LastSeenAt) > 2*time.Minute {
			status = "offline"
		}
		if placement.Status == "verified" && status == "online" {
			healthy++
		}
		items = append(items, placementStatusView{ID: placement.ID, NodeID: node.ID, NodeName: node.Name, NodeType: node.NodeType, NodeStatus: status, Status: placement.Status, SHA256: placement.Sha256, VerifiedAt: placement.VerifiedAt, CreatedAt: placement.CreatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"file_object_id": fileID, "preservation_status": file.PreservationStatus, "required_replica_count": requiredReplicaCount, "healthy_replica_count": healthy, "items": items})
}

func (s *Server) requestReplica(c *gin.Context) {
	fileID := c.Param("id")
	if _, err := uuid.Parse(fileID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}
	var file model.MFileObject
	if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ? AND deleted_at IS NULL AND deletion_requested_at IS NULL", fileID).Error; err != nil {
		notFoundOrError(c, err, "file object")
		return
	}
	taskID, sourceID, targetID, scheduled, reason, err := s.scheduleReplicaTask(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not schedule replica task"})
		return
	}
	actor, _ := currentUser(c)
	if scheduled {
		s.audit(c, &actor, "file_object", fileID, "placement.replication.requested", "success", stringPtr("Scheduled a replica transfer from node "+sourceID+" to node "+targetID))
	}
	status := http.StatusOK
	if scheduled {
		status = http.StatusAccepted
	}
	c.JSON(status, gin.H{"file_object_id": fileID, "scheduled": scheduled, "task_id": taskID, "source_node_id": sourceID, "target_node_id": targetID, "reason": reason})
}

func (s *Server) reconcileNow(c *gin.Context) {
	if err := s.reconcilePlacements(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "placement reconciliation failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "reconciled"})
}

func (s *Server) runPlacementReconciler() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		if err := s.reconcilePlacements(ctx); err != nil && s.logger != nil {
			s.logger.Warn("placement_reconcile_failed", "error", err)
		}
		cancel()
	}
}

func (s *Server) reconcilePlacements(ctx context.Context) error {
	db := s.db.WithContext(ctx)
	cutoff := time.Now().UTC().Add(-2 * time.Minute)
	if err := db.Exec("UPDATE nodes SET status = 'offline', online_since = NULL, updated_at = now() WHERE status <> 'offline' AND (last_seen_at IS NULL OR last_seen_at < ?)", cutoff).Error; err != nil {
		return err
	}
	if err := db.Exec(`UPDATE tasks AS t SET status='failed', last_error=COALESCE(t.last_error, 'node went offline during placement task'), lease_until=NULL, lease_token=NULL, updated_at=now()
		WHERE t.status IN ('queued','running') AND ((t.task_type IN ('store','delete') AND t.target_node_id IN (SELECT id FROM m_nodes WHERE status='offline')) OR (t.task_type='replicate' AND t.source_node_id IN (SELECT id FROM m_nodes WHERE status='offline')))`).Error; err != nil {
		return err
	}
	if err := db.Exec(`UPDATE placements SET status='missing' WHERE node_id IN (SELECT id FROM m_nodes WHERE status='offline') AND status IN ('queued','running','stored','deleting')`).Error; err != nil {
		return err
	}
	var fileIDs []string
	if err := db.Raw("SELECT id FROM m_file_objects WHERE deleted_at IS NULL AND deletion_requested_at IS NULL AND preservation_status IN ('verified','degraded','missing','error','queued','stored') ORDER BY created_at ASC LIMIT 10000").Scan(&fileIDs).Error; err != nil {
		return err
	}
	for _, fileID := range fileIDs {
		if err := db.Transaction(func(tx *gorm.DB) error { return recomputeFilePreservation(tx, fileID) }); err != nil {
			return err
		}
		_, _, _, _, _, err := s.scheduleReplicaTask(ctx, fileID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := s.scheduleSurplusPlacementDeletion(ctx, fileID); err != nil {
			return err
		}
	}
	return nil
}

// shouldQueueSurplusDeletion keeps one spare healthy replica beyond the required minimum.
// It queues at most one deletion task per file at a time so repeated reconciliation is idempotent.
func shouldQueueSurplusDeletion(healthyCount, activeTaskCount int64) bool {
	return healthyCount > maxHealthyReplicaCount && activeTaskCount == 0
}

func (s *Server) scheduleSurplusPlacementDeletion(ctx context.Context, fileID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var file model.MFileObject
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&file, "id = ?", fileID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		var activeTasks int64
		if err := tx.Model(&model.TTask{}).Where("file_object_id = ? AND status IN ('queued', 'running')", fileID).Count(&activeTasks).Error; err != nil {
			return err
		}
		if activeTasks > 0 {
			return nil
		}

		cutoff := time.Now().UTC().Add(-2 * time.Minute)
		type candidate struct {
			PlacementID string
			NodeID      string
			NodeName    string
			VerifiedAt  *time.Time
			OnlineSince *time.Time
		}
		var healthy []candidate
		if err := tx.Table("placements AS p").
			Select("p.id AS placement_id, p.node_id, n.name AS node_name, p.verified_at, n.online_since").
			Joins("JOIN m_nodes AS n ON n.id = p.node_id").
			Where("p.file_object_id = ? AND p.status = 'verified' AND n.status = 'online' AND n.last_seen_at >= ?", fileID, cutoff).
			Order("p.verified_at DESC NULLS LAST, p.created_at DESC").Find(&healthy).Error; err != nil {
			return err
		}
		if !shouldQueueSurplusDeletion(int64(len(healthy)), activeTasks) {
			return nil
		}

		// Wait until every online node holding this file has remained continuously
		// online for five minutes. This prevents cleanup immediately after a node recovers.
		stableCutoff := time.Now().UTC().Add(-surplusReplicaCooldown)
		for _, placement := range healthy {
			if placement.OnlineSince == nil || placement.OnlineSince.After(stableCutoff) {
				return nil
			}
		}
		// Candidates are sorted newest-first. Do not remove a newly verified copy until
		// it has itself been stable for the same cooldown period.
		candidateToDelete := healthy[0]
		if candidateToDelete.VerifiedAt == nil || candidateToDelete.VerifiedAt.After(stableCutoff) {
			return nil
		}

		result := tx.Model(&model.TPlacement{}).Where("id = ? AND status = 'verified'", candidateToDelete.PlacementID).Update("status", "deleting")
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}

		taskID := uuid.NewString()
		targetNodeID := candidateToDelete.NodeID
		task := model.TTask{ID: taskID, TaskType: "delete", FileObjectID: fileID, TargetNodeID: &targetNodeID, Status: "queued", Priority: 0}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		return tx.Create(&model.TAuditLog{
			ActorLabel: "system:placement-reconciler", TargetType: "file_object", TargetID: &fileID,
			Action: "placement.replica.deletion_queued", Result: "success",
			Reason: stringPtr("Queued deletion of surplus verified replica on node " + candidateToDelete.NodeName + "; retaining at most one spare beyond the required replica count"),
		}).Error
	})
}

func (s *Server) scheduleReplicaTask(ctx context.Context, fileID string) (taskID, sourceNodeID, targetNodeID string, scheduled bool, reason string, err error) {
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var file model.MFileObject
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&file, "id = ?", fileID).Error; err != nil {
			return err
		}
		if value, _ := file.DeletedAt.Value(); file.DeletionRequestedAt != nil || value != nil {
			reason = "file deletion is requested or already complete"
			return nil
		}
		cutoff := time.Now().UTC().Add(-2 * time.Minute)
		var placements []model.TPlacement
		if err := tx.Where("file_object_id = ?", fileID).Find(&placements).Error; err != nil {
			return err
		}
		placementByNode := make(map[string]model.TPlacement, len(placements))
		for _, p := range placements {
			placementByNode[p.NodeID] = p
		}
		var healthySources []model.TPlacement
		if err := tx.Table("placements AS p").Select("p.*").Joins("JOIN m_nodes AS n ON n.id = p.node_id").Where("p.file_object_id = ? AND p.status = 'verified' AND n.status = 'online' AND n.last_seen_at >= ?", fileID, cutoff).Order("p.verified_at DESC NULLS LAST").Find(&healthySources).Error; err != nil {
			return err
		}
		var active int64
		if err := tx.Model(&model.TTask{}).Where("file_object_id = ? AND status IN ('queued','running')", fileID).Count(&active).Error; err != nil {
			return err
		}
		if int64(len(healthySources))+active >= requiredReplicaCount {
			reason = "required replica count is already satisfied or tasks are active"
			return nil
		}
		if len(healthySources) == 0 {
			reason = "no online verified source placement is available"
			return nil
		}
		var nodes []model.MNode
		if err := tx.Where("status = 'online' AND last_seen_at >= ?", cutoff).Order("available_bytes DESC, last_seen_at DESC").Limit(500).Find(&nodes).Error; err != nil {
			return err
		}
		var source model.MNode
		if err := tx.First(&source, "id = ?", healthySources[0].NodeID).Error; err != nil {
			return err
		}
		var target *model.MNode
		for _, candidate := range nodes {
			if candidate.ID == source.ID {
				continue
			}
			if p, exists := placementByNode[candidate.ID]; exists && p.Status == "verified" {
				continue
			}
			var reserved int64
			if err := tx.Table("tasks AS t").Select("COALESCE(SUM(f.size_bytes), 0)").Joins("JOIN m_file_objects AS f ON f.id=t.file_object_id").Where("t.target_node_id = ? AND t.status IN ('queued','running')", candidate.ID).Scan(&reserved).Error; err != nil {
				return err
			}
			if candidate.AvailableBytes-reserved < file.SizeBytes {
				continue
			}
			if candidate.RequestedCapacityBytes > 0 && candidate.UsedBytes+reserved+file.SizeBytes > candidate.RequestedCapacityBytes {
				continue
			}
			var targetActive int64
			if err := tx.Model(&model.TTask{}).Where("file_object_id = ? AND target_node_id = ? AND status IN ('queued','running')", fileID, candidate.ID).Count(&targetActive).Error; err != nil {
				return err
			}
			if targetActive > 0 {
				continue
			}
			copy := candidate
			target = &copy
			break
		}
		if target == nil {
			reason = "no additional online node has sufficient free capacity"
			return nil
		}
		newTaskID := uuid.NewString()
		task := model.TTask{ID: newTaskID, TaskType: "replicate", FileObjectID: fileID, SourceNodeID: &source.ID, TargetNodeID: &target.ID, Status: "queued", Priority: 3}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		storageKey := "objects/" + fileID + "/" + newTaskID
		if existing, ok := placementByNode[target.ID]; ok {
			storageKey = existing.StorageKey
			if err := tx.Model(&model.TPlacement{}).Where("id = ?", existing.ID).Updates(map[string]any{"storage_key": storageKey, "sha256": file.Sha256, "status": "queued", "verified_at": nil}).Error; err != nil {
				return err
			}
		} else if err := tx.Create(&model.TPlacement{FileObjectID: fileID, NodeID: target.ID, StorageKey: storageKey, Sha256: file.Sha256, Status: "queued"}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.MFileObject{}).Where("id = ?", fileID).Update("preservation_status", "queued").Error; err != nil {
			return err
		}
		if err := tx.Create(&model.TAuditLog{ActorLabel: "system:placement-reconciler", TargetType: "file_object", TargetID: &fileID, Action: "placement.replication.queued", Result: "success", Reason: stringPtr("Scheduled replica from " + source.Name + " to " + target.Name)}).Error; err != nil {
			return err
		}
		taskID, sourceNodeID, targetNodeID, scheduled, reason = newTaskID, source.ID, target.ID, true, "replica task queued"
		return nil
	})
	return
}
