package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
)

func replicaClaim(s *Server, node model.Node) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/claim", nil).WithContext(ctx)
	c.Set("currentNode", nodeContext{Node: node})
	s.claimNodeTask(c)
	c.Writer.WriteHeaderNow()
	return w
}

func TestClaimSerializesWithFileDeletionIntegration(t *testing.T) {
	db := retryTestDB(t)
	s, file, node, task := retryFixture(t, db)
	retryExec(t, db, "UPDATE tasks SET task_type = 'replicate', source_node_id = ?, status = 'queued', attempt_count = 0 WHERE id = ?", node.ID, task.ID)
	// A deletion transaction owns the file before it examines/cancels tasks.
	tx := db.Begin()
	defer tx.Rollback()
	var locked model.FileObject
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ?", file.ID).Error; err != nil {
		t.Fatal(err)
	}
	if w := replicaClaim(s, node); w.Code != http.StatusNoContent {
		t.Fatalf("claim during file lock: %d %s", w.Code, w.Body)
	}
	if err := queueFileDeletionTx(tx, file, nil, uuid.NewString(), "test deletion"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	// Pause physical deletion to isolate claims of preservation tasks.
	retryExec(t, db, "UPDATE tasks SET status = 'failed' WHERE task_type = 'delete'")
	// Even a queued task inserted after cancellation cannot escape the deletion guard.
	for _, kind := range []string{"replicate", "store", "source_fetch"} {
		retryExec(t, db, "UPDATE tasks SET task_type = ?, status = 'queued' WHERE id = ?", kind, task.ID)
		if w := replicaClaim(s, node); w.Code != http.StatusNoContent {
			t.Fatalf("%s claim after deletion: %d %s", kind, w.Code, w.Body)
		}
	}
	retryExec(t, db, "UPDATE tasks SET status = 'failed' WHERE id = ?", task.ID)
	// Physical deletion tasks must remain claimable.
	retryExec(t, db, "UPDATE tasks SET status = 'queued' WHERE task_type = 'delete'")
	if w := replicaClaim(s, node); w.Code != http.StatusOK {
		t.Fatalf("delete claim: %d %s", w.Code, w.Body)
	}
}

func TestClaimBlocksDeletionWhileReplicaRunsIntegration(t *testing.T) {
	db := retryTestDB(t)
	s, file, node, task := retryFixture(t, db)
	retryExec(t, db, "UPDATE tasks SET task_type = 'replicate', source_node_id = ?, status = 'queued', attempt_count = 0 WHERE id = ?", node.ID, task.ID)
	if w := replicaClaim(s, node); w.Code != http.StatusOK {
		t.Fatalf("claim: %d %s", w.Code, w.Body)
	}
	err := db.Transaction(func(tx *gorm.DB) error { return queueFileDeletionTx(tx, file, nil, uuid.NewString(), "test deletion") })
	if err == nil {
		t.Fatal("deletion accepted while replica is running")
	}
}

type replicaDeletionReader struct {
	io.Reader
	beforeRead func()
}

func (r *replicaDeletionReader) Read(p []byte) (int, error) {
	if r.beforeRead != nil {
		f := r.beforeRead
		r.beforeRead = nil
		f()
	}
	return r.Reader.Read(p)
}

func TestReplicaUploadRechecksDeletionIntegration(t *testing.T) {
	db := retryTestDB(t)
	s, file, node, task := retryFixture(t, db)
	lease := uuid.NewString()
	retryExec(t, db, "UPDATE tasks SET task_type = 'replicate', source_node_id = ?, status = 'running', lease_token = ?, lease_until = ? WHERE id = ?", node.ID, lease, time.Now().Add(time.Minute), task.ID)
	body := &replicaDeletionReader{Reader: bytes.NewBufferString("synthetic retry content"), beforeRead: func() {
		// Simulate lease exhaustion during an upload followed by accepted deletion.
		retryExec(t, db, "UPDATE tasks SET status = 'failed' WHERE id = ?", task.ID)
		if err := db.Transaction(func(tx *gorm.DB) error { return queueFileDeletionTx(tx, file, nil, uuid.NewString(), "test deletion") }); err != nil {
			t.Fatal(err)
		}
		// Keep the lease valid to prove that the file guard itself rejects the upload.
		retryExec(t, db, "UPDATE tasks SET status = 'running' WHERE id = ?", task.ID)
	}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/replica-content?lease_token="+lease, body)
	c.Params = gin.Params{{Key: "taskID", Value: task.ID}}
	c.Set("currentNode", nodeContext{Node: node})
	s.replicaContent(c)
	if w.Code != http.StatusConflict {
		t.Fatalf("upload after deletion: %d %s", w.Code, w.Body)
	}
	var placement model.Placement
	if err := db.First(&placement, "file_object_id = ?", file.ID).Error; err != nil {
		t.Fatal(err)
	}
	if placement.Status != "deleting" {
		t.Fatalf("placement resurrected: %s", placement.Status)
	}
	if _, err := os.Stat(filepath.Join(s.cfg.StagingDir, task.ID)); !os.IsNotExist(err) {
		t.Fatalf("replica staging survived rejection: %v", err)
	}
}

func TestConcurrentDeleteResultsFinalizeFileIntegration(t *testing.T) {
	db := retryTestDB(t)
	s, file, firstNode, firstTask := retryFixture(t, db)
	secondNode := model.Node{Name: "second", NodeType: "B", Status: "online"}
	if err := db.Create(&secondNode).Error; err != nil {
		t.Fatal(err)
	}
	firstLease, secondLease := uuid.NewString(), uuid.NewString()
	retryExec(t, db, "UPDATE file_objects SET deletion_requested_at = now(), public_download_enabled = false WHERE id = ?", file.ID)
	retryExec(t, db, "UPDATE placements SET status = 'deleting' WHERE file_object_id = ?", file.ID)
	retryExec(t, db, "UPDATE tasks SET task_type = 'delete', status = 'running', attempt_count = 1, lease_token = ?, lease_until = now() + interval '5 minutes' WHERE id = ?", firstLease, firstTask.ID)
	secondTask := model.Task{TaskType: "delete", FileObjectID: file.ID, TargetNodeID: &secondNode.ID, Status: "running", AttemptCount: 1}
	if err := db.Create(&secondTask).Error; err != nil {
		t.Fatal(err)
	}
	retryExec(t, db, "UPDATE tasks SET lease_token = ?, lease_until = now() + interval '5 minutes' WHERE id = ?", secondLease, secondTask.ID)
	retryExec(t, db, "INSERT INTO placements (file_object_id, node_id, storage_key, status) VALUES (?, ?, 'second-copy', 'deleting')", file.ID, secondNode.ID)

	blocker := db.Begin()
	if blocker.Error != nil {
		t.Fatal(blocker.Error)
	}
	defer blocker.Rollback()
	var locked model.FileObject
	if err := blocker.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ?", file.ID).Error; err != nil {
		t.Fatal(err)
	}

	type result struct {
		status int
		body   string
	}
	results := make(chan result, 2)
	var requests sync.WaitGroup
	requests.Add(2)
	send := func(node model.Node, taskID, lease string) {
		defer requests.Done()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/result", strings.NewReader(fmt.Sprintf(`{"lease_token":%q,"status":"success"}`, lease)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "taskID", Value: taskID}}
		c.Set("currentNode", nodeContext{Node: node})
		s.nodeTaskResult(c)
		results <- result{status: w.Code, body: w.Body.String()}
	}
	go send(firstNode, firstTask.ID, firstLease)
	go send(secondNode, secondTask.ID, secondLease)

	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting int64
		if err := db.Raw("SELECT count(*) FROM pg_stat_activity WHERE application_name = current_setting('application_name') AND state = 'active' AND wait_event_type = 'Lock'").Scan(&waiting).Error; err != nil {
			t.Fatal(err)
		}
		if waiting >= 2 {
			break
		}
		if time.Now().After(deadline) {
			_ = blocker.Rollback().Error
			requests.Wait()
			t.Fatalf("delete results waiting on file lock = %d, want 2", waiting)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := blocker.Commit().Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		got := <-results
		if got.status != http.StatusOK {
			t.Fatalf("delete result: %d %s", got.status, got.body)
		}
	}
	requests.Wait()
	var completed model.FileObject
	if err := db.Unscoped().First(&completed, "id = ?", file.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !completed.DeletedAt.Valid {
		t.Fatalf("all placements were deleted but file is still pending: %+v", completed)
	}
}
