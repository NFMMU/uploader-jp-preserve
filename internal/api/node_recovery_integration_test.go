package api

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestReconcileRepairsOfflineReplicaIntegration verifies the scheduler's state
// transitions against an isolated test database. It simulates the agent's
// successful transfer; the real data transfer is covered by the manual test.
func TestReconcileRepairsOfflineReplicaIntegration(t *testing.T) {
	if os.Getenv("RUN_SURPLUS_DB_INTEGRATION") != "1" {
		t.Skip("set RUN_SURPLUS_DB_INTEGRATION=1 via scripts/test-surplus-cleanup.sh")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	parsed, err := url.Parse(dsn)
	if err != nil || parsed == nil || parsed.Path != "/archive_test" || parsed.Port() != "55432" || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1") {
		t.Fatalf("refusing integration test database; expected localhost:55432 database /archive_test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test database handle: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("ping test database: %v", err)
	}

	fileID, nodeAID, nodeBID, nodeCID := createNodeRecoveryFixture(t, db)
	s := &Server{db: db}
	if err := s.reconcilePlacements(context.Background()); err != nil {
		t.Fatalf("reconcile after node outage: %v", err)
	}

	var nodeBStatus string
	if err := db.Raw("SELECT status FROM nodes WHERE id = ?", nodeBID).Scan(&nodeBStatus).Error; err != nil {
		t.Fatalf("read offline node status: %v", err)
	}
	if nodeBStatus != "offline" {
		t.Fatalf("stale node status = %q, want offline", nodeBStatus)
	}

	var task struct {
		TaskType     string
		Status       string
		TargetNodeID string
	}
	if err := db.Raw("SELECT task_type, status, target_node_id FROM tasks WHERE file_object_id = ? AND status IN ('queued','running') ORDER BY created_at DESC LIMIT 1", fileID).Scan(&task).Error; err != nil {
		t.Fatalf("read replacement task: %v", err)
	}
	if task.TaskType != "replicate" || task.Status != "queued" || task.TargetNodeID != nodeCID {
		t.Fatalf("replacement task = %+v, want queued replicate targeting node C (%s)", task, nodeCID)
	}

	var cPlacementStatus string
	if err := db.Raw("SELECT status FROM placements WHERE file_object_id = ? AND node_id = ?", fileID, nodeCID).Scan(&cPlacementStatus).Error; err != nil {
		t.Fatalf("read replacement placement: %v", err)
	}
	if cPlacementStatus != "queued" {
		t.Fatalf("replacement placement status = %q, want queued", cPlacementStatus)
	}

	// Simulate successful transfer, hash verification, and the original node's return.
	now := time.Now().UTC()
	if err := db.Exec("UPDATE tasks SET status = 'completed', updated_at = ? WHERE file_object_id = ? AND task_type = 'replicate' AND status = 'queued'", now, fileID).Error; err != nil {
		t.Fatalf("complete simulated replication task: %v", err)
	}
	if err := db.Exec("UPDATE placements SET status = 'verified', verified_at = ? WHERE file_object_id = ? AND node_id = ?", now, fileID, nodeCID).Error; err != nil {
		t.Fatalf("verify replacement placement: %v", err)
	}
	if err := db.Exec("UPDATE nodes SET status = 'online', last_seen_at = ?, online_since = ?, updated_at = ? WHERE id = ?", now, now, now, nodeBID).Error; err != nil {
		t.Fatalf("simulate original node returning: %v", err)
	}
	if err := s.reconcilePlacements(context.Background()); err != nil {
		t.Fatalf("reconcile after node recovery: %v", err)
	}

	var healthyCount int64
	if err := db.Table("placements AS p").Joins("JOIN nodes AS n ON n.id = p.node_id").Where("p.file_object_id = ? AND p.status = 'verified' AND n.status = 'online' AND n.last_seen_at >= now() - interval '2 minutes'", fileID).Count(&healthyCount).Error; err != nil {
		t.Fatalf("count healthy placements after recovery: %v", err)
	}
	if healthyCount != 3 {
		t.Fatalf("healthy placement count after recovery = %d, want 3 (nodes A=%s, B=%s, C=%s)", healthyCount, nodeAID, nodeBID, nodeCID)
	}
	var deleteCount int64
	if err := db.Table("tasks").Where("file_object_id = ? AND task_type = 'delete'", fileID).Count(&deleteCount).Error; err != nil {
		t.Fatalf("count delete tasks after recovery: %v", err)
	}
	if deleteCount != 0 {
		t.Fatalf("delete task count after recovery = %d, want 0", deleteCount)
	}
	var preservationStatus string
	if err := db.Raw("SELECT preservation_status FROM file_objects WHERE id = ?", fileID).Scan(&preservationStatus).Error; err != nil {
		t.Fatalf("read preservation status after recovery: %v", err)
	}
	if preservationStatus != "verified" {
		t.Fatalf("preservation status after recovery = %q, want verified", preservationStatus)
	}
}

func createNodeRecoveryFixture(t *testing.T, db *gorm.DB) (fileID, nodeAID, nodeBID, nodeCID string) {
	t.Helper()
	now := time.Now().UTC()
	artifactID := uuid.NewString()
	fileID = uuid.NewString()
	nodeAID, nodeBID, nodeCID = uuid.NewString(), uuid.NewString(), uuid.NewString()
	sha := strings.Repeat("b", 64)

	t.Cleanup(func() {
		if err := db.Exec("DELETE FROM audit_logs WHERE target_id = ?", fileID).Error; err != nil {
			t.Errorf("cleanup audit logs: %v", err)
		}
		if err := db.Exec("DELETE FROM tasks WHERE file_object_id = ?", fileID).Error; err != nil {
			t.Errorf("cleanup tasks: %v", err)
		}
		if err := db.Exec("DELETE FROM placements WHERE file_object_id = ?", fileID).Error; err != nil {
			t.Errorf("cleanup placements: %v", err)
		}
		if err := db.Exec("DELETE FROM file_objects WHERE id = ?", fileID).Error; err != nil {
			t.Errorf("cleanup file object: %v", err)
		}
		if err := db.Exec("DELETE FROM artifacts WHERE id = ?", artifactID).Error; err != nil {
			t.Errorf("cleanup artifact: %v", err)
		}
		if err := db.Exec("DELETE FROM nodes WHERE id IN ?", []string{nodeAID, nodeBID, nodeCID}).Error; err != nil {
			t.Errorf("cleanup nodes: %v", err)
		}
	})

	if err := db.Exec("INSERT INTO artifacts (id, title, source_url) VALUES (?, ?, ?)", artifactID, "node recovery integration test", "https://example.invalid/"+artifactID).Error; err != nil {
		t.Fatalf("insert artifact: %v", err)
	}
	if err := db.Exec("INSERT INTO file_objects (id, artifact_id, original_filename, size_bytes, sha256, preservation_status, redistribution_policy, public_download_enabled) VALUES (?, ?, ?, ?, ?, 'verified', 'unknown', false)", fileID, artifactID, "node-recovery-test.bin", 32, sha).Error; err != nil {
		t.Fatalf("insert file object: %v", err)
	}

	for _, n := range []struct {
		id       string
		name     string
		typ      string
		lastSeen time.Time
	}{
		{id: nodeAID, name: "recovery-test-a-" + fileID[:8], typ: "A", lastSeen: now.Add(-10 * time.Second)},
		{id: nodeBID, name: "recovery-test-b-" + fileID[:8], typ: "B", lastSeen: now.Add(-3 * time.Minute)},
		{id: nodeCID, name: "recovery-test-c-" + fileID[:8], typ: "B", lastSeen: now.Add(-10 * time.Second)},
	} {
		onlineSince := now.Add(-15 * time.Minute)
		if err := db.Exec(`INSERT INTO nodes (id, name, node_type, status, capacity_bytes, used_bytes, available_bytes, requested_capacity_bytes, last_seen_at, created_at, updated_at, online_since)
			VALUES (?, ?, ?, 'online', 1073741824, 0, 1073741824, 1073741824, ?, ?, ?, ?)`, n.id, n.name, n.typ, n.lastSeen, now.Add(-20*time.Minute), now, onlineSince).Error; err != nil {
			t.Fatalf("insert node %s: %v", n.name, err)
		}
	}

	for _, n := range []struct{ id, name string }{{nodeAID, "a"}, {nodeBID, "b"}} {
		if err := db.Exec(`INSERT INTO placements (id, file_object_id, node_id, storage_key, sha256, status, verified_at, created_at)
			VALUES (?, ?, ?, ?, ?, 'verified', ?, ?)`, uuid.NewString(), fileID, n.id, "objects/"+fileID+"/"+n.name, sha, now.Add(-10*time.Minute), now.Add(-20*time.Minute)).Error; err != nil {
			t.Fatalf("insert placement on node %s: %v", n.id, err)
		}
	}
	return fileID, nodeAID, nodeBID, nodeCID
}
