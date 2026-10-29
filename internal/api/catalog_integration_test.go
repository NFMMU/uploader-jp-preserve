package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type catalogQueryCounter struct {
	logger.Interface
	queries int
}

func (l *catalogQueryCounter) Trace(ctx context.Context, begin time.Time, sql func() (string, int64), err error) {
	l.queries++
	l.Interface.Trace(ctx, begin, sql, err)
}

func catalogTestDB(t *testing.T) (*gorm.DB, *catalogQueryCounter) {
	t.Helper()
	if os.Getenv("RUN_CATALOG_DB_INTEGRATION") != "1" {
		t.Skip("set RUN_CATALOG_DB_INTEGRATION=1 and TEST_DATABASE_URL to a disposable local archive_test database")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	parsed, err := url.Parse(dsn)
	if err != nil || parsed == nil || parsed.Path != "/archive_test" || parsed.Port() != "55432" || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1") {
		t.Fatal("refusing integration test database; expected localhost:55432 database /archive_test")
	}
	counter := &catalogQueryCounter{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: counter})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	// Each test owns a schema; rollback removes its synthetic data and schema.
	schema := "catalog_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	catalogExec(t, tx, "CREATE SCHEMA "+schema)
	catalogExec(t, tx, "SET LOCAL search_path TO "+schema)
	if err := tx.AutoMigrate(&model.Artifact{}, &model.FileObject{}, &model.PublicRequest{}, &model.Node{}, &model.Placement{}); err != nil {
		t.Fatal(err)
	}
	return tx, counter
}

func catalogExec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatal(err)
	}
}

func catalogResponse(t *testing.T, handler gin.HandlerFunc, path string, out any) int {
	t.Helper()
	router := gin.New()
	router.GET("/", handler)
	router.GET("/:id", handler)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if err := json.Unmarshal(response.Body.Bytes(), out); err != nil {
		t.Fatalf("decode response (%d): %v", response.Code, err)
	}
	return response.Code
}

func TestListFilesAllIntegration(t *testing.T) {
	db, _ := catalogTestDB(t)
	// More distinct artifacts than the PostgreSQL extended protocol permits
	// as bind parameters in a single statement.
	const count = 65536
	catalogExec(t, db, `INSERT INTO artifacts (id, title, source_url, created_at, updated_at)
		SELECT md5(i::text)::uuid, 'artifact-' || i, 'https://example.invalid/' || i, now(), now()
		FROM generate_series(1, ?) i`, count)
	catalogExec(t, db, `INSERT INTO file_objects (id, artifact_id, original_filename, size_bytes,
		preservation_status, redistribution_policy, public_download_enabled, created_at)
		SELECT md5('file-' || i)::uuid, md5(i::text)::uuid, 'file-' || i, i, 'verified', 'unknown', false, now()
		FROM generate_series(1, ?) i`, count)
	s := &Server{db: db}
	var response struct {
		Items              []fileListItem
		Page, Limit, Total int
	}
	if status := catalogResponse(t, s.listFiles, "/?all=true", &response); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if len(response.Items) != count || response.Total != count || response.Page != 0 || response.Limit != 0 {
		t.Fatalf("items=%d total=%d page=%d limit=%d", len(response.Items), response.Total, response.Page, response.Limit)
	}
	for _, item := range response.Items {
		if item.ArtifactTitle != "artifact-"+fmt.Sprint(item.SizeBytes) {
			t.Fatalf("missing or incorrect metadata for %s: %q", item.ID, item.ArtifactTitle)
		}
	}
	if status := catalogResponse(t, s.listFiles, "/?page=2&limit=7", &response); status != http.StatusOK || len(response.Items) != 7 || response.Page != 2 || response.Limit != 7 || response.Total != count {
		t.Fatalf("paged files: status=%d items=%d page=%d limit=%d total=%d", status, len(response.Items), response.Page, response.Limit, response.Total)
	}
}

func TestListPublicRequestsAllIntegration(t *testing.T) {
	db, counter := catalogTestDB(t)
	artifact := model.Artifact{Title: "file artifact", SourceURL: "https://example.invalid/file"}
	explicit := model.Artifact{Title: "explicit artifact", SourceURL: "https://example.invalid/explicit"}
	for _, record := range []any{&artifact, &explicit} {
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	file := model.FileObject{ArtifactID: artifact.ID, OriginalFilename: "linked.bin", PreservationStatus: "verified", RedistributionPolicy: "unknown"}
	if err := db.Create(&file).Error; err != nil {
		t.Fatal(err)
	}
	secret := "synthetic-encrypted-password"
	missingID := uuid.NewString()
	requests := []model.PublicRequest{
		{RequestType: "preservation", Status: "pending", FileObjectID: &file.ID, SourcePasswordEncrypted: &secret},
		{RequestType: "preservation", Status: "pending", FileObjectID: &file.ID, ArtifactID: &explicit.ID},
		{RequestType: "preservation", Status: "pending", ArtifactID: &explicit.ID},
		{RequestType: "preservation", Status: "pending"},
		{RequestType: "preservation", Status: "pending", FileObjectID: &missingID},
		{RequestType: "preservation", Status: "pending", ArtifactID: &missingID},
		{RequestType: "preservation", Status: "in_review", FileObjectID: &file.ID},
	}
	for i := range requests {
		requests[i].CreatedAt = time.Now().Add(time.Duration(i) * time.Second)
		if err := db.Create(&requests[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Server{db: db}
	var all struct {
		Items              []publicRequestListItem
		Page, Limit, Total int
	}
	counter.queries = 0
	if status := catalogResponse(t, s.listPublicRequests, "/?all=true&status=pending", &all); status != http.StatusOK {
		t.Fatalf("status=%d", status)
	}
	if counter.queries != 2 {
		t.Fatalf("all requests used %d queries, want 2", counter.queries)
	}
	if len(all.Items) != 6 || all.Total != 6 || all.Page != 0 || all.Limit != 0 {
		t.Fatalf("all requests: items=%d total=%d page=%d limit=%d", len(all.Items), all.Total, all.Page, all.Limit)
	}
	if all.Items[0].ArtifactID == nil || *all.Items[0].ArtifactID != artifact.ID || all.Items[0].LinkedFileName != file.OriginalFilename || all.Items[0].LinkedArtifactTitle != artifact.Title || !all.Items[0].HasSourcePassword {
		t.Fatalf("file artifact fallback or metadata missing: %+v", all.Items[0])
	}
	if all.Items[1].ArtifactID == nil || *all.Items[1].ArtifactID != explicit.ID || all.Items[1].LinkedArtifactTitle != explicit.Title {
		t.Fatalf("explicit artifact was not preserved: %+v", all.Items[1])
	}
	// Compare the joined path with the existing paginated response, including
	// null links, missing records, ordering, and password-presence metadata.
	var paged struct {
		Items              []publicRequestListItem
		Page, Limit, Total int
	}
	if status := catalogResponse(t, s.listPublicRequests, "/?status=pending&limit=100", &paged); status != http.StatusOK {
		t.Fatalf("status=%d", status)
	}
	want, _ := json.Marshal(paged.Items)
	got, _ := json.Marshal(all.Items)
	if string(got) != string(want) {
		t.Fatal("all and paginated metadata differ")
	}
	if status := catalogResponse(t, s.listPublicRequests, "/?status=pending&page=2&limit=2", &paged); status != http.StatusOK || len(paged.Items) != 2 || paged.Items[0].ID != requests[2].ID || paged.Page != 2 || paged.Limit != 2 || paged.Total != 6 {
		t.Fatalf("paged requests: status=%d items=%d page=%d limit=%d total=%d", status, len(paged.Items), paged.Page, paged.Limit, paged.Total)
	}
}

func TestPublicCatalogPaginationIntegration(t *testing.T) {
	db, _ := catalogTestDB(t)
	catalogExec(t, db, `INSERT INTO artifacts (id, title, source_url, created_at, updated_at)
		SELECT md5(i::text)::uuid, lpad(i::text, 3, '0'), 'https://example.invalid/' || i, now(), now()
		FROM generate_series(1, 105) i`)
	catalogExec(t, db, `INSERT INTO nodes (id, name, node_type, status, capacity_bytes, used_bytes,
		available_bytes, requested_capacity_bytes, created_at, updated_at, last_seen_at)
		VALUES (md5('node')::uuid, 'test node', 'A', 'online', 1000, 0, 1000, 1000, now(), now(), now())`)
	catalogExec(t, db, `INSERT INTO file_objects (id, artifact_id, original_filename, size_bytes,
		sha256, preservation_status, redistribution_policy, public_download_enabled, created_at)
		SELECT md5('file-' || i)::uuid, md5(i::text)::uuid, 'file-' || i, i, repeat('a', 64), 'verified', 'allowed', true, now()
		FROM generate_series(1, 105) i`)
	catalogExec(t, db, `INSERT INTO placements (id, file_object_id, node_id, storage_key, status, created_at)
		SELECT md5('placement-' || i)::uuid, md5('file-' || i)::uuid, md5('node')::uuid, 'test', 'verified', now()
		FROM generate_series(1, 105) i`)
	s := &Server{db: db}
	var list struct {
		Items              []publicCatalogArtifact
		Page, Limit, Total int
	}
	if status := catalogResponse(t, s.listPublicArtifacts, "/?page=2&limit=999", &list); status != http.StatusOK || len(list.Items) != 5 || list.Total != 105 || list.Page != 2 || list.Limit != 100 {
		t.Fatalf("public list: status=%d items=%d total=%d page=%d limit=%d", status, len(list.Items), list.Total, list.Page, list.Limit)
	}
	for _, value := range []string{"true", "1", "TRUE"} {
		var failure map[string]any
		if status := catalogResponse(t, s.listPublicArtifacts, "/?all="+value, &failure); status != http.StatusBadRequest {
			t.Fatalf("all=%s status=%d", value, status)
		}
	}
	artifactID := list.Items[0].ID
	var detail struct {
		Item  publicCatalogArtifact `json:"item"`
		Page  int                   `json:"files_page"`
		Limit int                   `json:"files_limit"`
		Total int                   `json:"files_total"`
	}
	for _, tc := range []struct {
		query       string
		page, limit int
	}{
		{"?all=true&page=99&limit=99", 0, 0},
		{"?page=-1&limit=0", 1, 50},
		{"?limit=999", 1, 100},
	} {
		if status := catalogResponse(t, s.getPublicArtifact, "/"+artifactID+tc.query, &detail); status != http.StatusOK || detail.Page != tc.page || detail.Limit != tc.limit || detail.Total != 1 || len(detail.Item.Files) != 1 {
			t.Fatalf("detail %s: status=%d page=%d limit=%d total=%d files=%d", tc.query, status, detail.Page, detail.Limit, detail.Total, len(detail.Item.Files))
		}
	}
}
