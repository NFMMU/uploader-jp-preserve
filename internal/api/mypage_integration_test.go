package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
)

func mypageTestServer(t *testing.T) *Server {
	t.Helper()
	db, _ := catalogTestDB(t)
	if err := db.AutoMigrate(&model.User{}, &model.AuditLog{}, &model.AuthorArtifact{}, &model.Artifact{}, &model.FileObject{}, &model.Node{}, &model.Placement{}, &model.Task{}); err != nil {
		t.Fatal(err)
	}
	return &Server{db: db, logger: slog.Default()}
}

func mypageTestUser(t *testing.T, db *gorm.DB, role string) model.User {
	t.Helper()
	id := uuid.NewString()
	user := model.User{ID: id, Email: id + "@example.test", DisplayName: "before", PasswordHash: "unchanged", Role: role}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

func mypageTestArtifact(t *testing.T, db *gorm.DB) model.Artifact {
	t.Helper()
	artifact := model.Artifact{ID: uuid.NewString(), Title: "synthetic", SourceURL: "https://example.test/" + uuid.NewString()}
	if err := db.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	return artifact
}

func mypageAssign(t *testing.T, db *gorm.DB, userID, artifactID string) {
	t.Helper()
	catalogExec(t, db, "INSERT INTO author_artifact (user_id,artifact_id) VALUES (?,?)", userID, artifactID)
}

func TestMypageOwnAccountIsolationIntegration(t *testing.T) {
	s := mypageTestServer(t)
	other := mypageTestUser(t, s.db, "viewer")
	for _, role := range []string{"admin", "reviewer", "viewer", "author"} {
		t.Run(role, func(t *testing.T) {
			actor := mypageTestUser(t, s.db, role)
			name := strings.Repeat("名", 120)
			body := `{"display_name":"` + name + `","id":"` + other.ID + `","role":"admin","disabled":true,"password_hash":"changed"}`
			rec := mypageRequest(t, http.MethodPut, "/", "/", body, &actor, s.updateOwnAccount)
			if rec.Code != http.StatusOK {
				t.Fatalf("%d %s", rec.Code, rec.Body.String())
			}
			var stored model.User
			if err := s.db.First(&stored, "id = ?", actor.ID).Error; err != nil {
				t.Fatal(err)
			}
			if stored.DisplayName != name || stored.Role != role || stored.DisabledAt != nil || stored.PasswordHash != actor.PasswordHash {
				t.Fatalf("unexpected profile update: %+v", stored)
			}
			var response struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.ID != actor.ID || response.DisplayName != name {
				t.Fatalf("response=%+v", response)
			}
		})
	}
	var stored model.User
	if err := s.db.First(&stored, "id = ?", other.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.DisplayName != "before" || stored.Role != "viewer" {
		t.Fatal("another account was modified")
	}
	actor := mypageTestUser(t, s.db, "author")
	rec := mypageRequest(t, http.MethodPut, "/", "/", `{"display_name":"  trimmed name  "}`, &actor, s.updateOwnAccount)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var trimmed model.User
	if err := s.db.First(&trimmed, "id = ?", actor.ID).Error; err != nil {
		t.Fatal(err)
	}
	if trimmed.DisplayName != "trimmed name" {
		t.Fatalf("name=%q", trimmed.DisplayName)
	}
	missing := model.User{ID: uuid.NewString(), Role: "author"}
	mypageError(t, mypageRequest(t, http.MethodPut, "/", "/", `{"display_name":"valid"}`, &missing, s.updateOwnAccount), http.StatusNotFound, "account not found")
}

func TestMypageReplaceAssignmentsIntegration(t *testing.T) {
	s := mypageTestServer(t)
	admin := mypageTestUser(t, s.db, "admin")
	author := mypageTestUser(t, s.db, "author")
	other := mypageTestUser(t, s.db, "author")
	old := mypageTestArtifact(t, s.db)
	next := mypageTestArtifact(t, s.db)
	mypageAssign(t, s.db, author.ID, old.ID)
	mypageAssign(t, s.db, other.ID, old.ID)
	assertIDs := func(userID string, want []string) {
		t.Helper()
		var ids []string
		if err := s.db.Table("author_artifact").Where("user_id = ?", userID).Order("artifact_id").Pluck("artifact_id", &ids).Error; err != nil {
			t.Fatal(err)
		}
		if len(ids) != len(want) || (len(ids) > 0 && !reflect.DeepEqual(ids, want)) {
			t.Fatalf("assignments=%v want %v", ids, want)
		}
	}
	put := func(userID, body string) int {
		t.Helper()
		return mypageRequest(t, http.MethodPut, "/:userID", "/"+userID, body, &admin, s.replaceAuthorArtifactAssignments).Code
	}
	if status := put(author.ID, `{"artifact_ids":["`+next.ID+`","`+uuid.NewString()+`"]}`); status != http.StatusBadRequest {
		t.Fatalf("missing artifact: %d", status)
	}
	assertIDs(author.ID, []string{old.ID})
	if status := put(author.ID, `{"artifact_ids":["`+next.ID+`"]}`); status != http.StatusOK {
		t.Fatalf("replace: %d", status)
	}
	assertIDs(author.ID, []string{next.ID})
	assertIDs(other.ID, []string{old.ID})
	rec := mypageRequest(t, http.MethodGet, "/:userID", "/"+author.ID, "", &admin, s.getAuthorArtifactAssignments)
	var response struct {
		UserID      string   `json:"user_id"`
		ArtifactIDs []string `json:"artifact_ids"`
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.UserID != author.ID || !reflect.DeepEqual(response.ArtifactIDs, []string{next.ID}) {
		t.Fatalf("response=%+v", response)
	}
	if status := put(author.ID, `{"artifact_ids":[]}`); status != http.StatusOK {
		t.Fatalf("clear: %d", status)
	}
	assertIDs(author.ID, nil)
	if status := put(admin.ID, `{"artifact_ids":[]}`); status != http.StatusConflict {
		t.Fatalf("non-author: %d", status)
	}
	if status := put(uuid.NewString(), `{"artifact_ids":[]}`); status != http.StatusNotFound {
		t.Fatalf("missing user: %d", status)
	}
}

func TestMypageArtifactAccessIntegration(t *testing.T) {
	s := mypageTestServer(t)
	author := mypageTestUser(t, s.db, "author")
	other := mypageTestUser(t, s.db, "author")
	assigned := mypageTestArtifact(t, s.db)
	unassigned := mypageTestArtifact(t, s.db)
	mypageAssign(t, s.db, author.ID, assigned.ID)
	mypageAssign(t, s.db, other.ID, unassigned.ID)
	for _, tc := range []struct {
		id     string
		status int
	}{{assigned.ID, http.StatusNoContent}, {unassigned.ID, http.StatusForbidden}, {uuid.NewString(), http.StatusForbidden}} {
		t.Run(tc.id, func(t *testing.T) {
			reached := false
			rec := mypageRequest(t, http.MethodGet, "/:id", "/"+tc.id, "", &author, s.requireAuthorArtifactAccess(), func(c *gin.Context) { reached = true; c.Status(http.StatusNoContent) })
			if rec.Code != tc.status || reached != (tc.status == http.StatusNoContent) {
				t.Fatalf("status=%d reached=%v", rec.Code, reached)
			}
		})
	}
	// Revoking an assignment must take effect on the very next request.
	catalogExec(t, s.db, "DELETE FROM author_artifact WHERE user_id = ?", author.ID)
	rec := mypageRequest(t, http.MethodGet, "/:id", "/"+assigned.ID, "", &author, s.requireAuthorArtifactAccess(), func(c *gin.Context) { t.Error("revoked author reached handler") })
	mypageError(t, rec, http.StatusForbidden, "artifact access denied")
}

func TestMypagePhysicalDeletionQueueIntegration(t *testing.T) {
	for _, tc := range []struct {
		name, role                 string
		assigned, pending, missing bool
		status                     int
	}{
		{name: "assigned author", role: "author", assigned: true, status: http.StatusAccepted},
		{name: "unassigned author", role: "author", status: http.StatusForbidden},
		{name: "admin without assignment", role: "admin", status: http.StatusAccepted},
		{name: "pending author", role: "author", assigned: true, pending: true, status: http.StatusConflict},
		{name: "pending admin", role: "admin", pending: true, status: http.StatusConflict},
		{name: "missing file", role: "admin", missing: true, status: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := mypageTestServer(t)
			actor := mypageTestUser(t, s.db, tc.role)
			artifact := mypageTestArtifact(t, s.db)
			if tc.assigned {
				mypageAssign(t, s.db, actor.ID, artifact.ID)
			}
			file := model.FileObject{ID: uuid.NewString(), ArtifactID: artifact.ID, OriginalFilename: "synthetic.txt", SizeBytes: 123}
			if tc.pending {
				now := time.Now().UTC()
				file.DeletionRequestedAt = &now
			}
			if !tc.missing {
				if err := s.db.Create(&file).Error; err != nil {
					t.Fatal(err)
				}
				if !tc.pending {
					node := model.Node{Name: "delete-test-node", NodeType: "A", Status: "online"}
					if err := s.db.Create(&node).Error; err != nil {
						t.Fatal(err)
					}
					placement := model.Placement{FileObjectID: file.ID, NodeID: node.ID, StorageKey: "objects/" + file.ID, Status: "verified"}
					if err := s.db.Create(&placement).Error; err != nil {
						t.Fatal(err)
					}
				}
			}
			rec := mypageRequest(t, http.MethodDelete, "/:id", "/"+file.ID, "", &actor, s.deleteFileObject)
			if rec.Code != tc.status {
				t.Fatalf("status=%d want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if tc.missing {
				return
			}
			var stored model.FileObject
			if err := s.db.Unscoped().First(&stored, "id = ?", file.ID).Error; err != nil {
				t.Fatalf("physical file record lost: %v", err)
			}
			if tc.status == http.StatusAccepted {
				if stored.DeletedAt.Valid || stored.DeletionRequestedAt == nil {
					t.Fatalf("expected pending physical deletion, got deleted_at=%v deletion_requested_at=%v", stored.DeletedAt, stored.DeletionRequestedAt)
				}
			} else if stored.DeletedAt.Valid != (tc.status == http.StatusNoContent) {
				t.Fatalf("deleted=%v", stored.DeletedAt.Valid)
			}
			if stored.SizeBytes != file.SizeBytes || stored.OriginalFilename != file.OriginalFilename {
				t.Fatal("file metadata was changed")
			}
			var tasks int64
			if err := s.db.Model(&model.Task{}).Count(&tasks).Error; err != nil {
				t.Fatal(err)
			}
			wantTasks := int64(0)
			if tc.status == http.StatusAccepted {
				wantTasks = 1
			}
			if tasks != wantTasks {
				t.Fatalf("delete tasks=%d want %d", tasks, wantTasks)
			}
			if tc.status == http.StatusAccepted {
				again := mypageRequest(t, http.MethodDelete, "/:id", "/"+file.ID, "", &actor, s.deleteFileObject)
				mypageError(t, again, http.StatusNotFound, "file object not found")
			}
		})
	}
}

func TestMypageCatalogFiltersIntegration(t *testing.T) {
	s := mypageTestServer(t)
	author := mypageTestUser(t, s.db, "author")
	other := mypageTestUser(t, s.db, "author")
	assigned := mypageTestArtifact(t, s.db)
	hidden := mypageTestArtifact(t, s.db)
	mypageAssign(t, s.db, author.ID, assigned.ID)
	mypageAssign(t, s.db, other.ID, hidden.ID)
	for _, artifact := range []model.Artifact{assigned, hidden} {
		file := model.FileObject{ID: uuid.NewString(), ArtifactID: artifact.ID, OriginalFilename: "synthetic.txt"}
		if err := s.db.Create(&file).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name    string
		handler gin.HandlerFunc
		want    string
	}{
		{"artifacts", s.listArtifacts, assigned.ID},
		{"files", s.listFiles, assigned.ID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, query := range []string{"?all=true", "?page=1&limit=10"} {
				rec := mypageRequest(t, http.MethodGet, "/", "/"+query, "", &author, tc.handler)
				if rec.Code != http.StatusOK {
					t.Fatalf("%d %s", rec.Code, rec.Body.String())
				}
				var response struct {
					Items []struct {
						ID         string `json:"id"`
						ArtifactID string `json:"artifact_id"`
					} `json:"items"`
					Total int `json:"total"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.Total != 1 || len(response.Items) != 1 {
					t.Fatalf("filtered catalog=%s", rec.Body.String())
				}
				id := response.Items[0].ID
				if tc.name == "files" {
					id = response.Items[0].ArtifactID
				}
				if id != tc.want {
					t.Fatalf("unassigned artifact leaked: %s", rec.Body.String())
				}
			}
		})
	}
	catalogExec(t, s.db, "DELETE FROM author_artifact WHERE user_id = ?", author.ID)
	for _, handler := range []gin.HandlerFunc{s.listArtifacts, s.listFiles} {
		rec := mypageRequest(t, http.MethodGet, "/", "/?all=true", "", &author, handler)
		var response struct {
			Total int               `json:"total"`
			Items []json.RawMessage `json:"items"`
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Total != 0 || len(response.Items) != 0 {
			t.Fatalf("revoked catalog access: %s", rec.Body.String())
		}
	}
}

func TestMypageOwnIdentitiesIsolationIntegration(t *testing.T) {
	s := mypageTestServer(t)
	if err := s.db.AutoMigrate(&model.OidcIdentity{}); err != nil {
		t.Fatal(err)
	}
	actor := mypageTestUser(t, s.db, "author")
	other := mypageTestUser(t, s.db, "author")
	providerID := uuid.NewString()
	own := model.OidcIdentity{ID: uuid.NewString(), Issuer: "https://issuer.example.test", Subject: "self", Email: actor.Email, UserID: actor.ID, ProviderID: providerID}
	foreign := model.OidcIdentity{ID: uuid.NewString(), Issuer: own.Issuer, Subject: "other", Email: other.Email, UserID: other.ID, ProviderID: providerID}
	for _, identity := range []model.OidcIdentity{own, foreign} {
		if err := s.db.Create(&identity).Error; err != nil {
			t.Fatal(err)
		}
	}
	rec := mypageRequest(t, http.MethodGet, "/", "/", "", &actor, s.listOwnUserOidcIdentities)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || response.Items[0].ID != own.ID {
		t.Fatalf("linked accounts leaked: %s", rec.Body.String())
	}
	// Supplying another user's identity ID must leave that identity intact.
	rec = mypageRequest(t, http.MethodDelete, "/:identityID", "/"+foreign.ID, "", &actor, s.deleteOwnUserOidcIdentity)
	mypageError(t, rec, http.StatusNotFound, "OIDC identity not found")
	var stored model.OidcIdentity
	if err := s.db.First(&stored, "id = ?", foreign.ID).Error; err != nil {
		t.Fatalf("another user's identity was deleted: %v", err)
	}
	rec = mypageRequest(t, http.MethodDelete, "/:identityID", "/"+own.ID, "", &actor, s.deleteOwnUserOidcIdentity)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unlink self: %d %s", rec.Code, rec.Body.String())
	}
	var count int64
	if err := s.db.Model(&model.OidcIdentity{}).Where("id = ?", own.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("own identity remains linked")
	}
}
