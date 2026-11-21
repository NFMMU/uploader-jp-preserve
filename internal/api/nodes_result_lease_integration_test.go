package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
)

// TestNodeTaskResultRejectsStaleLeaseAfterFileLockIntegration は、ファイル
// ロック待ちの間にタスクが再取得された場合、古いリースでの結果报告がタスク
// 状態を変えずに 409 を返すことを検証する。
func TestNodeTaskResultRejectsStaleLeaseAfterFileLockIntegration(t *testing.T) {
	db := retryTestDB(t)
	s, file, node, task := retryFixture(t, db)
	oldLease := uuid.NewString()
	retryExec(t, db, "UPDATE tasks SET status = 'running', attempt_count = 4, lease_token = ?, lease_until = now() + interval '5 minutes' WHERE id = ?", oldLease, task.ID)

	// ファイル行ロックを保持して、結果报告がトランザクション内で待機するようにする。
	blocker := db.Begin()
	if blocker.Error != nil {
		t.Fatal(blocker.Error)
	}
	if err := blocker.Exec("SELECT id FROM file_objects WHERE id = ? FOR UPDATE", file.ID).Error; err != nil {
		t.Fatal(err)
	}

	type outcome struct {
		code int
		body string
	}
	done := make(chan outcome, 1)
	go func() {
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		c.Request = httptest.NewRequest(http.MethodPost, "/result", strings.NewReader(fmt.Sprintf(`{"lease_token":%q,"status":"success"}`, oldLease)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "taskID", Value: task.ID}}
		c.Set("currentNode", nodeContext{Node: node})
		s.nodeTaskResult(c)
		done <- outcome{code: response.Code, body: response.Body.String()}
	}()

	// ハンドラがファイルロックの待ちで停車するまで待つ。
	parked := false
	for range 100 {
		var waiting int64
		if err := db.Raw("SELECT count(*) FROM pg_stat_activity WHERE state = 'active' AND wait_event_type = 'Lock' AND query LIKE '%file_objects%FOR UPDATE%'").Scan(&waiting).Error; err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			parked = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !parked {
		_ = blocker.Rollback()
		t.Fatal("handler did not wait on the file lock")
	}

	// claimNodeTask による再取得を模擬する。
	newLease := uuid.NewString()
	retryExec(t, db, "UPDATE tasks SET lease_token = ?, attempt_count = 5, lease_until = now() + interval '5 minutes', last_error = NULL WHERE id = ?", newLease, task.ID)
	if err := blocker.Commit().Error; err != nil {
		t.Fatal(err)
	}

	result := <-done
	if result.code != http.StatusConflict {
		t.Fatalf("result: %d %s; want 409", result.code, result.body)
	}

	var got model.Task
	if err := db.First(&got, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != "running" || got.AttemptCount != 5 || got.LeaseToken == nil || *got.LeaseToken != newLease {
		t.Fatalf("task state changed by stale result: %+v", got)
	}
	var audits int64
	if err := db.Model(&model.AuditLog{}).Where("target_type = 'task' AND target_id = ?", task.ID).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if audits != 0 {
		t.Fatalf("stale result wrote audit logs: %d", audits)
	}
	var placement model.Placement
	if err := db.First(&placement, "file_object_id = ? AND node_id = ?", file.ID, node.ID).Error; err != nil {
		t.Fatal(err)
	}
	if placement.Status != "error" {
		t.Fatalf("placement changed by stale result: %+v", placement)
	}
}
