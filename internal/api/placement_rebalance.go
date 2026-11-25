package api

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
)

// Capacity rebalancing moves a verified replica off a node that is filling up
// onto a less-used node. The move is always "replicate first, delete later":
// the source copy is only marked with retire_requested_at, and it is deleted
// after the replacement has been verified and only while enough stable copies
// remain. A move never reduces the live copy count below requiredReplicaCount.
const (
	rebalanceHotUtilization = 0.80
	// Percentage points that a destination must be below the source.
	rebalanceMinUtilizationGapPercent = int64(20)
	rebalancePerFileCooldown          = 15 * time.Minute
	maxRebalanceMovesPerRun           = 20
	maxRebalanceMovesPerNode          = 5
	rebalanceSourceNodeLimit          = 5
	rebalanceCandidateLimit           = 20
	rebalanceFinalizeLimit            = 500
)

// effectiveCapacityBytes is the quota a node allows itself: the declared
// requested capacity when set, otherwise the reported disk capacity.
func effectiveCapacityBytes(capacityBytes, requestedCapacityBytes int64) int64 {
	if requestedCapacityBytes > 0 {
		return requestedCapacityBytes
	}
	return capacityBytes
}

// utilizationGapAtLeast reports whether the target is at least the configured
// number of percentage points below the source, using exact rational arithmetic
// so a gap of exactly 20 points is not excluded by floating-point error.
func utilizationGapAtLeast(targetUsed, targetCapacity, sourceUsed, sourceCapacity int64) bool {
	if targetCapacity <= 0 || sourceCapacity <= 0 {
		return false
	}
	target := new(big.Rat).SetFrac64(targetUsed, targetCapacity)
	source := new(big.Rat).SetFrac64(sourceUsed, sourceCapacity)
	source.Sub(source, big.NewRat(rebalanceMinUtilizationGapPercent, 100))
	return target.Cmp(source) <= 0
}

// Stable reason codes shared by the reconciler and the admin UI.
const (
	reasonDeletionPending   = "deletion_pending"
	reasonActiveTasks       = "active_tasks"
	reasonReplicasSatisfied = "replicas_satisfied"
	reasonNoHealthySource   = "no_healthy_source"
	reasonNoTargetCapacity  = "no_target_capacity"
	reasonNotExactlyTwo     = "rebalance_not_exactly_two"
	reasonUnstableCopies    = "rebalance_unstable_copies"
	reasonNoHotSource       = "rebalance_no_hot_source"
	reasonNoRebalanceTarget = "rebalance_no_target_capacity"
	reasonCooldown          = "rebalance_cooldown"
	reasonPerSourceLimit    = "rebalance_source_limit"
)

// placementReconcileStats is the per-run summary returned to the admin UI.
type placementReconcileStats struct {
	FilesScanned      int            `json:"files_scanned"`
	ReplicasScheduled int            `json:"replicas_scheduled"`
	RebalanceMoves    int            `json:"rebalance_moves"`
	DeletesQueued     int            `json:"deletes_queued"`
	MovesAbandoned    int            `json:"moves_abandoned"`
	Errors            int            `json:"errors"`
	Reasons           map[string]int `json:"reasons"`
}

func newPlacementReconcileStats() placementReconcileStats {
	return placementReconcileStats{Reasons: map[string]int{}}
}

func (p *placementReconcileStats) skip(reason string) {
	if p == nil {
		return
	}
	if p.Reasons == nil {
		p.Reasons = map[string]int{}
	}
	p.Reasons[reason]++
}

// nodeUsage is the capacity view used to compare nodes. ReservedBytes is the
// sum of sizes of tasks that will land on the node but have not been counted in
// its heartbeat yet.
type nodeUsage struct {
	ID                     string
	Name                   string
	UsedBytes              int64
	ReservedBytes          int64
	CapacityBytes          int64
	RequestedCapacityBytes int64
	AvailableBytes         int64
}

// rebalanceCopy is one healthy replica held by one node.
type rebalanceCopy struct {
	PlacementID string
	NodeID      string
	NodeName    string
	Utilization float64
	Usable      bool
	VerifiedAt  *time.Time
	OnlineSince *time.Time
}

// rebalanceTarget is a candidate destination for a relocated replica.
type rebalanceTarget struct {
	ID                     string
	Name                   string
	Utilization            float64
	Usable                 bool
	UsedBytes              int64
	ReservedBytes          int64
	CapacityBytes          int64
	RequestedCapacityBytes int64
	AvailableBytes         int64
}

// nodeUtilization returns used divided by the effective capacity. The
// participant-declared quota is used when set because exceeding it stops the
// node from accepting new copies. ok is false when the capacity is unknown, so
// such a node is never mistaken for an empty one.
func nodeUtilization(usedBytes, capacityBytes, requestedCapacityBytes int64) (float64, bool) {
	limit := capacityBytes
	if requestedCapacityBytes > 0 {
		limit = requestedCapacityBytes
	}
	if limit <= 0 {
		return 0, false
	}
	return float64(usedBytes) / float64(limit), true
}

func (u nodeUsage) utilization() (float64, bool) {
	return nodeUtilization(u.UsedBytes+u.ReservedBytes, u.CapacityBytes, u.RequestedCapacityBytes)
}

// selectRebalanceSource returns the healthy copy held by the most utilized
// node, provided that node is at or above the hot threshold. ok is false when
// no node is under enough pressure to justify moving a copy.
func selectRebalanceSource(copies []rebalanceCopy) (rebalanceCopy, bool) {
	var best rebalanceCopy
	found := false
	for _, copy := range copies {
		if !copy.Usable || copy.Utilization < rebalanceHotUtilization {
			continue
		}
		if !found || copy.Utilization > best.Utilization ||
			(copy.Utilization == best.Utilization && verifiedLater(copy.VerifiedAt, best.VerifiedAt)) {
			best, found = copy, true
		}
	}
	return best, found
}

// rankRebalanceTargets returns nodes that may receive the moved copy, ordered
// by ascending utilization. A target must be at least rebalanceMinUtilizationGap
// below the source and have room for one more copy.
func rankRebalanceTargets(nodes []rebalanceTarget, sourceID string, sourceUsed, sourceCapacity int64, occupied map[string]bool, sizeBytes int64) []rebalanceTarget {
	out := make([]rebalanceTarget, 0, len(nodes))
	for _, node := range nodes {
		if node.ID == sourceID || occupied[node.ID] || !node.Usable {
			continue
		}
		targetUsed := node.UsedBytes + node.ReservedBytes
		targetCapacity := effectiveCapacityBytes(node.CapacityBytes, node.RequestedCapacityBytes)
		if !utilizationGapAtLeast(targetUsed, targetCapacity, sourceUsed, sourceCapacity) {
			continue
		}
		if node.AvailableBytes-node.ReservedBytes < sizeBytes {
			continue
		}
		if node.RequestedCapacityBytes > 0 && node.UsedBytes+node.ReservedBytes+sizeBytes > node.RequestedCapacityBytes {
			continue
		}
		out = append(out, node)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Utilization != out[j].Utilization {
			return out[i].Utilization < out[j].Utilization
		}
		if out[i].AvailableBytes != out[j].AvailableBytes {
			return out[i].AvailableBytes > out[j].AvailableBytes
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// selectSurplusVictim picks the copy on the most utilized node so that cleaning
// up a surplus replica also reduces skew. Copies on nodes with unknown capacity
// are never chosen.
func selectSurplusVictim(copies []rebalanceCopy) (rebalanceCopy, bool) {
	var best rebalanceCopy
	found := false
	for _, copy := range copies {
		if !copy.Usable {
			continue
		}
		if !found || copy.Utilization > best.Utilization ||
			(copy.Utilization == best.Utilization && verifiedLater(copy.VerifiedAt, best.VerifiedAt)) {
			best, found = copy, true
		}
	}
	return best, found
}

// deletionMinimumHealthy is the number of other stable healthy copies required
// before a delete may run. A rebalance deletion (marked placement) may leave
// the required replica count; a surplus deletion keeps the extra spare.
func deletionMinimumHealthy(retireRequestedAt *time.Time) int64 {
	if retireRequestedAt != nil {
		return requiredReplicaCount
	}
	return requiredReplicaCount + 1
}

func deletionSafetySatisfied(healthy, unstable, minimum int64) bool {
	if minimum < requiredReplicaCount {
		minimum = requiredReplicaCount
	}
	return healthy >= minimum && unstable == 0
}

func verifiedLater(a, b *time.Time) bool {
	if a == nil {
		return false
	}
	return b == nil || a.After(*b)
}

// scheduleRebalanceMove queues a replicate task from a hot node to a colder one
// and marks the source replica for retirement. The marker alone deletes
// nothing; finalizeRetireMoves waits for the replacement to be verified.
func (s *Server) scheduleRebalanceMove(ctx context.Context, fileID string, perSource map[string]int) (reason string, scheduled bool, sourceNodeID string, err error) {
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var file model.FileObject
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&file, "id = ?", fileID).Error; err != nil {
			return err
		}
		if file.DeletedAt.Valid || file.DeletionRequestedAt != nil {
			reason = reasonDeletionPending
			return nil
		}
		var active int64
		if err := tx.Model(&model.Task{}).Where("file_object_id = ? AND status IN ('queued','running')", fileID).Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			reason = reasonActiveTasks
			return nil
		}

		now := time.Now().UTC()
		freshCutoff := now.Add(-2 * time.Minute)
		stableCutoff := now.Add(-surplusReplicaCooldown)

		var placements []model.Placement
		if err := tx.Where("file_object_id = ?", fileID).Find(&placements).Error; err != nil {
			return err
		}
		placementByNode := make(map[string]model.Placement, len(placements))
		occupied := make(map[string]bool, len(placements))
		for _, placement := range placements {
			placementByNode[placement.NodeID] = placement
			// A deleted or failed placement no longer counts as a usable copy, so
			// its node is a valid destination again; the row is reused below.
			if placement.Status != "deleted" && placement.Status != "error" {
				occupied[placement.NodeID] = true
			}
		}

		var healthy []struct {
			PlacementID string
			NodeID      string
			NodeName    string
			VerifiedAt  *time.Time
			OnlineSince *time.Time
		}
		if err := tx.Raw(`
			SELECT p.id AS placement_id, p.node_id, n.name AS node_name, p.verified_at, n.online_since
			FROM placements AS p
			JOIN nodes AS n ON n.id = p.node_id
			WHERE p.file_object_id = ? AND p.status = 'verified'
			  AND p.retire_requested_at IS NULL
			  AND n.status = 'online' AND n.last_seen_at >= ?
			ORDER BY p.verified_at DESC NULLS LAST
		`, fileID, freshCutoff).Scan(&healthy).Error; err != nil {
			return err
		}
		if len(healthy) != requiredReplicaCount {
			reason = reasonNotExactlyTwo
			return nil
		}
		for _, copy := range healthy {
			if copy.OnlineSince == nil || copy.OnlineSince.After(stableCutoff) ||
				copy.VerifiedAt == nil || copy.VerifiedAt.After(stableCutoff) {
				reason = reasonUnstableCopies
				return nil
			}
		}

		var recent int64
		if err := tx.Table("audit_logs").
			Where("target_type = 'file_object' AND target_id = ? AND action = 'placement.rebalance.move.queued' AND created_at >= ?", fileID, now.Add(-rebalancePerFileCooldown)).
			Count(&recent).Error; err != nil {
			return err
		}
		if recent > 0 {
			reason = reasonCooldown
			return nil
		}

		var nodes []model.Node
		if err := tx.Where("status = 'online' AND last_seen_at >= ?", freshCutoff).Limit(500).Find(&nodes).Error; err != nil {
			return err
		}
		reserved, err := reservedBytesByNode(tx)
		if err != nil {
			return err
		}
		nodeByID := make(map[string]model.Node, len(nodes))
		for _, node := range nodes {
			nodeByID[node.ID] = node
		}
		copies := make([]rebalanceCopy, 0, len(healthy))
		for _, copy := range healthy {
			node, ok := nodeByID[copy.NodeID]
			if !ok {
				continue
			}
			usage := nodeUsage{ID: node.ID, Name: node.Name, UsedBytes: node.UsedBytes, ReservedBytes: reserved[node.ID], CapacityBytes: node.CapacityBytes, RequestedCapacityBytes: node.RequestedCapacityBytes, AvailableBytes: node.AvailableBytes}
			utilization, usable := usage.utilization()
			copies = append(copies, rebalanceCopy{PlacementID: copy.PlacementID, NodeID: copy.NodeID, NodeName: copy.NodeName, Utilization: utilization, Usable: usable, VerifiedAt: copy.VerifiedAt, OnlineSince: copy.OnlineSince})
		}
		source, ok := selectRebalanceSource(copies)
		if !ok {
			reason = reasonNoHotSource
			return nil
		}
		// The per-node cap must follow the node actually chosen as the source,
		// which can differ from the node whose candidates are being scanned.
		if perSource[source.NodeID] >= maxRebalanceMovesPerNode {
			reason = reasonPerSourceLimit
			return nil
		}

		targets := make([]rebalanceTarget, 0, len(nodes))
		for _, node := range nodes {
			usage := nodeUsage{ID: node.ID, Name: node.Name, UsedBytes: node.UsedBytes, ReservedBytes: reserved[node.ID], CapacityBytes: node.CapacityBytes, RequestedCapacityBytes: node.RequestedCapacityBytes, AvailableBytes: node.AvailableBytes}
			utilization, usable := usage.utilization()
			targets = append(targets, rebalanceTarget{ID: node.ID, Name: node.Name, Utilization: utilization, Usable: usable, UsedBytes: usage.UsedBytes, ReservedBytes: usage.ReservedBytes, CapacityBytes: usage.CapacityBytes, RequestedCapacityBytes: usage.RequestedCapacityBytes, AvailableBytes: usage.AvailableBytes})
		}
		sourceNode := nodeByID[source.NodeID]
		sourceUsed := sourceNode.UsedBytes + reserved[source.NodeID]
		sourceCapacity := effectiveCapacityBytes(sourceNode.CapacityBytes, sourceNode.RequestedCapacityBytes)
		ranked := rankRebalanceTargets(targets, source.NodeID, sourceUsed, sourceCapacity, occupied, file.SizeBytes)
		if len(ranked) == 0 {
			reason = reasonNoRebalanceTarget
			return nil
		}
		target := ranked[0]

		taskID := uuid.NewString()
		sourceNodeID := source.NodeID
		targetNodeID := target.ID
		if err := tx.Create(&model.Task{ID: taskID, TaskType: "replicate", FileObjectID: fileID, SourceNodeID: &sourceNodeID, TargetNodeID: &targetNodeID, Status: "queued", Priority: 1}).Error; err != nil {
			return err
		}
		storageKey := "objects/" + fileID + "/" + taskID
		if existing, ok := placementByNode[target.ID]; ok {
			// Reuse the row (deleted or failed) so the placements_file_node_key
			// unique index is not violated. Keep an existing storage key so a
			// failed copy is overwritten in place instead of orphaned.
			if existing.StorageKey != "" {
				storageKey = existing.StorageKey
			}
			if err := tx.Model(&model.Placement{}).Where("id = ?", existing.ID).Updates(map[string]any{
				"storage_key": storageKey, "sha256": file.Sha256, "status": "queued",
				"verified_at": nil, "retire_requested_at": nil, "move_task_id": taskID,
			}).Error; err != nil {
				return err
			}
		} else if err := tx.Create(&model.Placement{FileObjectID: fileID, NodeID: targetNodeID, StorageKey: storageKey, Sha256: file.Sha256, Status: "queued", MoveTaskID: &taskID}).Error; err != nil {
			return err
		}
		result := tx.Model(&model.Placement{}).
			Where("id = ? AND status = 'verified' AND retire_requested_at IS NULL", source.PlacementID).
			Update("retire_requested_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("rebalance source placement was no longer eligible")
		}
		reasonText := "Queued rebalance move from " + source.NodeName + " to " + target.Name
		if err := tx.Create(&model.AuditLog{ActorLabel: "system:placement-reconciler", TargetType: "file_object", TargetID: &fileID, Action: "placement.rebalance.move.queued", Result: "success", Reason: &reasonText}).Error; err != nil {
			return err
		}
		scheduled = true
		sourceNodeID = source.NodeID
		return nil
	})
	return
}

func reservedBytesByNode(tx *gorm.DB) (map[string]int64, error) {
	var rows []struct {
		NodeID   string
		Reserved int64
	}
	if err := tx.Raw(`
		SELECT t.target_node_id AS node_id, COALESCE(SUM(f.size_bytes), 0) AS reserved
		FROM tasks AS t
		JOIN file_objects AS f ON f.id = t.file_object_id
		WHERE t.status IN ('queued','running') AND t.task_type <> 'delete' AND t.target_node_id IS NOT NULL
		GROUP BY t.target_node_id
	`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.NodeID] = row.Reserved
	}
	return out, nil
}

// rebalancePlacements walks the most utilized online nodes and moves a bounded
// number of copies to colder nodes. It is only invoked for admin-triggered
// reconciliations; the periodic reconciler keeps repairing missing copies.
func (s *Server) rebalancePlacements(ctx context.Context, stats *placementReconcileStats) error {
	now := time.Now().UTC()
	freshCutoff := now.Add(-2 * time.Minute)
	var hot []struct {
		ID   string
		Name string
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, name FROM nodes
		WHERE status = 'online' AND last_seen_at >= ?
		  AND COALESCE(NULLIF(requested_capacity_bytes, 0), capacity_bytes) > 0
		  AND used_bytes::float8 / COALESCE(NULLIF(requested_capacity_bytes, 0), capacity_bytes) >= ?
		ORDER BY used_bytes::float8 / COALESCE(NULLIF(requested_capacity_bytes, 0), capacity_bytes) DESC, id
		LIMIT ?
	`, freshCutoff, rebalanceHotUtilization, rebalanceSourceNodeLimit).Scan(&hot).Error; err != nil {
		return err
	}
	if len(hot) == 0 {
		return nil
	}

	moves := 0
	perSource := make(map[string]int)
	for _, node := range hot {
		if moves >= maxRebalanceMovesPerRun {
			break
		}
		// Eligibility is filtered in SQL so a page full of ineligible files does
		// not hide movable candidates further down the list. A keyset cursor is
		// used instead of OFFSET because rows leave this result set as soon as
		// they are moved.
		var (
			cursorSize    int64
			cursorCreated time.Time
			cursorID      string
			hasCursor     bool
		)
		for {
			if moves >= maxRebalanceMovesPerRun {
				break
			}
			query := `
				SELECT p.file_object_id, f.size_bytes, p.created_at
				FROM placements AS p
				JOIN file_objects AS f ON f.id = p.file_object_id
				WHERE p.node_id = ? AND p.status = 'verified' AND p.retire_requested_at IS NULL
				  AND f.deleted_at IS NULL AND f.deletion_requested_at IS NULL
				  AND NOT EXISTS (
				    SELECT 1 FROM tasks AS t
				    WHERE t.file_object_id = f.id AND t.status IN ('queued','running')
				  )
				  AND (
				    SELECT count(*) FROM placements AS p2
				    JOIN nodes AS n2 ON n2.id = p2.node_id
				    WHERE p2.file_object_id = f.id AND p2.status = 'verified'
				      AND p2.retire_requested_at IS NULL
				      AND n2.status = 'online' AND n2.last_seen_at >= ?
				  ) = ?`
			args := []any{node.ID, freshCutoff, requiredReplicaCount}
			if hasCursor {
				query += `
				  AND (
				    f.size_bytes < ?
				    OR (f.size_bytes = ? AND p.created_at > ?)
				    OR (f.size_bytes = ? AND p.created_at = ? AND f.id > ?)
				  )`
				args = append(args, cursorSize, cursorSize, cursorCreated, cursorSize, cursorCreated, cursorID)
			}
			query += " ORDER BY f.size_bytes DESC, p.created_at ASC, f.id ASC LIMIT ?"
			args = append(args, rebalanceCandidateLimit)

			var candidates []struct {
				FileObjectID string
				SizeBytes    int64
				CreatedAt    time.Time
			}
			if err := s.db.WithContext(ctx).Raw(query, args...).Scan(&candidates).Error; err != nil {
				return err
			}
			for _, candidate := range candidates {
				if moves >= maxRebalanceMovesPerRun {
					break
				}
				reason, scheduled, sourceID, err := s.scheduleRebalanceMove(ctx, candidate.FileObjectID, perSource)
				if err != nil {
					if stats != nil {
						stats.Errors++
					}
					if s.logger != nil {
						s.logger.Warn("placement_rebalance_failed", "file_object_id", candidate.FileObjectID, "error", err)
					}
					continue
				}
				if scheduled {
					moves++
					perSource[sourceID]++
					if stats != nil {
						stats.RebalanceMoves++
					}
					continue
				}
				stats.skip(reason)
			}
			if len(candidates) < rebalanceCandidateLimit {
				break
			}
			last := candidates[len(candidates)-1]
			cursorSize, cursorCreated, cursorID, hasCursor = last.SizeBytes, last.CreatedAt, last.FileObjectID, true
		}
	}
	return nil
}

// finalizeRetireMoves queues the delete task for replicas whose replacement has
// been verified. Every step re-checks the copy count under locks.
func (s *Server) finalizeRetireMoves(ctx context.Context, stats *placementReconcileStats) error {
	var rows []struct {
		PlacementID       string
		FileObjectID      string
		NodeID            string
		RetireRequestedAt time.Time
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id AS placement_id, file_object_id, node_id, retire_requested_at
		FROM placements
		WHERE retire_requested_at IS NOT NULL AND status = 'verified'
		ORDER BY retire_requested_at ASC
		LIMIT ?
	`, rebalanceFinalizeLimit).Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		outcome, err := s.finalizeRetireMove(ctx, row.FileObjectID, row.PlacementID, row.NodeID, row.RetireRequestedAt)
		if err != nil {
			if stats != nil {
				stats.Errors++
			}
			if s.logger != nil {
				s.logger.Warn("placement_retire_failed", "file_object_id", row.FileObjectID, "error", err)
			}
			continue
		}
		switch outcome {
		case retireAbandoned:
			if stats != nil {
				stats.MovesAbandoned++
			}
		case retireDeletionQueued:
			if stats != nil {
				stats.DeletesQueued++
			}
		}
	}
	return nil
}

// retireOutcome is applied to the run counters only after the transaction
// commits, so a rollback cannot inflate the numbers shown to the admin.
type retireOutcome int

const (
	retireNone retireOutcome = iota
	retireDeletionQueued
	retireAbandoned
)

func (s *Server) finalizeRetireMove(ctx context.Context, fileID, placementID, nodeID string, requestedAt time.Time) (retireOutcome, error) {
	outcome := retireNone
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var file model.FileObject
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&file, "id = ?", fileID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if file.DeletedAt.Valid || file.DeletionRequestedAt != nil {
			return tx.Model(&model.Placement{}).Where("id = ?", placementID).Update("retire_requested_at", nil).Error
		}
		var active int64
		if err := tx.Model(&model.Task{}).Where("file_object_id = ? AND status IN ('queued','running')", fileID).Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return nil
		}

		now := time.Now().UTC()
		freshCutoff := now.Add(-2 * time.Minute)
		stableCutoff := now.Add(-surplusReplicaCooldown)
		var replacement *struct {
			PlacementID string
			NodeID      string
			NodeName    string
			VerifiedAt  *time.Time
			OnlineSince *time.Time
		}
		var found []struct {
			PlacementID string
			NodeID      string
			NodeName    string
			VerifiedAt  *time.Time
			OnlineSince *time.Time
		}
		if err := tx.Raw(`
			SELECT p.id AS placement_id, p.node_id, n.name AS node_name, p.verified_at, n.online_since
			FROM placements AS p
			JOIN nodes AS n ON n.id = p.node_id
			WHERE p.file_object_id = ? AND p.node_id <> ? AND p.status = 'verified'
			  AND p.verified_at IS NOT NULL AND p.verified_at >= ?
			  AND n.status = 'online' AND n.last_seen_at >= ?
			ORDER BY p.verified_at DESC
			LIMIT 1
		`, fileID, nodeID, requestedAt, freshCutoff).Scan(&found).Error; err != nil {
			return err
		}
		if len(found) > 0 {
			replacement = &found[0]
		}
		if replacement == nil {
			var pending int64
			if err := tx.Model(&model.Placement{}).
				Where("file_object_id = ? AND node_id <> ? AND status IN ('queued','running','stored')", fileID, nodeID).
				Count(&pending).Error; err != nil {
				return err
			}
			// A replication task that failed for good leaves its target placement
			// queued forever. Identify this move's own task through the explicit
			// move_task_id link; timestamps and storage keys are not reliable.
			var failedReplicate int64
			if err := tx.Raw(`
				SELECT count(*) FROM placements AS p
				JOIN tasks AS t ON t.id = p.move_task_id
				WHERE p.file_object_id = ? AND p.status IN ('queued','running','stored')
				  AND t.status = 'failed'
			`, fileID).Scan(&failedReplicate).Error; err != nil {
				return err
			}
			if pending == 0 || failedReplicate > 0 {
				if err := tx.Model(&model.Placement{}).
					Where("file_object_id = ? AND node_id <> ? AND status IN ('queued','running','stored')", fileID, nodeID).
					Update("status", "error").Error; err != nil {
					return err
				}
				reason := "Rebalance move was abandoned before a replacement copy was verified"
				if err := tx.Model(&model.Placement{}).Where("id = ?", placementID).Update("retire_requested_at", nil).Error; err != nil {
					return err
				}
				outcome = retireAbandoned
				return tx.Create(&model.AuditLog{ActorLabel: "system:placement-reconciler", TargetType: "file_object", TargetID: &fileID, Action: "placement.rebalance.move.abandoned", Result: "success", Reason: &reason}).Error
			}
			return nil
		}
		if replacement.OnlineSince == nil || replacement.OnlineSince.After(stableCutoff) ||
			replacement.VerifiedAt == nil || replacement.VerifiedAt.After(stableCutoff) {
			return nil
		}

		safe, err := surplusDeletionSafeTx(tx, fileID, nodeID, now, true, requiredReplicaCount)
		if err != nil {
			return err
		}
		if !safe {
			return nil
		}

		result := tx.Model(&model.Placement{}).
			Where("id = ? AND status = 'verified' AND retire_requested_at IS NOT NULL", placementID).
			Update("status", "deleting")
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		targetNodeID := nodeID
		if err := tx.Create(&model.Task{ID: uuid.NewString(), TaskType: "delete", FileObjectID: fileID, TargetNodeID: &targetNodeID, Status: "queued", Priority: 0}).Error; err != nil {
			return err
		}
		reason := "Queued deletion of the relocated replica on node " + replacement.NodeName + " replacement"
		outcome = retireDeletionQueued
		return tx.Create(&model.AuditLog{ActorLabel: "system:placement-reconciler", TargetType: "file_object", TargetID: &fileID, Action: "placement.rebalance.replica.deletion_queued", Result: "success", Reason: &reason}).Error
	})
	if err != nil {
		return retireNone, err
	}
	return outcome, nil
}

// sortNodesByAscendingUtilization orders repair targets so that the least used
// node comes first. Nodes with unknown capacity are placed last.
func sortNodesByAscendingUtilization(nodes []model.Node) {
	sort.SliceStable(nodes, func(i, j int) bool {
		ui, knownI := nodeUtilization(nodes[i].UsedBytes, nodes[i].CapacityBytes, nodes[i].RequestedCapacityBytes)
		uj, knownJ := nodeUtilization(nodes[j].UsedBytes, nodes[j].CapacityBytes, nodes[j].RequestedCapacityBytes)
		if knownI != knownJ {
			return knownI
		}
		if ui != uj {
			return ui < uj
		}
		if nodes[i].AvailableBytes != nodes[j].AvailableBytes {
			return nodes[i].AvailableBytes > nodes[j].AvailableBytes
		}
		return nodes[i].ID < nodes[j].ID
	})
}

const (
	manualReconcileTimeout    = 10 * time.Minute
	scheduledReconcileTimeout = 25 * time.Second
)

// placementReconcileSnapshot is the mutable state of the single in-flight
// placement reconciliation. The zero value means "idle".
type placementReconcileSnapshot struct {
	Running    bool
	RunID      string
	Trigger    string
	Actor      string
	StartedAt  time.Time
	FinishedAt *time.Time
	Error      string
	Counts     placementReconcileStats
}

// reconcileSemaphore serializes actual runs. It is created lazily so that a
// zero-value Server (used by tests) still works.
func (s *Server) reconcileSemaphore() chan struct{} {
	s.placementReconcileMu.Lock()
	defer s.placementReconcileMu.Unlock()
	if s.placementReconcileGate == nil {
		s.placementReconcileGate = make(chan struct{}, 1)
	}
	return s.placementReconcileGate
}

// reserveManualReconcile claims the status for an admin-triggered run. A queued
// manual run is allowed to wait for a periodic run to finish; a second manual
// click is rejected.
func (s *Server) reserveManualReconcile(actor string) (string, bool) {
	s.placementReconcileMu.Lock()
	defer s.placementReconcileMu.Unlock()
	if s.placementReconcileState.Running && s.placementReconcileState.Trigger == "manual" {
		return "", false
	}
	runID := uuid.NewString()
	s.placementReconcileState = placementReconcileSnapshot{
		Running:   true,
		RunID:     runID,
		Trigger:   "manual",
		Actor:     actor,
		StartedAt: time.Now().UTC(),
		Counts:    newPlacementReconcileStats(),
	}
	return runID, true
}

func (s *Server) reserveScheduledReconcile() (string, bool) {
	s.placementReconcileMu.Lock()
	defer s.placementReconcileMu.Unlock()
	if s.placementReconcileState.Running {
		return "", false
	}
	runID := uuid.NewString()
	s.placementReconcileState = placementReconcileSnapshot{
		Running:   true,
		RunID:     runID,
		Trigger:   "scheduled",
		Actor:     "system:placement-reconciler",
		StartedAt: time.Now().UTC(),
		Counts:    newPlacementReconcileStats(),
	}
	return runID, true
}

func (s *Server) finishReconcile(runID string, counts placementReconcileStats, err error) {
	s.placementReconcileMu.Lock()
	defer s.placementReconcileMu.Unlock()
	if s.placementReconcileState.RunID != runID {
		return
	}
	finished := time.Now().UTC()
	s.placementReconcileState.Running = false
	s.placementReconcileState.FinishedAt = &finished
	s.placementReconcileState.Counts = counts
	if counts.Reasons == nil {
		s.placementReconcileState.Counts.Reasons = map[string]int{}
	}
	if err != nil {
		s.placementReconcileState.Error = err.Error()
	}
	if s.placementReconcileState.Trigger == "manual" {
		s.lastManualReconcile = s.placementReconcileState
	}
	s.reconcileHistory = append(s.reconcileHistory, s.placementReconcileState)
	if len(s.reconcileHistory) > 10 {
		s.reconcileHistory = s.reconcileHistory[len(s.reconcileHistory)-10:]
	}
}

func (s *Server) currentReconcileRunID() string {
	s.placementReconcileMu.Lock()
	defer s.placementReconcileMu.Unlock()
	return s.placementReconcileState.RunID
}

// executeReconcile runs one reconciliation under the shared gate so that the
// periodic pass and the admin button can never overlap.
func (s *Server) executeReconcile(runID string, timeout time.Duration, allowRebalance bool) {
	gate := s.reconcileSemaphore()
	gate <- struct{}{}
	defer func() { <-gate }()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	stats := newPlacementReconcileStats()
	err := s.reconcilePlacementsInto(ctx, &stats, allowRebalance)
	s.finishReconcile(runID, stats, err)
	if err != nil && s.logger != nil {
		s.logger.Warn("placement_reconcile_failed", "error", err, "run_id", runID)
	}
}

func (s *Server) reconcileSnapshot(runID string) gin.H {
	s.placementReconcileMu.Lock()
	state := s.placementReconcileState
	lastManual := s.lastManualReconcile
	history := append([]placementReconcileSnapshot(nil), s.reconcileHistory...)
	s.placementReconcileMu.Unlock()

	if runID != "" {
		if state.RunID != runID {
			found := false
			for _, entry := range history {
				if entry.RunID == runID {
					state, found = entry, true
					break
				}
			}
			if !found {
				return gin.H{"status": "unknown", "run_id": runID}
			}
		}
	} else if !(state.Running && state.Trigger == "manual") && lastManual.RunID != "" {
		// The screen polls after an admin click, so expose that run's result even
		// when a periodic run starts or finishes before the next poll.
		state = lastManual
	}

	counts := state.Counts
	if counts.Reasons == nil {
		counts.Reasons = map[string]int{}
	}
	status := "idle"
	switch {
	case state.Running:
		status = "running"
	case state.FinishedAt == nil:
		status = "idle"
	case state.Error != "":
		status = "failed"
	default:
		status = "completed"
	}
	out := gin.H{
		"status":  status,
		"run_id":  state.RunID,
		"trigger": state.Trigger,
		"actor":   state.Actor,
		"counts":  counts,
		"reasons": counts.Reasons,
	}
	if !state.StartedAt.IsZero() {
		out["started_at"] = state.StartedAt
		if state.Running {
			out["elapsed_seconds"] = time.Since(state.StartedAt).Seconds()
		}
	}
	if state.FinishedAt != nil {
		out["finished_at"] = *state.FinishedAt
		out["elapsed_seconds"] = state.FinishedAt.Sub(state.StartedAt).Seconds()
	}
	if state.Error != "" {
		out["error"] = state.Error
	}
	return out
}
