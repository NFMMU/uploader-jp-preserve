package api

import (
	"encoding/json"
	"log/slog"
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

func replaceTestServer(t *testing.T) *Server {
	t.Helper()
	db, _ := catalogTestDB(t)
	if err := db.AutoMigrate(&model.User{}, &model.AuditLog{}, &model.AuthorArtifact{}, &model.Artifact{}, &model.FileObject{}, &model.Node{}, &model.Placement{}, &model.Task{}); err != nil {
		t.Fatal(err)
	}
	return &Server{db: db, logger: slog.Default()}
}

func replaceTestUser(t *testing.T, db *gorm.DB, role string) model.User {
	t.Helper()
	id := uuid.NewString()
	user := model.User{ID: id, Email: id + "@example.test", DisplayName: "before", PasswordHash: "unchanged", Role: role}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

func replaceTestArtifact(t *testing.T, db *gorm.DB) model.Artifact {
	t.Helper()
	artifact := model.Artifact{ID: uuid.NewString(), Title: "synthetic", SourceURL: "https://example.test/" + uuid.NewString()}
	if err := db.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	return artifact
}

func replaceTestFile(t *testing.T, db *gorm.DB, artifactID, name string) model.FileObject {
	t.Helper()
	file := model.FileObject{ArtifactID: artifactID, OriginalFilename: name, SizeBytes: 10}
	if err := db.Create(&file).Error; err != nil {
		t.Fatal(err)
	}
	return file
}

func replaceAssign(t *testing.T, db *gorm.DB, userID, artifactID string) {
	t.Helper()
	if err := db.Exec("INSERT INTO author_artifact (user_id,artifact_id) VALUES (?,?)", userID, artifactID).Error; err != nil {
		t.Fatal(err)
	}
}

func replaceRequest(t *testing.T, s *Server, urlPath, body string, actor *model.User) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	withActor := func(c *gin.Context) {
		if actor != nil {
			c.Set("currentUser", *actor)
		}
	}
	router.POST("/catalog/files/:id/replace", withActor, s.requireAuthorFileArtifactAccess(), s.replaceFileObject)
	router.POST("/catalog/artifacts/:id/files", withActor, s.requireAuthorArtifactAccess(), s.createFileObject)
	response := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, urlPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, req)
	return response
}

func TestReplaceFileObjectAsAuthorIntegration(t *testing.T) {
	s := replaceTestServer(t)
	author := replaceTestUser(t, s.db, "author")
	artifact := replaceTestArtifact(t, s.db)
	replaceAssign(t, s.db, author.ID, artifact.ID)
	oldFile := replaceTestFile(t, s.db, artifact.ID, "old.bin")
	if err := s.db.Model(&model.FileObject{}).Where("id = ?", oldFile.ID).Update("public_download_enabled", true).Error; err != nil {
		t.Fatal(err)
	}

	response := replaceRequest(t, s, "/catalog/files/"+oldFile.ID+"/replace",
		`{"original_filename":"new.bin","size_bytes":20,"media_type":"application/octet-stream","description":"replacement"}`, &author)
	if response.Code != http.StatusCreated {
		t.Fatalf("replace status = %d, body %s", response.Code, response.Body.String())
	}
	var created model.FileObject
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == oldFile.ID {
		t.Fatal("replacement must be a new file object")
	}
	if created.ArtifactID != artifact.ID {
		t.Fatalf("replacement artifact = %s, want %s", created.ArtifactID, artifact.ID)
	}
	if created.PreservationStatus != "uncollected" {
		t.Fatalf("replacement status = %s, want uncollected", created.PreservationStatus)
	}
	if created.PublicDownloadEnabled {
		t.Fatal("replacement must start unpublished")
	}

	var reloaded model.FileObject
	if err := s.db.First(&reloaded, "id = ?", oldFile.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.ReplacedByFileObjectID == nil || *reloaded.ReplacedByFileObjectID != created.ID {
		t.Fatalf("old file replaced_by = %v, want %s", reloaded.ReplacedByFileObjectID, created.ID)
	}
	if reloaded.ReplacedByUserID == nil || *reloaded.ReplacedByUserID != author.ID {
		t.Fatalf("old file replaced_by_user = %v, want %s", reloaded.ReplacedByUserID, author.ID)
	}
	if reloaded.PublicDownloadEnabled {
		t.Fatal("old file should be unpublished after replacement")
	}

	var auditCount int64
	s.db.Model(&model.AuditLog{}).Where("action = ? AND target_id = ?", "file_object.replace", oldFile.ID).Count(&auditCount)
	if auditCount != 1 {
		t.Fatalf("audit logs for file_object.replace = %d, want 1", auditCount)
	}
}

func TestReplaceFileObjectAuthorOtherArtifactIntegration(t *testing.T) {
	s := replaceTestServer(t)
	author := replaceTestUser(t, s.db, "author")
	artifact := replaceTestArtifact(t, s.db)
	file := replaceTestFile(t, s.db, artifact.ID, "old.bin")

	response := replaceRequest(t, s, "/catalog/files/"+file.ID+"/replace", `{"original_filename":"new.bin"}`, &author)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body %s", response.Code, response.Body.String())
	}
}

func TestReplaceFileObjectAlreadyReplacedIntegration(t *testing.T) {
	s := replaceTestServer(t)
	admin := replaceTestUser(t, s.db, "admin")
	artifact := replaceTestArtifact(t, s.db)
	oldFile := replaceTestFile(t, s.db, artifact.ID, "old.bin")

	response := replaceRequest(t, s, "/catalog/files/"+oldFile.ID+"/replace", `{"original_filename":"new.bin"}`, &admin)
	if response.Code != http.StatusCreated {
		t.Fatalf("first replace status = %d, body %s", response.Code, response.Body.String())
	}
	response = replaceRequest(t, s, "/catalog/files/"+oldFile.ID+"/replace", `{"original_filename":"new2.bin"}`, &admin)
	if response.Code != http.StatusConflict {
		t.Fatalf("second replace status = %d, want 409, body %s", response.Code, response.Body.String())
	}
}

func TestReplaceFileObjectDeletionRequestedIntegration(t *testing.T) {
	s := replaceTestServer(t)
	admin := replaceTestUser(t, s.db, "admin")
	artifact := replaceTestArtifact(t, s.db)
	file := replaceTestFile(t, s.db, artifact.ID, "old.bin")
	now := time.Now().UTC()
	if err := s.db.Model(&model.FileObject{}).Where("id = ?", file.ID).Update("deletion_requested_at", now).Error; err != nil {
		t.Fatal(err)
	}

	response := replaceRequest(t, s, "/catalog/files/"+file.ID+"/replace", `{"original_filename":"new.bin"}`, &admin)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body %s", response.Code, response.Body.String())
	}
}

func TestReplaceFileObjectVerifiedRetiresReplacedIntegration(t *testing.T) {
	s := replaceTestServer(t)
	admin := replaceTestUser(t, s.db, "admin")
	artifact := replaceTestArtifact(t, s.db)
	oldFile := replaceTestFile(t, s.db, artifact.ID, "old.bin")

	response := replaceRequest(t, s, "/catalog/files/"+oldFile.ID+"/replace", `{"original_filename":"new.bin","size_bytes":20}`, &admin)
	if response.Code != http.StatusCreated {
		t.Fatalf("replace status = %d, body %s", response.Code, response.Body.String())
	}
	var created model.FileObject
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	node1 := model.Node{ID: uuid.NewString(), Name: "n1", NodeType: "storage", Status: "online", LastSeenAt: &now, OnlineSince: &now}
	node2 := model.Node{ID: uuid.NewString(), Name: "n2", NodeType: "storage", Status: "online", LastSeenAt: &now, OnlineSince: &now}
	if err := s.db.Create(&node1).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&node2).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&model.Placement{FileObjectID: created.ID, NodeID: node1.ID, StorageKey: "a", Status: "verified", VerifiedAt: &now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&model.Placement{FileObjectID: created.ID, NodeID: node2.ID, StorageKey: "b", Status: "verified", VerifiedAt: &now}).Error; err != nil {
		t.Fatal(err)
	}

	if err := recomputeFilePreservation(s.db, created.ID); err != nil {
		t.Fatal(err)
	}

	var retired model.FileObject
	if err := s.db.Unscoped().First(&retired, "id = ?", oldFile.ID).Error; err != nil {
		t.Fatal(err)
	}
	if retired.DeletionRequestedAt == nil {
		t.Fatal("old file deletion was not requested after replacement verified")
	}
	if !retired.DeletedAt.Valid {
		t.Fatal("old file should be soft-deleted once it has no placements")
	}
}

func TestCreateFileObjectAsAuthorIntegration(t *testing.T) {
	s := replaceTestServer(t)
	author := replaceTestUser(t, s.db, "author")
	artifact := replaceTestArtifact(t, s.db)
	replaceAssign(t, s.db, author.ID, artifact.ID)
	other := replaceTestArtifact(t, s.db)

	response := replaceRequest(t, s, "/catalog/artifacts/"+artifact.ID+"/files",
		`{"original_filename":"new.bin","size_bytes":10}`, &author)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body %s", response.Code, response.Body.String())
	}
	var created model.FileObject
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ArtifactID != artifact.ID || created.PreservationStatus != "uncollected" || created.PublicDownloadEnabled {
		t.Fatal("created file has unexpected defaults")
	}

	response = replaceRequest(t, s, "/catalog/artifacts/"+other.ID+"/files", `{"original_filename":"new.bin"}`, &author)
	if response.Code != http.StatusForbidden {
		t.Fatalf("create on other artifact status = %d, want 403, body %s", response.Code, response.Body.String())
	}
}

func TestReplaceFileObjectRetiresAfterOldTasksFinishIntegration(t *testing.T) {
	s := replaceTestServer(t)
	admin := replaceTestUser(t, s.db, "admin")
	artifact := replaceTestArtifact(t, s.db)
	oldFile := replaceTestFile(t, s.db, artifact.ID, "old.bin")
	activeTask := model.Task{TaskType: "replicate", FileObjectID: oldFile.ID, Status: "queued", AttemptCount: 0}
	if err := s.db.Create(&activeTask).Error; err != nil {
		t.Fatal(err)
	}

	response := replaceRequest(t, s, "/catalog/files/"+oldFile.ID+"/replace", `{"original_filename":"new.bin"}`, &admin)
	if response.Code != http.StatusCreated {
		t.Fatalf("replace status = %d, body %s", response.Code, response.Body.String())
	}
	var created model.FileObject
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	node1 := model.Node{ID: uuid.NewString(), Name: "n1", NodeType: "storage", Status: "online", LastSeenAt: &now, OnlineSince: &now}
	node2 := model.Node{ID: uuid.NewString(), Name: "n2", NodeType: "storage", Status: "online", LastSeenAt: &now, OnlineSince: &now}
	if err := s.db.Create(&node1).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&node2).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&model.Placement{FileObjectID: created.ID, NodeID: node1.ID, StorageKey: "a", Status: "verified", VerifiedAt: &now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&model.Placement{FileObjectID: created.ID, NodeID: node2.ID, StorageKey: "b", Status: "verified", VerifiedAt: &now}).Error; err != nil {
		t.Fatal(err)
	}

	// 旧ファイルのタスクが残っていても、新ファイルの verified 遷移は確定する。
	if err := recomputeFilePreservation(s.db, created.ID); err != nil {
		t.Fatalf("recompute with active old task should not fail: %v", err)
	}
	var newFile model.FileObject
	if err := s.db.First(&newFile, "id = ?", created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if newFile.PreservationStatus != "verified" {
		t.Fatalf("new file status = %s, want verified", newFile.PreservationStatus)
	}
	var oldAfter model.FileObject
	if err := s.db.Unscoped().First(&oldAfter, "id = ?", oldFile.ID).Error; err != nil {
		t.Fatal(err)
	}
	if oldAfter.DeletionRequestedAt != nil {
		t.Fatal("old file should not be retired while its task is active")
	}

	// 旧ファイルのタスクが完了したら、再計算で退役される。
	if err := s.db.Model(&model.Task{}).Where("id = ?", activeTask.ID).Update("status", "completed").Error; err != nil {
		t.Fatal(err)
	}
	if err := recomputeFilePreservation(s.db, oldFile.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Unscoped().First(&oldAfter, "id = ?", oldFile.ID).Error; err != nil {
		t.Fatal(err)
	}
	if oldAfter.DeletionRequestedAt == nil {
		t.Fatal("old file should be retired after its tasks finish")
	}
}
