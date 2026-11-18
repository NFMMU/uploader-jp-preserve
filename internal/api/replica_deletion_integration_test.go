package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	c.Request = httptest.NewRequest(http.MethodPost, "/claim", nil)
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
