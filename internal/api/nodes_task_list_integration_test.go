package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
)

func TestListTasksIncludesFailedTasksForVerifiedFilesIntegration(t *testing.T) {
	db := retryTestDB(t)

	artifact := model.Artifact{
		Title:     "task list integration test",
		SourceURL: "https://example.invalid/task-list",
	}
	if err := db.Create(&artifact).Error; err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	file := model.FileObject{
		ArtifactID:          artifact.ID,
		OriginalFilename:    "verified.bin",
		SizeBytes:           1,
		PreservationStatus:  "verified",
		RedistributionPolicy: "unknown",
	}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("create verified file: %v", err)
	}

	for _, taskType := range []string{"source_fetch", "store"} {
		task := model.Task{
			TaskType:     taskType,
			FileObjectID: file.ID,
			Status:       "failed",
			AttemptCount: 5,
		}
		if err := db.Create(&task).Error; err != nil {
			t.Fatalf("create %s task: %v", taskType, err)
		}
	}

	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/tasks?status=failed&page=1&limit=10", nil)

	(&Server{db: db}).listTasks(ctx)

	if response.Code != http.StatusOK {
		t.Fatalf("listTasks status = %d, body = %s", response.Code, response.Body.String())
	}

	var body struct {
		Items []struct {
			TaskType string `json:"task_type"`
			Status   string `json:"status"`
		} `json:"items"`
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode listTasks response: %v", err)
	}
	if body.Total != 2 {
		t.Fatalf("total = %d, want 2; response = %s", body.Total, response.Body.String())
	}
	if len(body.Items) != 2 {
		t.Fatalf("items = %d, want 2; response = %s", len(body.Items), response.Body.String())
	}
	for _, item := range body.Items {
		if item.Status != "failed" {
			t.Errorf("task status = %q, want failed", item.Status)
		}
	}
}
