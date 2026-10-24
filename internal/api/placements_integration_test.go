package api

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// This test uses a dedicated, disposable PostgreSQL database. It deliberately
// refuses non-local hosts and databases other than archive_test.
func TestScheduleSurplusPlacementDeletionIntegration(t *testing.T) {
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

	tests := []struct {
		name             string
		replicaCount     int
		activeTask       bool
		nodeDAge         time.Duration
		nodeDVerifiedAge time.Duration
		wantDelete       bool
	}{
		{name: "four stable replicas queues one delete", replicaCount: 4, nodeDAge: 10 * time.Minute, nodeDVerifiedAge: 10 * time.Minute, wantDelete: true},
		{name: "large surplus still queues only one delete", replicaCount: 5, nodeDAge: 10 * time.Minute, nodeDVerifiedAge: 10 * time.Minute, wantDelete: true},
		{name: "three replicas are retained", replicaCount: 3, nodeDAge: 10 * time.Minute, nodeDVerifiedAge: 10 * time.Minute},
		{name: "active task blocks deletion", replicaCount: 4, activeTask: true, nodeDAge: 10 * time.Minute, nodeDVerifiedAge: 10 * time.Minute},
		{name: "recently online node blocks deletion", replicaCount: 4, nodeDAge: 2 * time.Minute, nodeDVerifiedAge: 10 * time.Minute},
		{name: "recently verified replica blocks deletion", replicaCount: 4, nodeDAge: 10 * time.Minute, nodeDVerifiedAge: 2 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileID, newestNodeID := createSurplusCleanupFixture(t, db, tt.replicaCount, tt.nodeDAge, tt.nodeDVerifiedAge)
			if tt.activeTask {
				if err := db.Exec("INSERT INTO tasks (id, task_type, file_object_id, status, priority, attempt_count) VALUES (?, 'replicate', ?, 'queued', 3, 0)", uuid.NewString(), fileID).Error; err != nil {
					t.Fatalf("insert active task: %v", err)
				}
			}

			s := &Server{db: db}
			if err := s.scheduleSurplusPlacementDeletion(context.Background(), fileID); err != nil {
				t.Fatalf("schedule surplus deletion: %v", err)
			}
			var deleteCount int64
			if err := db.Table("tasks").Where("file_object_id = ? AND task_type = 'delete'", fileID).Count(&deleteCount).Error; err != nil {
				t.Fatalf("count delete tasks: %v", err)
			}
			wantCount := int64(0)
			if tt.wantDelete {
				wantCount = 1
			}
			if deleteCount != wantCount {
				t.Fatalf("delete task count = %d, want %d", deleteCount, wantCount)
			}

			var deletingCount int64
			if err := db.Table("placements").Where("file_object_id = ? AND status = 'deleting'", fileID).Count(&deletingCount).Error; err != nil {
				t.Fatalf("count deleting placements: %v", err)
			}
			if deletingCount != wantCount {
				t.Fatalf("deleting placement count = %d, want %d", deletingCount, wantCount)
			}
			var verifiedCount int64
			if err := db.Table("placements").Where("file_object_id = ? AND status = 'verified'", fileID).Count(&verifiedCount).Error; err != nil {
				t.Fatalf("count verified placements: %v", err)
			}
			wantVerified := int64(tt.replicaCount)
			if tt.wantDelete {
				wantVerified--
			}
			if verifiedCount != wantVerified {
				t.Fatalf("verified placement count = %d, want %d", verifiedCount, wantVerified)
			}

			if tt.wantDelete {
				var targetID string
				if err := db.Raw("SELECT target_node_id FROM tasks WHERE file_object_id = ? AND task_type = 'delete' LIMIT 1", fileID).Scan(&targetID).Error; err != nil {
					t.Fatalf("read delete target: %v", err)
				}
				if targetID != newestNodeID {
					t.Fatalf("delete target = %s, want newest replica node %s", targetID, newestNodeID)
				}
				if err := s.scheduleSurplusPlacementDeletion(context.Background(), fileID); err != nil {
					t.Fatalf("repeat surplus deletion scheduling: %v", err)
				}
				if err := db.Table("tasks").Where("file_object_id = ? AND task_type = 'delete'", fileID).Count(&deleteCount).Error; err != nil {
					t.Fatalf("recount delete tasks: %v", err)
				}
				if deleteCount != 1 {
					t.Fatalf("repeated reconciliation created %d delete tasks, want exactly 1", deleteCount)
				}
			}
		})
	}
}

func createSurplusCleanupFixture(t *testing.T, db *gorm.DB, replicaCount int, nodeDAge, nodeDVerifiedAge time.Duration) (fileID string, newestNodeID string) {
	var nodeIDs []string
	t.Helper()
	now := time.Now().UTC()
	artifactID := uuid.NewString()
	fileID = uuid.NewString()
	sha := strings.Repeat("a", 64)

	t.Cleanup(func() {
		if err := db.Exec("DELETE FROM audit_logs WHERE target_id = ?", fileID).Error; err != nil {
			t.Errorf("cleanup audit logs: %v", err)
		}
		if err := db.Exec("DELETE FROM artifacts WHERE id = ?", artifactID).Error; err != nil {
			t.Errorf("cleanup artifact: %v", err)
		}
		if len(nodeIDs) > 0 {
			if err := db.Exec("DELETE FROM nodes WHERE id IN ?", nodeIDs).Error; err != nil {
				t.Errorf("cleanup nodes: %v", err)
			}
		}
	})

	if err := db.Exec("INSERT INTO artifacts (id, title, source_url) VALUES (?, ?, ?)", artifactID, "surplus cleanup integration test", "https://example.invalid/test").Error; err != nil {
		t.Fatalf("insert artifact: %v", err)
	}
	if err := db.Exec("INSERT INTO file_objects (id, artifact_id, original_filename, size_bytes, sha256, preservation_status, redistribution_policy, public_download_enabled) VALUES (?, ?, ?, ?, ?, 'verified', 'unknown', false)", fileID, artifactID, "test.bin", 32, sha).Error; err != nil {
		t.Fatalf("insert file object: %v", err)
	}

	for i := 0; i < replicaCount; i++ {
		nodeID := uuid.NewString()
		name := fmt.Sprintf("surplus-test-%s-%d", fileID[:8], i)
		onlineAge := 10 * time.Minute
		verifiedAge := time.Duration(10+i) * time.Minute
		if i == replicaCount-1 {
			newestNodeID = nodeID
			if replicaCount >= 4 {
				onlineAge = nodeDAge
				verifiedAge = nodeDVerifiedAge
			}
		}
		lastSeen := now.Add(-10 * time.Second)
		onlineSince := now.Add(-onlineAge)
		createdAt := now.Add(-time.Duration(replicaCount-i) * time.Minute)
		verifiedAt := now.Add(-verifiedAge)
		if err := db.Exec(`INSERT INTO nodes (id, name, node_type, status, capacity_bytes, used_bytes, available_bytes, requested_capacity_bytes, last_seen_at, created_at, updated_at, online_since)
			VALUES (?, ?, ?, 'online', 1073741824, 32, 1073741792, 1073741824, ?, ?, ?, ?)`, nodeID, name, map[bool]string{true: "A", false: "B"}[i == 0], lastSeen, createdAt, now, onlineSince).Error; err != nil {
			t.Fatalf("insert node: %v", err)
		}
		if err := db.Exec(`INSERT INTO placements (id, file_object_id, node_id, storage_key, sha256, status, verified_at, created_at)
			VALUES (?, ?, ?, ?, ?, 'verified', ?, ?)`, uuid.NewString(), fileID, nodeID, "objects/"+fileID+"/"+nodeID, sha, verifiedAt, createdAt).Error; err != nil {
			t.Fatalf("insert placement: %v", err)
		}
		nodeIDs = append(nodeIDs, nodeID)
	}
	return fileID, newestNodeID
}
