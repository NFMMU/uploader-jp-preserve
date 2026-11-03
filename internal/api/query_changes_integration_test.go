package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"github.com/nfmmu/uploader-archive/internal/gen/query"
)

// These tests use the same disposable, rolled-back PostgreSQL schema as the
// catalog integration tests. The generated query package holds global state,
// so tests using this helper must not run in parallel.
func queryChangesDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, _ := catalogTestDB(t)
	previous := *query.Q
	artifact, file, policy := query.Artifact, query.FileObject, query.Policy
	node, placement, job := query.Node, query.Placement, query.DownloadJob
	audit, downloadAudit, user := query.AuditLog, query.DownloadAuditLog, query.User
	enrollment, publicRequest, conversation := query.NodeEnrollmentRequest, query.PublicRequest, query.RequestConversationLink
	registry, task := query.SourceURLRegistry, query.Task
	query.SetDefault(db)
	t.Cleanup(func() {
		*query.Q = previous
		query.Artifact, query.FileObject, query.Policy = artifact, file, policy
		query.Node, query.Placement, query.DownloadJob = node, placement, job
		query.AuditLog, query.DownloadAuditLog, query.User = audit, downloadAudit, user
		query.NodeEnrollmentRequest, query.PublicRequest, query.RequestConversationLink = enrollment, publicRequest, conversation
		query.SourceURLRegistry, query.Task = registry, task
	})
	return db
}

func insertQueryFixture(t *testing.T, db *gorm.DB) (model.Artifact, model.FileObject) {
	t.Helper()
	a := model.Artifact{ID: uuid.NewString(), Title: "Example", SourceURL: "https://example.invalid/source"}
	f := model.FileObject{ID: uuid.NewString(), ArtifactID: a.ID, OriginalFilename: "example.bin", SizeBytes: 12, PreservationStatus: "verified", RedistributionPolicy: "allowed", Sha256: new(strings.Repeat("a", 64))}
	for _, v := range []any{&a, &f} {
		if err := db.Create(v).Error; err != nil {
			t.Fatal(err)
		}
	}
	return a, f
}

func TestGeneratedCatalogQueriesIntegration(t *testing.T) {
	db := queryChangesDB(t)
	// Production migrations use ICU collations for catalog text and UUID casts.
	catalogExec(t, db, `CREATE COLLATION IF NOT EXISTS public.case_strict_insensitive (provider = icu, locale = 'und-u-ks-level2', deterministic = false)`)
	catalogExec(t, db, `CREATE COLLATION IF NOT EXISTS public.case_insensitive (provider = icu, locale = 'und-u-ks-level1', deterministic = false)`)
	catalogExec(t, db, `ALTER TABLE artifacts ALTER COLUMN title TYPE varchar(500) COLLATE public.case_insensitive, ALTER COLUMN creator_name TYPE varchar(255) COLLATE public.case_insensitive, ALTER COLUMN description TYPE text COLLATE public.case_insensitive`)
	catalogExec(t, db, `ALTER TABLE file_objects ALTER COLUMN original_filename TYPE varchar(500) COLLATE public.case_insensitive`)
	if err := db.AutoMigrate(&model.Policy{}); err != nil {
		t.Fatal(err)
	}
	a, file := insertQueryFixture(t, db)
	catalogExec(t, db, `UPDATE artifacts SET creator_name = 'CreatorName', description = 'DescriptionNeedle' WHERE id = ?`, a.ID)
	now := time.Now().UTC().Truncate(time.Second)
	pending := model.FileObject{ID: uuid.NewString(), ArtifactID: a.ID, OriginalFilename: "pending.bin", SizeBytes: 2, DeletionRequestedAt: &now}
	deleted := model.FileObject{ID: uuid.NewString(), ArtifactID: a.ID, OriginalFilename: "deleted.bin", SizeBytes: 3, DeletedAt: gorm.DeletedAt{Time: now, Valid: true}}
	for _, f := range []*model.FileObject{&pending, &deleted} {
		if err := db.Create(f).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Server{db: db}
	for _, term := range []string{"example", "creatorname", "source", "descriptionneedle"} {
		t.Run("artifact search "+term, func(t *testing.T) {
			var got struct {
				Items []model.Artifact
				Total int
			}
			if status := catalogResponse(t, s.listArtifacts, "/?q="+url.QueryEscape(term), &got); status != http.StatusOK || got.Total != 1 || len(got.Items) != 1 || got.Items[0].ID != a.ID {
				t.Fatalf("status=%d response=%+v", status, got)
			}
		})
	}
	for _, tc := range []struct {
		name, path string
		want       []string
	}{
		{"default excludes deleted and pending", "/?all=true", []string{file.ID}},
		{"include pending deletion", "/?all=true&include_deleted=true", []string{file.ID, pending.ID}},
		{"search combined with visibility", "/?all=true&q=example", []string{file.ID}},
		{"artifact filter", "/?all=true&artifact_id=" + a.ID, []string{file.ID}},
		{"no match", "/?all=true&q=does-not-exist", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got struct {
				Items []fileListItem
				Total int
			}
			status := catalogResponse(t, s.listFiles, tc.path, &got)
			ids := map[string]bool{}
			for _, item := range got.Items {
				ids[item.ID] = true
				if item.ArtifactTitle != a.Title {
					t.Errorf("missing artifact metadata: %+v", item)
				}
			}
			if status != http.StatusOK || got.Total != len(tc.want) || len(ids) != len(tc.want) {
				t.Fatalf("status=%d total=%d ids=%v want=%v", status, got.Total, ids, tc.want)
			}
			for _, id := range tc.want {
				if !ids[id] {
					t.Errorf("missing file %s", id)
				}
			}
		})
	}
	// Public listings require every publication and placement condition.
	seen := now.Add(-time.Second)
	node := model.Node{ID: uuid.NewString(), Name: "healthy", NodeType: "A", Status: "online", LastSeenAt: &seen}
	placement := model.Placement{FileObjectID: file.ID, NodeID: node.ID, StorageKey: "object", Status: "verified"}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&placement).Error; err != nil {
		t.Fatal(err)
	}
	catalogExec(t, db, `UPDATE file_objects SET public_download_enabled = true WHERE id = ?`, file.ID)
	for _, tc := range []struct {
		name, mutation string
		count          int
	}{
		{"healthy", "", 1},
		{"offline node", "UPDATE nodes SET status = 'offline'", 0},
		{"stale node", "UPDATE nodes SET last_seen_at = now() - interval '3 minutes'", 0},
		{"unverified placement", "UPDATE placements SET status = 'queued'", 0},
		{"invalid digest", "UPDATE file_objects SET sha256 = 'invalid'", 0},
		{"restricted policy", "UPDATE file_objects SET redistribution_policy = 'restricted'", 0},
		{"private flag", "UPDATE file_objects SET public_download_enabled = false", 0},
	} {
		t.Run("public "+tc.name, func(t *testing.T) {
			catalogExec(t, db, `UPDATE nodes SET status = 'online', last_seen_at = now()`)
			catalogExec(t, db, `UPDATE placements SET status = 'verified'`)
			catalogExec(t, db, `UPDATE file_objects SET public_download_enabled = true, redistribution_policy = 'allowed', sha256 = ? WHERE id = ?`, strings.Repeat("a", 64), file.ID)
			if tc.mutation != "" {
				catalogExec(t, db, tc.mutation)
			}
			var got struct {
				Items []fileListItem
				Total int
			}
			status := catalogResponse(t, s.listFiles, "/?all=true&publication=public", &got)
			if status != http.StatusOK || got.Total != tc.count || len(got.Items) != tc.count {
				t.Errorf("status=%d total=%d items=%d, want %d", status, got.Total, len(got.Items), tc.count)
			}
		})
	}
	for _, path := range []string{"/?artifact_id=invalid", "/?preservation_status=invalid", "/?redistribution_policy=invalid", "/?publication=invalid"} {
		var got map[string]any
		if status := catalogResponse(t, s.listFiles, path, &got); status != http.StatusBadRequest {
			t.Errorf("%s status=%d", path, status)
		}
	}
	older, newer := now.Add(-time.Hour), now.Add(-time.Minute)
	for _, p := range []model.Policy{{FileObjectID: file.ID, PolicyType: "allowed", CheckedAt: &older, CreatedAt: now}, {FileObjectID: file.ID, PolicyType: "restricted", CheckedAt: &newer, CreatedAt: older}} {
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
	}
	var detail struct {
		File    model.FileObject
		History []model.Policy `json:"policy_history"`
	}
	if status := catalogResponse(t, s.getFileObject, "/"+file.ID, &detail); status != http.StatusOK || len(detail.History) != 2 || detail.History[0].PolicyType != "restricted" {
		t.Fatalf("status=%d history=%+v", status, detail.History)
	}
	for _, id := range []string{pending.ID, deleted.ID, uuid.NewString()} {
		var got map[string]any
		if status := catalogResponse(t, s.getFileObject, "/"+id, &got); status != http.StatusNotFound {
			t.Errorf("file %s status=%d", id, status)
		}
	}
}

func TestGeneratedDownloadQueriesIntegration(t *testing.T) {
	db := queryChangesDB(t)
	// Use the production table definition's relevant columns, including NULL leases.
	catalogExec(t, db, `CREATE TABLE download_jobs (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), file_object_id uuid NOT NULL, selected_node_id uuid, status varchar(16) NOT NULL, access_token_hash varchar(64) NOT NULL, lease_token uuid, lease_until timestamptz, attempt_count integer NOT NULL DEFAULT 0, size_bytes bigint, sha256 varchar(64), last_error text, expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now())`)
	_, file := insertQueryFixture(t, db)
	now := time.Now().UTC()
	for i, n := range []struct {
		status          string
		age             time.Duration
		placementStatus string
	}{{"online", time.Second, "verified"}, {"online", time.Second, "verified"}, {"offline", time.Second, "verified"}, {"online", 3 * time.Minute, "verified"}, {"online", time.Second, "queued"}} {
		node := model.Node{ID: uuid.NewString(), Name: uuid.NewString(), NodeType: "A", Status: n.status, LastSeenAt: new(now.Add(-n.age))}
		p := model.Placement{ID: uuid.NewString(), FileObjectID: file.ID, NodeID: node.ID, StorageKey: "object", Status: n.placementStatus, CreatedAt: now.Add(time.Duration(i-5) * time.Minute)}
		if err := db.Create(&node).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Server{db: db, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	_, p, err := s.fileDownloadIsAvailable(nil, file.ID)
	if err != nil || p == nil || !p.CreatedAt.Equal(now.Add(-5*time.Minute).Truncate(time.Microsecond)) {
		t.Fatalf("available placement=%+v err=%v", p, err)
	}
	for _, update := range []string{"preservation_status = 'error'", "sha256 = NULL", "sha256 = 'short'", "size_bytes = -1"} {
		catalogExec(t, db, `UPDATE file_objects SET preservation_status = 'verified', sha256 = ?, size_bytes = 12 WHERE id = ?`, strings.Repeat("a", 64), file.ID)
		catalogExec(t, db, `UPDATE file_objects SET `+update+` WHERE id = ?`, file.ID)
		_, _, err := s.fileDownloadIsAvailable(nil, file.ID)
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("%s: %v", update, err)
		}
	}
	lease := uuid.NewString()
	for _, tc := range []struct {
		name, status string
		attempts     int
		expiry       time.Time
		supplied     *string
		wantStatus   string
		changed      bool
	}{
		{"retry", "running", 2, now.Add(time.Hour), &lease, "queued", true},
		{"attempt limit", "running", 3, now.Add(time.Hour), &lease, "failed", true},
		{"expired", "running", 1, now.Add(-time.Hour), &lease, "failed", true},
		{"different lease", "running", 1, now.Add(time.Hour), new(uuid.NewString()), "running", false},
		{"not running", "ready", 1, now.Add(time.Hour), &lease, "ready", false},
		{"missing lease", "running", 1, now.Add(time.Hour), nil, "running", false},
		{"empty lease", "running", 1, now.Add(time.Hour), new(""), "running", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.NewString()
			until := now.Add(time.Minute)
			catalogExec(t, db, `INSERT INTO download_jobs (id, file_object_id, status, access_token_hash, lease_token, lease_until, attempt_count, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id, file.ID, tc.status, tokenHash("secret"), lease, until, tc.attempts, tc.expiry)
			s.failDelivery(model.DownloadJob{ID: id, LeaseToken: tc.supplied}, "synthetic failure")
			var got model.DownloadJob
			if err := db.First(&got, "id = ?", id).Error; err != nil {
				t.Fatal(err)
			}
			if got.Status != tc.wantStatus {
				t.Errorf("status=%s want=%s", got.Status, tc.wantStatus)
			}
			if tc.changed {
				if got.LeaseToken != nil || got.LeaseUntil != nil || got.LastError == nil || *got.LastError != "synthetic failure" {
					t.Errorf("updated job=%+v", got)
				}
			} else if got.LeaseToken == nil || *got.LeaseToken != lease || got.LeaseUntil == nil || got.LastError != nil {
				t.Errorf("job changed: %+v", got)
			}
		})
	}
}

func TestDownloadAuthorizationAndResponseIntegration(t *testing.T) {
	db := queryChangesDB(t)
	catalogExec(t, db, `CREATE TABLE download_jobs (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), file_object_id uuid NOT NULL, selected_node_id uuid, status varchar(16) NOT NULL, access_token_hash varchar(64) NOT NULL, lease_token uuid, lease_until timestamptz, attempt_count integer NOT NULL DEFAULT 0, size_bytes bigint, sha256 varchar(64), last_error text, expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now())`)
	_, file := insertQueryFixture(t, db)
	id, expired := uuid.NewString(), uuid.NewString()
	for _, row := range []struct {
		id     string
		expiry time.Time
	}{{id, time.Now().Add(time.Hour)}, {expired, time.Now().Add(-time.Hour)}} {
		catalogExec(t, db, `INSERT INTO download_jobs (id, file_object_id, status, access_token_hash, expires_at) VALUES (?, ?, 'queued', ?, ?)`, row.id, file.ID, tokenHash("secret"), row.expiry)
	}
	s := &Server{db: db}
	router := gin.New()
	router.GET("/:id", s.getDownloadJob)
	for _, tc := range []struct {
		name, id, auth string
		status         int
	}{
		{"valid", id, "Bearer secret", http.StatusOK}, {"bad id", "invalid", "Bearer secret", http.StatusBadRequest}, {"missing bearer", id, "", http.StatusUnauthorized}, {"wrong token", id, "Bearer wrong", http.StatusNotFound}, {"expired", expired, "Bearer secret", http.StatusNotFound}, {"unknown job", uuid.NewString(), "Bearer secret", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/"+tc.id, nil)
			req.Header.Set("Download-Authorization", tc.auth)
			router.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if tc.status == http.StatusOK {
				var body map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body["id"] != id {
					t.Errorf("job=%v", body)
				}
				if _, ok := body["access_token_hash"]; ok {
					t.Error("response exposes access token hash")
				}
				if _, ok := body["lease_token"]; ok {
					t.Error("response exposes lease token")
				}
			}
		})
	}
}

func TestGeneratedPolicyUpdateIntegration(t *testing.T) {
	db := queryChangesDB(t)
	if err := db.AutoMigrate(&model.Policy{}, &model.AuditLog{}, &model.User{}); err != nil {
		t.Fatal(err)
	}
	_, file := insertQueryFixture(t, db)
	catalogExec(t, db, `UPDATE file_objects SET public_download_enabled = true WHERE id = ?`, file.ID)
	s := &Server{db: db, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	router := gin.New()
	router.PATCH("/:id", func(c *gin.Context) {
		c.Set("currentUser", model.User{ID: uuid.NewString(), Email: "reviewer@example.invalid"})
		s.updateFilePolicy(c)
	})
	update := func(id, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, "/"+id, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		return w
	}
	w := update(file.ID, `{"redistribution_policy":"restricted","reason":"reviewed","evidence_note":"  evidence  "}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var updated model.FileObject
	if err := db.First(&updated, "id = ?", file.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.RedistributionPolicy != "restricted" || updated.PublicDownloadEnabled {
		t.Errorf("updated file=%+v", updated)
	}
	var policies []model.Policy
	if err := db.Where("file_object_id = ?", file.ID).Find(&policies).Error; err != nil {
		t.Fatal(err)
	}
	if len(policies) != 1 || policies[0].PolicyType != "restricted" || policies[0].CheckedAt == nil || policies[0].CheckedBy == nil || policies[0].EvidenceNote == nil || *policies[0].EvidenceNote != "evidence" {
		t.Fatalf("policies=%+v", policies)
	}
	var audits []model.AuditLog
	if err := db.Find(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if len(audits) != 1 || audits[0].Action != "file_object.policy.update" || audits[0].TargetID == nil || *audits[0].TargetID != file.ID {
		t.Errorf("audits=%+v", audits)
	}

	// A file with deletion pending must remain inaccessible to policy updates.
	catalogExec(t, db, `UPDATE file_objects SET deletion_requested_at = now() WHERE id = ?`, file.ID)
	w = update(file.ID, `{"redistribution_policy":"allowed","reason":"reviewed"}`)
	if w.Code != http.StatusNotFound {
		t.Errorf("pending deletion status=%d body=%s", w.Code, w.Body.String())
	}
	if err := db.First(&updated, "id = ?", file.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.RedistributionPolicy != "restricted" {
		t.Errorf("pending deletion policy changed to %s", updated.RedistributionPolicy)
	}
	// A failed history insert must roll back the file update as well.
	catalogExec(t, db, `UPDATE file_objects SET deletion_requested_at = NULL, redistribution_policy = 'restricted' WHERE id = ?`, file.ID)
	catalogExec(t, db, `ALTER TABLE policies ADD CONSTRAINT reject_allowed CHECK (policy_type <> 'allowed') NOT VALID`)
	w = update(file.ID, `{"redistribution_policy":"allowed","reason":"reviewed"}`)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("failed insert status=%d body=%s", w.Code, w.Body.String())
	}
	if err := db.First(&updated, "id = ?", file.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.RedistributionPolicy != "restricted" {
		t.Errorf("failed transaction changed policy to %s", updated.RedistributionPolicy)
	}
}

func TestCreateDownloadJobResponseIntegration(t *testing.T) {
	db := queryChangesDB(t)
	catalogExec(t, db, `CREATE TABLE download_jobs (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), file_object_id uuid NOT NULL, selected_node_id uuid, status varchar(16) NOT NULL, access_token_hash varchar(64) NOT NULL, lease_token uuid, lease_until timestamptz, attempt_count integer NOT NULL DEFAULT 0, size_bytes bigint, sha256 varchar(64), last_error text, expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now())`)
	if err := db.AutoMigrate(&model.DownloadAuditLog{}); err != nil {
		t.Fatal(err)
	}
	_, file := insertQueryFixture(t, db)
	now := time.Now()
	node := model.Node{ID: uuid.NewString(), Name: "online", NodeType: "A", Status: "online", LastSeenAt: &now}
	placement := model.Placement{FileObjectID: file.ID, NodeID: node.ID, StorageKey: "object", Status: "verified"}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&placement).Error; err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db}
	router := gin.New()
	router.POST("/:id", s.createDownloadJob)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/"+file.ID, nil))
	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var response struct {
		Job   map[string]any `json:"job"`
		Token string         `json:"access_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Token == "" || response.Job["status"] != "queued" {
		t.Errorf("unexpected response: %s", w.Body.String())
	}
	if _, ok := response.Job["access_token_hash"]; ok {
		t.Error("response exposes access token hash")
	}
	if _, ok := response.Job["lease_token"]; ok {
		t.Error("response exposes lease token")
	}
	var saved model.DownloadJob
	if err := db.First(&saved, "id = ?", response.Job["id"]).Error; err != nil {
		t.Fatal(err)
	}
	if saved.AccessTokenHash != tokenHash(response.Token) || saved.ExpiresAt.Before(now.Add(29*time.Minute)) || saved.ExpiresAt.After(now.Add(31*time.Minute)) {
		t.Errorf("stored job does not match token or expected expiry")
	}
}

func TestGeneratedCatalogWritesIntegration(t *testing.T) {
	db := queryChangesDB(t)
	if err := db.AutoMigrate(&model.User{}, &model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	actor := model.User{ID: uuid.NewString(), Email: "actor@example.invalid", PasswordHash: "hash", DisplayName: "Actor", Role: "admin"}
	active := model.User{ID: uuid.NewString(), Email: "reviewer@example.invalid", PasswordHash: "hash", DisplayName: "Reviewer", Role: "reviewer"}
	disabled := model.User{ID: uuid.NewString(), Email: "disabled@example.invalid", PasswordHash: "hash", DisplayName: "Disabled", Role: "reviewer", DisabledAt: new(time.Now())}
	viewer := model.User{ID: uuid.NewString(), Email: "viewer@example.invalid", PasswordHash: "hash", DisplayName: "Viewer", Role: "viewer"}
	for _, u := range []*model.User{&actor, &active, &disabled, &viewer} {
		if err := db.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Server{db: db, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("currentUser", actor); c.Next() })
	router.POST("/artifacts", s.createArtifact)
	router.POST("/artifacts/:id/files", s.createFileObject)
	router.PATCH("/files/:id/publication", s.updatePublicationFlag)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		return w
	}
	w := request(http.MethodPost, "/artifacts", `{"title":"  Title  ","source_url":"https://uploader.jp/test","description":"  description  "}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("artifact status=%d body=%s", w.Code, w.Body.String())
	}
	var artifact model.Artifact
	if err := json.Unmarshal(w.Body.Bytes(), &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.Title != "Title" || artifact.Description == nil || *artifact.Description != "description" {
		t.Errorf("artifact=%+v", artifact)
	}
	if w = request(http.MethodPost, "/artifacts", `{"title":"bad","source_url":"https://example.invalid/"}`); w.Code != http.StatusBadRequest {
		t.Errorf("invalid source status=%d", w.Code)
	}
	for _, tc := range []struct {
		name, representative string
		status               int
	}{{"active reviewer", active.ID, http.StatusCreated}, {"disabled reviewer", disabled.ID, http.StatusBadRequest}, {"viewer", viewer.ID, http.StatusBadRequest}, {"missing user", uuid.NewString(), http.StatusBadRequest}} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{"original_filename": " example.bin ", "size_bytes": 12, "sha256": strings.Repeat("A", 64), "representative": tc.representative})
			if err != nil {
				t.Fatal(err)
			}
			w := request(http.MethodPost, "/artifacts/"+artifact.ID+"/files", string(body))
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if tc.status == http.StatusCreated {
				var file model.FileObject
				if err := json.Unmarshal(w.Body.Bytes(), &file); err != nil {
					t.Fatal(err)
				}
				if file.OriginalFilename != "example.bin" || file.Sha256 == nil || *file.Sha256 != strings.Repeat("a", 64) || file.PreservationStatus != "uncollected" || file.RedistributionPolicy != "unknown" || file.PublicDownloadEnabled {
					t.Errorf("file=%+v", file)
				}
			}
		})
	}
	_, file := insertQueryFixture(t, db)
	now := time.Now()
	node := model.Node{ID: uuid.NewString(), Name: "publication", NodeType: "A", Status: "online", LastSeenAt: &now}
	placement := model.Placement{FileObjectID: file.ID, NodeID: node.ID, StorageKey: "object", Status: "verified"}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&placement).Error; err != nil {
		t.Fatal(err)
	}
	path := "/files/" + file.ID + "/publication"
	w = request(http.MethodPatch, path, `{"enabled":true,"reason":"reviewed"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("enable status=%d body=%s", w.Code, w.Body.String())
	}
	var saved model.FileObject
	if err := db.First(&saved, "id = ?", file.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !saved.PublicDownloadEnabled {
		t.Error("publication not enabled")
	}
	w = request(http.MethodPatch, path, `{"enabled":false,"reason":"reviewed"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", w.Code, w.Body.String())
	}
	catalogExec(t, db, `UPDATE nodes SET last_seen_at = now() - interval '3 minutes' WHERE id = ?`, node.ID)
	w = request(http.MethodPatch, path, `{"enabled":true,"reason":"reviewed"}`)
	if w.Code != http.StatusConflict {
		t.Errorf("stale node status=%d body=%s", w.Code, w.Body.String())
	}
	if err := db.First(&saved, "id = ?", file.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.PublicDownloadEnabled {
		t.Error("stale node enabled publication")
	}
}
