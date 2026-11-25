package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
)

// TestRebalanceMoveIntegration walks a capacity move end to end: a copy is
// replicated off the hot node, its source is marked, the delete is queued only
// once the replacement is verified, and the claim still requires two remaining
// stable copies.
func TestRebalanceMoveIntegration(t *testing.T) {
	db := retryTestDB(t)
	s, file, hot, cold, spare := rebalanceFixture(t, db)
	_ = cold

	stats := newPlacementReconcileStats()
	if err := s.reconcilePlacementsInto(context.Background(), &stats, true); err != nil {
		t.Fatalf("reconcile with rebalance: %v", err)
	}
	if stats.RebalanceMoves != 1 {
		t.Fatalf("rebalance moves = %d, want 1 (stats=%+v)", stats.RebalanceMoves, stats)
	}

	var move struct {
		ID           string
		TaskType     string
		SourceNodeID string
		TargetNodeID string
		Priority     int16
		Status       string
	}
	if err := db.Raw("SELECT id, task_type, source_node_id, target_node_id, priority, status FROM tasks WHERE file_object_id = ? AND task_type = 'replicate'", file.ID).Scan(&move).Error; err != nil {
		t.Fatalf("read move task: %v", err)
	}
	if move.SourceNodeID != hot.ID || move.TargetNodeID != spare.ID || move.Priority != 1 || move.Status != "queued" {
		t.Fatalf("move task = %+v, want replicate hot -> spare with priority 1", move)
	}

	var hotPlacement struct {
		Marker *time.Time
		Status string
	}
	if err := db.Raw("SELECT retire_requested_at AS marker, status FROM placements WHERE file_object_id = ? AND node_id = ?", file.ID, hot.ID).Scan(&hotPlacement).Error; err != nil {
		t.Fatalf("read hot placement: %v", err)
	}
	marker := hotPlacement.Marker
	if marker == nil || hotPlacement.Status != "verified" {
		t.Fatalf("hot placement = marker %v status %q, want a marker on a still-verified copy", marker, hotPlacement.Status)
	}
	var preservation string
	if err := db.Raw("SELECT preservation_status FROM file_objects WHERE id = ?", file.ID).Scan(&preservation).Error; err != nil {
		t.Fatalf("read preservation status: %v", err)
	}
	if preservation != "verified" {
		t.Fatalf("preservation status during a move = %q, want verified", preservation)
	}

	// The replacement is verified a few minutes after the move was queued and
	// has been stable for the cooldown period.
	now := time.Now().UTC()
	requestedAt := now.Add(-10 * time.Minute)
	verifiedAt := now.Add(-6 * time.Minute)
	retryExec(t, db, "UPDATE tasks SET status = 'completed' WHERE id = ?", move.ID)
	retryExec(t, db, "UPDATE placements SET status = 'verified', verified_at = ? WHERE file_object_id = ? AND node_id = ?", verifiedAt, file.ID, spare.ID)
	retryExec(t, db, "UPDATE placements SET retire_requested_at = ? WHERE file_object_id = ? AND node_id = ?", requestedAt, file.ID, hot.ID)

	stats = newPlacementReconcileStats()
	if err := s.reconcilePlacementsInto(context.Background(), &stats, false); err != nil {
		t.Fatalf("reconcile after replacement verification: %v", err)
	}
	if stats.DeletesQueued != 1 {
		t.Fatalf("deletes queued = %d, want 1 (stats=%+v)", stats.DeletesQueued, stats)
	}

	var deleteTask struct {
		ID           string
		TargetNodeID string
		Status       string
	}
	if err := db.Raw("SELECT id, target_node_id, status FROM tasks WHERE file_object_id = ? AND task_type = 'delete'", file.ID).Scan(&deleteTask).Error; err != nil {
		t.Fatalf("read delete task: %v", err)
	}
	if deleteTask.TargetNodeID != hot.ID || deleteTask.Status != "queued" {
		t.Fatalf("delete task = %+v, want queued deletion on the hot node", deleteTask)
	}
	var afterDeleteQueued struct {
		Status string
		Marker *time.Time
	}
	if err := db.Raw("SELECT status, retire_requested_at AS marker FROM placements WHERE file_object_id = ? AND node_id = ?", file.ID, hot.ID).Scan(&afterDeleteQueued).Error; err != nil {
		t.Fatalf("read hot placement status: %v", err)
	}
	if afterDeleteQueued.Status != "deleting" {
		t.Fatalf("hot placement status = %q, want deleting", afterDeleteQueued.Status)
	}
	if afterDeleteQueued.Marker == nil {
		t.Fatal("marker must be retained until the delete completes")
	}

	response := replicaClaim(s, hot)
	if response.Code != http.StatusOK {
		t.Fatalf("rebalance deletion claim = %d %s, want 200 (two stable copies remain)", response.Code, response.Body)
	}

	var remaining int64
	if err := db.Table("placements").Where("file_object_id = ? AND status = 'verified'", file.ID).Count(&remaining).Error; err != nil {
		t.Fatalf("count copies: %v", err)
	}
	if remaining != 2 {
		t.Fatalf("verified copies = %d, want 2", remaining)
	}
}

func TestRebalanceSkipsWhenNoColdTargetExists(t *testing.T) {
	db := retryTestDB(t)
	s, file, _, _, _ := rebalanceFixture(t, db)
	retryExec(t, db, "DELETE FROM nodes WHERE name LIKE 'rebalance-spare-%'")
	// Only the two copy holders remain, so there is nowhere to move a copy.
	stats := newPlacementReconcileStats()
	if err := s.reconcilePlacementsInto(context.Background(), &stats, true); err != nil {
		t.Fatalf("reconcile without a spare node: %v", err)
	}
	if stats.RebalanceMoves != 0 {
		t.Fatalf("rebalance moves = %d, want 0", stats.RebalanceMoves)
	}
	if stats.Reasons[reasonNoRebalanceTarget] == 0 {
		t.Fatalf("reasons = %+v, want %s", stats.Reasons, reasonNoRebalanceTarget)
	}
	var count int64
	if err := db.Table("tasks").Where("file_object_id = ? AND task_type = 'replicate'", file.ID).Count(&count).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if count != 0 {
		t.Fatalf("replicate tasks = %d, want 0", count)
	}
}

func TestRebalanceMoveAbandonedWithoutReplacement(t *testing.T) {
	db := retryTestDB(t)
	s, file, hot, _, _ := rebalanceFixture(t, db)
	now := time.Now().UTC()
	retryExec(t, db, "UPDATE placements SET retire_requested_at = ? WHERE file_object_id = ? AND node_id = ?", now.Add(-10*time.Minute), file.ID, hot.ID)
	retryExec(t, db, "INSERT INTO tasks (id, task_type, file_object_id, target_node_id, status, attempt_count) VALUES (?, 'replicate', ?, ?, 'failed', 5)", uuid.NewString(), file.ID, hot.ID)

	stats := newPlacementReconcileStats()
	if err := s.reconcilePlacementsInto(context.Background(), &stats, false); err != nil {
		t.Fatalf("reconcile after failed move: %v", err)
	}
	if stats.MovesAbandoned != 1 {
		t.Fatalf("moves abandoned = %d, want 1 (stats=%+v)", stats.MovesAbandoned, stats)
	}
	var cleared struct {
		Marker *time.Time
	}
	if err := db.Raw("SELECT retire_requested_at AS marker FROM placements WHERE file_object_id = ? AND node_id = ?", file.ID, hot.ID).Scan(&cleared).Error; err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if cleared.Marker != nil {
		t.Fatal("marker must be cleared when no replacement was produced")
	}
	var deletes int64
	if err := db.Table("tasks").Where("file_object_id = ? AND task_type = 'delete'", file.ID).Count(&deletes).Error; err != nil {
		t.Fatalf("count delete tasks: %v", err)
	}
	if deletes != 0 {
		t.Fatalf("delete tasks = %d, want 0", deletes)
	}
}

func TestClaimRestoresRebalanceCopyWhenUnsafe(t *testing.T) {
	db := retryTestDB(t)
	s, file, hot, cold, _ := rebalanceFixture(t, db)
	now := time.Now().UTC()
	// The only other copy is on an offline node, so deleting the marked copy
	// would drop below the required replica count.
	retryExec(t, db, "UPDATE nodes SET status = 'offline' WHERE id = ?", cold.ID)
	retryExec(t, db, "UPDATE placements SET status = 'deleting', retire_requested_at = ? WHERE file_object_id = ? AND node_id = ?", now.Add(-10*time.Minute), file.ID, hot.ID)
	if err := db.Create(&model.Task{TaskType: "delete", FileObjectID: file.ID, TargetNodeID: &hot.ID, Status: "queued"}).Error; err != nil {
		t.Fatalf("create delete task: %v", err)
	}

	response := replicaClaim(s, hot)
	if response.Code != http.StatusNoContent {
		t.Fatalf("unsafe rebalance claim = %d %s, want 204", response.Code, response.Body)
	}
	var restored struct {
		Status string
		Marker *time.Time
	}
	if err := db.Raw("SELECT status, retire_requested_at AS marker FROM placements WHERE file_object_id = ? AND node_id = ?", file.ID, hot.ID).Scan(&restored).Error; err != nil {
		t.Fatalf("read restored placement: %v", err)
	}
	if restored.Status != "verified" || restored.Marker != nil {
		t.Fatalf("placement = %+v, want verified with no marker", restored)
	}
}

func rebalanceFixture(t *testing.T, db *gorm.DB) (s *Server, file model.FileObject, hot, cold, spare model.Node) {
	t.Helper()
	digest := strings.Repeat("c", 64)
	artifact := model.Artifact{Title: "rebalance test " + uuid.NewString(), SourceURL: "https://example.invalid/rebalance"}
	if err := db.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	file = model.FileObject{ArtifactID: artifact.ID, OriginalFilename: "rebalance.bin", SizeBytes: 100, Sha256: &digest, PreservationStatus: "verified", RedistributionPolicy: "allowed", PublicDownloadEnabled: true}
	if err := db.Create(&file).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	seen := now
	since := now.Add(-10 * time.Minute)
	makeNode := func(name string, used int64) model.Node {
		node := model.Node{Name: name + "-" + file.ID[:8], NodeType: "B", Status: "online", CapacityBytes: 1000, UsedBytes: used, AvailableBytes: 1000 - used, RequestedCapacityBytes: 1000, LastSeenAt: &seen, OnlineSince: &since}
		if err := db.Create(&node).Error; err != nil {
			t.Fatal(err)
		}
		return node
	}
	hot = makeNode("rebalance-hot", 900)
	cold = makeNode("rebalance-cold", 100)
	spare = makeNode("rebalance-spare", 50)

	verified := now.Add(-10 * time.Minute)
	for _, node := range []model.Node{hot, cold} {
		if err := db.Create(&model.Placement{FileObjectID: file.ID, NodeID: node.ID, StorageKey: "objects/" + file.ID + "/" + node.Name, Status: "verified", VerifiedAt: &verified}).Error; err != nil {
			t.Fatal(err)
		}
	}
	s = &Server{db: db}
	return s, file, hot, cold, spare
}

// TestReconcileEndpointIsAsynchronous verifies that the admin endpoint returns
// 202 immediately and exposes the run through the status endpoint.
func TestReconcileEndpointIsAsynchronous(t *testing.T) {
	db := retryTestDB(t)
	s := &Server{db: db}
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/placements/reconcile", nil)
	s.reconcileNow(c)
	if response.Code != http.StatusAccepted {
		t.Fatalf("reconcile POST = %d %s, want 202", response.Code, response.Body)
	}
	if !strings.Contains(response.Body.String(), `"status":"started"`) {
		t.Fatalf("body = %s, want status started", response.Body)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		snapshot := s.reconcileSnapshot("")
		status := snapshot["status"]
		if status == "completed" || status == "failed" {
			if status != "completed" {
				t.Fatalf("reconcile finished with %v (error: %v)", status, snapshot["error"])
			}
			counts, ok := snapshot["counts"].(placementReconcileStats)
			if !ok {
				t.Fatalf("counts = %T", snapshot["counts"])
			}
			if counts.FilesScanned != 0 {
				t.Fatalf("files scanned = %d, want 0 for an empty database", counts.FilesScanned)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("reconcile did not finish in time")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestClaimDoesNotRestoreRetriedRebalanceDelete walks the real sequence:
// first claim, lost lease, admin retry, another copy stopping, re-claim. The
// retry resets attempt_count but the copy may already be gone, so it must not
// be counted as verified again.
func TestClaimDoesNotRestoreRetriedRebalanceDelete(t *testing.T) {
	db := retryTestDB(t)
	s, file, hot, cold, spare := rebalanceFixture(t, db)
	now := time.Now().UTC()
	sha := strings.Repeat("d", 64)
	retryExec(t, db, `INSERT INTO placements (id, file_object_id, node_id, storage_key, sha256, status, verified_at, created_at)
		VALUES (?, ?, ?, ?, ?, 'verified', ?, ?)`, uuid.NewString(), file.ID, spare.ID, "objects/"+file.ID+"/spare", sha, now.Add(-10*time.Minute), now.Add(-10*time.Minute))
	retryExec(t, db, "UPDATE placements SET status = 'deleting', retire_requested_at = ? WHERE file_object_id = ? AND node_id = ?", now.Add(-10*time.Minute), file.ID, hot.ID)
	task := model.Task{TaskType: "delete", FileObjectID: file.ID, TargetNodeID: &hot.ID, Status: "queued"}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create delete task: %v", err)
	}

	// First claim records the claim history while two stable copies remain.
	if response := replicaClaim(s, hot); response.Code != http.StatusOK {
		t.Fatalf("initial deletion claim = %d %s, want 200", response.Code, response.Body)
	}
	var claimed model.Task
	if err := db.First(&claimed, "id = ?", task.ID).Error; err != nil {
		t.Fatalf("read claimed task: %v", err)
	}
	if claimed.FirstClaimedAt == nil {
		t.Fatal("first claim must record first_claimed_at")
	}
	// The node ran the delete and its result was lost; the task is failed.
	retryExec(t, db, "UPDATE tasks SET status = 'failed', attempt_count = 5, lease_until = NULL, lease_token = NULL WHERE id = ?", task.ID)

	actor := model.User{ID: uuid.NewString(), Email: "admin@example.invalid"}
	if err := s.retryFailedTask(context.Background(), task.ID, actor); err != nil {
		t.Fatalf("retry failed deletion: %v", err)
	}
	var retried model.Task
	if err := db.First(&retried, "id = ?", task.ID).Error; err != nil {
		t.Fatalf("read retried task: %v", err)
	}
	if retried.Status != "queued" || retried.AttemptCount != 0 {
		t.Fatalf("retried task = %+v, want queued with attempt_count 0", retried)
	}
	if retried.FirstClaimedAt == nil {
		t.Fatal("retry must keep the claim history")
	}

	// Both other copies stop before the task is claimed again.
	retryExec(t, db, "UPDATE nodes SET status = 'offline' WHERE id IN ?", []string{cold.ID, spare.ID})
	response := replicaClaim(s, hot)
	if response.Code != http.StatusNoContent {
		t.Fatalf("unsafe retried claim = %d %s, want 204", response.Code, response.Body)
	}
	var restored struct {
		Status string
		Marker *time.Time
	}
	if err := db.Raw("SELECT status, retire_requested_at AS marker FROM placements WHERE file_object_id = ? AND node_id = ?", file.ID, hot.ID).Scan(&restored).Error; err != nil {
		t.Fatalf("read placement: %v", err)
	}
	if restored.Status != "error" || restored.Marker != nil {
		t.Fatalf("placement = %+v, want error with no marker", restored)
	}
}
