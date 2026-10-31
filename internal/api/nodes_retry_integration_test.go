package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nfmmu/uploader-archive/internal/config"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func retryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if os.Getenv("RUN_SURPLUS_DB_INTEGRATION") != "1" {
		t.Skip("set RUN_SURPLUS_DB_INTEGRATION=1 and TEST_DATABASE_URL to a disposable local archive_test database")
	}
	dsn, err := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	if err != nil || dsn == nil || dsn.Path != "/archive_test" || dsn.Port() != "55432" || (dsn.Hostname() != "localhost" && dsn.Hostname() != "127.0.0.1") {
		t.Fatal("refusing integration test database; expected localhost:55432 database /archive_test")
	}
	db, err := gorm.Open(postgres.Open(dsn.String()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	// Isolate node selection and concurrent connections using the migrated schema.
	schema := "retry_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	retryExec(t, db, "CREATE SCHEMA "+schema)
	t.Cleanup(func() { retryExec(t, db, "DROP SCHEMA "+schema+" CASCADE") })
	for _, table := range []string{"artifacts", "file_objects", "nodes", "placements", "tasks", "audit_logs"} {
		retryExec(t, db, "CREATE TABLE "+schema+"."+table+" (LIKE public."+table+" INCLUDING ALL)")
	}
	query := dsn.Query()
	query.Set("search_path", schema)
	query.Set("application_name", schema)
	dsn.RawQuery = query.Encode()
	isolated, err := gorm.Open(postgres.Open(dsn.String()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	isolatedSQL, err := isolated.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = isolatedSQL.Close() })
	return isolated
}

func retryExec(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatal(err)
	}
}

func retryFixture(t *testing.T, db *gorm.DB) (*Server, model.MFileObject, model.MNode, model.TTask) {
	t.Helper()
	content := []byte("synthetic retry content")
	digest := fmt.Sprintf("%x", sha256.Sum256(content))
	artifact := model.MArtifact{Title: "retry test", SourceURL: "https://example.invalid/retry"}
	if err := db.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	file := model.MFileObject{ArtifactID: artifact.ID, OriginalFilename: "retry.bin", SizeBytes: int64(len(content)), Sha256: &digest, PreservationStatus: "error", RedistributionPolicy: "allowed", PublicDownloadEnabled: true}
	node := model.MNode{Name: "original", NodeType: "A", Status: "online"}
	for _, record := range []any{&file, &node} {
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	lastError := "previous failure"
	task := model.TTask{TaskType: "store", FileObjectID: file.ID, TargetNodeID: &node.ID, Status: "failed", AttemptCount: 5, LastError: &lastError}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	retryExec(t, db, "INSERT INTO placements (file_object_id, node_id, storage_key, status) VALUES (?, ?, ?, 'error')", file.ID, node.ID, "objects/"+file.ID+"/"+task.ID)
	s := &Server{db: db, cfg: config.Config{StagingDir: t.TempDir()}, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := os.WriteFile(filepath.Join(s.cfg.StagingDir, task.ID), content, 0o600); err != nil {
		t.Fatal(err)
	}
	return s, file, node, task
}

func retryRequest(s *Server, taskID string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/tasks/"+taskID+"/retry", nil).WithContext(ctx)
	c.Params = gin.Params{{Key: "id", Value: taskID}}
	c.Set("currentUser", model.MUser{ID: uuid.NewString(), Email: "admin@example.invalid"})
	s.retryTask(c)
	return response
}

func TestRetryStoreTaskIntegration(t *testing.T) {
	for _, alternativeStatus := range []string{"absent", "missing", "error", "verified", "offline"} {
		t.Run(alternativeStatus, func(t *testing.T) {
			db := retryTestDB(t)
			s, file, original, task := retryFixture(t, db)
			wantNode := original.ID
			wantOriginalStatus := "queued"
			var alternative model.MNode
			if alternativeStatus != "absent" {
				alternative = model.MNode{Name: "alternative", NodeType: "B", Status: "online"}
				if alternativeStatus == "offline" {
					alternative.Status = "offline"
				}
				if err := db.Create(&alternative).Error; err != nil {
					t.Fatal(err)
				}
				if alternativeStatus != "missing" {
					retryExec(t, db, "INSERT INTO placements (file_object_id, node_id, storage_key, status) VALUES (?, ?, 'alternative', ?)", file.ID, alternative.ID, alternativeStatus)
				}
				if alternativeStatus == "error" || alternativeStatus == "missing" {
					wantNode, wantOriginalStatus = alternative.ID, "error"
				}
			}
			response := retryRequest(s, task.ID)
			if response.Code != http.StatusAccepted {
				t.Fatalf("retry status = %d: %s", response.Code, response.Body)
			}
			var retried model.TTask
			if err := db.First(&retried, "id = ?", task.ID).Error; err != nil {
				t.Fatal(err)
			}
			if retried.Status != "queued" || retried.TargetNodeID == nil || *retried.TargetNodeID != wantNode || retried.AttemptCount != 0 || retried.LastError != nil {
				t.Fatalf("unexpected retried task: %+v; want target %s", retried, wantNode)
			}
			var placements []model.TPlacement
			if err := db.Find(&placements).Error; err != nil {
				t.Fatal(err)
			}
			wantPlacementCount := 2
			if alternativeStatus == "absent" {
				wantPlacementCount = 1
			}
			if len(placements) != wantPlacementCount {
				t.Fatalf("placement count = %d, want %d", len(placements), wantPlacementCount)
			}
			for _, placement := range placements {
				wantStatus := alternativeStatus
				if placement.NodeID == original.ID {
					wantStatus = wantOriginalStatus
				} else if placement.NodeID == wantNode {
					wantStatus = "queued"
				}
				if placement.Status != wantStatus {
					t.Errorf("placement on %s = %s, want %s", placement.NodeID, placement.Status, wantStatus)
				}
			}
			if err := db.First(&file, "id = ?", file.ID).Error; err != nil {
				t.Fatal(err)
			}
			if file.PreservationStatus != "queued" || file.PublicDownloadEnabled {
				t.Fatalf("unexpected file state: %+v", file)
			}
		})
	}

	t.Run("concurrent retries queue only one task", func(t *testing.T) {
		db := retryTestDB(t)
		s, file, _, first := retryFixture(t, db)
		second := first
		second.ID = uuid.NewString()
		if err := db.Create(&second).Error; err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(filepath.Join(s.cfg.StagingDir, first.ID))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(s.cfg.StagingDir, second.ID), content, 0o600); err != nil {
			t.Fatal(err)
		}
		alternative := model.MNode{Name: "alternative", NodeType: "B", Status: "online"}
		if err := db.Create(&alternative).Error; err != nil {
			t.Fatal(err)
		}
		retryExec(t, db, "INSERT INTO placements (file_object_id, node_id, storage_key, status) VALUES (?, ?, 'alternative', 'error')", file.ID, alternative.ID)

		// Hold the file until both requests are blocked. Without serialization,
		// both requests pass the active-task check before this lock is released.
		blocker := db.Begin()
		if blocker.Error != nil {
			t.Fatal(blocker.Error)
		}
		defer blocker.Rollback()
		retryExec(t, blocker, "SELECT id FROM m_file_objects WHERE id = ? FOR UPDATE", file.ID)
		responses := make(chan *httptest.ResponseRecorder, 2)
		var requests sync.WaitGroup
		requests.Add(2)
		defer func() {
			_ = blocker.Rollback().Error
			requests.Wait()
		}()
		for _, taskID := range []string{first.ID, second.ID} {
			go func() {
				defer requests.Done()
				responses <- retryRequest(s, taskID)
			}()
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			var waiting int64
			if err := db.Raw("SELECT count(*) FROM pg_stat_activity WHERE application_name = current_setting('application_name') AND state = 'active' AND wait_event_type = 'Lock'").Scan(&waiting).Error; err != nil {
				t.Fatal(err)
			}
			if waiting == 2 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("waiting retry requests = %d, want 2", waiting)
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err := blocker.Commit().Error; err != nil {
			t.Fatal(err)
		}
		statuses := map[int]int{}
		for range 2 {
			response := <-responses
			statuses[response.Code]++
		}
		if statuses[http.StatusAccepted] != 1 || statuses[http.StatusConflict] != 1 {
			t.Fatalf("retry response statuses = %v, want one accepted and one conflict", statuses)
		}
		var active int64
		if err := db.Model(&model.TTask{}).Where("file_object_id = ? AND status IN ('queued', 'running')", file.ID).Count(&active).Error; err != nil {
			t.Fatal(err)
		}
		if active != 1 {
			t.Fatalf("active tasks = %d, want 1", active)
		}
	})
}

func TestRetryErrorHandlingIntegration(t *testing.T) {
	const internalDetail = "synthetic database failure with private details"

	t.Run("single retry", func(t *testing.T) {
		for _, scenario := range []struct {
			name   string
			status int
			reason string
		}{
			{"missing task", http.StatusNotFound, "task or related file not found"},
			{"missing file", http.StatusNotFound, "task or related file not found"},
			{"rejected", http.StatusConflict, "file object is already verified"},
			{"internal error", http.StatusInternalServerError, "could not retry task"},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				db := retryTestDB(t)
				s, file, _, task := retryFixture(t, db)
				var logs bytes.Buffer
				s.logger = slog.New(slog.NewTextHandler(&logs, nil))
				switch scenario.name {
				case "missing task":
					task.ID = uuid.NewString()
				case "missing file":
					retryExec(t, db, "DELETE FROM m_file_objects WHERE id = ?", file.ID)
				case "rejected":
					retryExec(t, db, "UPDATE file_objects SET preservation_status = 'verified' WHERE id = ?", file.ID)
				case "internal error":
					prepareSourceFetchRetry(t, db, file, task)
					injectRetryAuditError(t, db, task.ID, internalDetail)
				}
				response := retryRequest(s, task.ID)
				var body struct {
					Error string `json:"error"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if response.Code != scenario.status || body.Error != scenario.reason {
					t.Fatalf("response = %d %q, want %d %q", response.Code, body.Error, scenario.status, scenario.reason)
				}
				if scenario.name == "internal error" && (!strings.Contains(logs.String(), internalDetail) || strings.Contains(response.Body.String(), internalDetail)) {
					t.Fatalf("internal failure must be logged without leaking into response: log=%q body=%q", logs.String(), response.Body.String())
				}
			})
		}
	})

	t.Run("bulk retry", func(t *testing.T) {
		db := retryTestDB(t)
		s, file, _, queued := retryFixture(t, db)
		prepareSourceFetchRetry(t, db, file, queued)
		_, rejectedFile, _, rejected := retryFixture(t, db)
		retryExec(t, db, "UPDATE file_objects SET preservation_status = 'verified' WHERE id = ?", rejectedFile.ID)
		_, failedFile, _, failed := retryFixture(t, db)
		prepareSourceFetchRetry(t, db, failedFile, failed)
		injectRetryAuditError(t, db, failed.ID, internalDetail)
		_, missingFile, _, missing := retryFixture(t, db)
		retryExec(t, db, "DELETE FROM m_file_objects WHERE id = ?", missingFile.ID)

		var logs bytes.Buffer
		s.logger = slog.New(slog.NewTextHandler(&logs, nil))
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		c.Request = httptest.NewRequest(http.MethodPost, "/tasks/retry-all", nil)
		c.Set("currentUser", model.MUser{ID: uuid.NewString(), Email: "admin@example.invalid"})
		s.retryAll(c)
		var body struct {
			Total     int `json:"total"`
			Succeeded int `json:"succeeded"`
			Failed    int `json:"failed"`
			Skipped   int `json:"skipped"`
			Results   []struct {
				TaskID string `json:"task_id"`
				Status string `json:"status"`
				Reason string `json:"reason"`
			} `json:"results"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusAccepted || body.Total != 4 || body.Succeeded != 1 || body.Skipped != 1 || body.Failed != 2 || len(body.Results) != 4 {
			t.Fatalf("unexpected bulk response: %d %s", response.Code, response.Body.String())
		}
		want := map[string][2]string{
			queued.ID:   {"queued", ""},
			rejected.ID: {"skipped", "file object is already verified"},
			failed.ID:   {"failed", "could not retry task"},
			missing.ID:  {"failed", "could not retry task"},
		}
		for _, result := range body.Results {
			if expected, ok := want[result.TaskID]; !ok || result.Status != expected[0] || result.Reason != expected[1] {
				t.Errorf("unexpected bulk result: %+v", result)
			} else {
				delete(want, result.TaskID)
			}
		}
		if len(want) != 0 || !strings.Contains(logs.String(), internalDetail) || strings.Contains(response.Body.String(), internalDetail) {
			t.Fatalf("missing results or internal error not safely logged: missing=%v log=%q body=%q", want, logs.String(), response.Body.String())
		}
		var rolledBack model.TTask
		if err := db.First(&rolledBack, "id = ?", failed.ID).Error; err != nil || rolledBack.Status != "failed" {
			t.Fatalf("failed retry must roll back: task=%+v err=%v", rolledBack, err)
		}
	})
}

func prepareSourceFetchRetry(t *testing.T, db *gorm.DB, file model.MFileObject, task model.TTask) {
	t.Helper()
	retryExec(t, db, "UPDATE file_objects SET source_url = ? WHERE id = ?", "https://example.invalid/file", file.ID)
	retryExec(t, db, "UPDATE tasks SET task_type = 'source_fetch' WHERE id = ?", task.ID)
}

func injectRetryAuditError(t *testing.T, db *gorm.DB, taskID, detail string) {
	t.Helper()
	if err := db.Callback().Create().Before("gorm:create").Register("retry_test_audit_error", func(tx *gorm.DB) {
		if audit, ok := tx.Statement.Dest.(*model.TAuditLog); ok && audit.TargetID != nil && *audit.TargetID == taskID {
			tx.AddError(errors.New(detail))
		}
	}); err != nil {
		t.Fatal(err)
	}
}
