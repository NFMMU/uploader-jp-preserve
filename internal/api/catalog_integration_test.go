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
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
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
	if parsed.User == nil || parsed.User.Username() != "archive_test" {
		t.Fatal("refusing integration test database; expected dedicated archive_test user")
	}
	password, hasPassword := parsed.User.Password()
	if !hasPassword || password != "archive_test_password" {
		t.Fatal("refusing integration test database; expected dedicated test password")
	}
	if parsed.RawQuery == "" || !strings.Contains(parsed.RawQuery, "sslmode=disable") {
		t.Fatal("refusing integration test database; expected explicit sslmode=disable")
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

func splitSQLStatements(sql string) []string {
	var statements []string
	start := 0
	var quote byte
	var dollarTag string
	for i := 0; i < len(sql); i++ {
		if dollarTag != "" {
			if strings.HasPrefix(sql[i:], dollarTag) {
				i += len(dollarTag) - 1
				dollarTag = ""
			}
			continue
		}
		if quote != 0 {
			if sql[i] == quote {
				if i+1 < len(sql) && sql[i+1] == quote {
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		switch sql[i] {
		case ''', '"':
			quote = sql[i]
		case '(t *testing.T) {
	db, _ := catalogTestDB(t)

	catalogExec(t, db, `CREATE TABLE author_artifact (
		user_id uuid NOT NULL,
		artifact_id uuid NOT NULL,
		created_at timestamptz NOT NULL DEFAULT now(),
		PRIMARY KEY (artifact_id, user_id)
	)`)
	catalogExec(t, db, `CREATE TABLE source_url_registry (
		normalized_url text PRIMARY KEY,
		source_url text NOT NULL,
		origin text NOT NULL,
		public_request_id uuid,
		created_at timestamptz NOT NULL DEFAULT now(),
		updated_at timestamptz NOT NULL DEFAULT now()
	)`)

	oldest := model.Artifact{CreatedAt: time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC), Title: "MMDkaz oldest", SourceURL: "https://uu.getuploader.com/MMDkaz/index/date/desc/2#artifact"}
	duplicate := model.Artifact{CreatedAt: time.Date(2026, time.November, 2, 0, 0, 0, 0, time.UTC), Title: "MMDkaz duplicate", SourceURL: "https://ux.getuploader.com:443/MMDkaz/page2#duplicate"}
	unrelated := model.Artifact{Title: "unrelated uploader", SourceURL: "https://uploader.jp/account"}
	for _, artifact := range []*model.Artifact{&oldest, &duplicate, &unrelated} {
		if err := db.Create(artifact).Error; err != nil {
			t.Fatalf("create artifact: %v", err)
		}
	}

	file7URL := "https://uu.getuploader.com:443/MMDkaz/DOWNLOAD/7/extra?token=file7#file7"
	file8URL := "https://ux.getuploader.com/MMDkaz/download/8/?token=abc/#file8"
	file7 := model.FileObject{
		ArtifactID: oldest.ID, OriginalFilename: "seven.zip", SizeBytes: 7,
		PreservationStatus: "stored", RedistributionPolicy: "unknown", SourceURL: &file7URL,
	}
	file8 := model.FileObject{
		ArtifactID: duplicate.ID, OriginalFilename: "eight.zip", SizeBytes: 8,
		PreservationStatus: "stored", RedistributionPolicy: "unknown", SourceURL: &file8URL,
	}
	for _, file := range []*model.FileObject{&file7, &file8} {
		if err := db.Create(file).Error; err != nil {
			t.Fatalf("create file: %v", err)
		}
	}

	requestWithFileURL := "https://uu.getuploader.com:443/MMDkaz/DOWNLOAD/7/extra?token=file7#request7"
	requestWithoutFileURL := "https://ux.getuploader.com/MMDkaz/download/8/?token=abc/#request8"
	requestWithFile := model.PublicRequest{
		RequestType: "preservation", Status: "pending", FileObjectID: &file7.ID,
		SourceURL: &requestWithFileURL, Details: "file-linked request",
	}
	requestWithoutFile := model.PublicRequest{
		RequestType: "preservation", Status: "pending",
		SourceURL: &requestWithoutFileURL, Details: "URL-only request",
	}
	for _, request := range []*model.PublicRequest{&requestWithFile, &requestWithoutFile} {
		if err := db.Create(request).Error; err != nil {
			t.Fatalf("create request: %v", err)
		}
	}

	authorID := uuid.NewString()
	catalogExec(t, db, "INSERT INTO author_artifact (user_id, artifact_id) VALUES (?, ?), (?, ?)", authorID, oldest.ID, authorID, duplicate.ID)

	registryRows := []struct {
		normalized string
		source     string
		origin     string
	}{
		{"https://uu.getuploader.com/MMDkaz/index/date/desc/1#registry", "https://uu.getuploader.com/MMDkaz/index/date/desc/1#registry", "catalogued"},
		{"https://ux.getuploader.com:443/MMDkaz/page2#registry", "https://ux.getuploader.com:443/MMDkaz/page2#registry", "catalogued"},
		{"https://uu.getuploader.com:443/MMDkaz/DOWNLOAD/7/extra?token=file7#registry", "https://uu.getuploader.com:443/MMDkaz/DOWNLOAD/7/extra?token=file7#registry", "catalogued"},
		{"/MMDkaz/download/7", "/MMDkaz/download/7", "catalogued"},
		{"/MMDkaz/download/8/?token=abc/", "/MMDkaz/DOWNLOAD/8/?token=abc/", "catalogued"},
		{"https://example.uploader.jp/index/1", "https://example.uploader.jp/index/1", "public_request"},
		{"https://example.uploader.jp/index/2", "https://example.uploader.jp/index/2", "public_request"},
	}
	for _, row := range registryRows {
		catalogExec(t, db,
			"INSERT INTO source_url_registry (normalized_url, source_url, origin) VALUES (?, ?, ?)",
			row.normalized, row.source, row.origin,
		)
	}

	migration, err := os.ReadFile("../../atlas/migrations/20261118090000_normalize_getuploader_source_paths.sql")
	if err != nil {
		t.Fatalf("read GetUploader migration: %v", err)
	}
	migrationLines := strings.Split(string(migration), "\n")
	sqlLines := make([]string, 0, len(migrationLines))
	for _, line := range migrationLines {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		sqlLines = append(sqlLines, line)
	}
	migrationSQL := strings.Join(sqlLines, "\n")
	migrationSQL = strings.Replace(migrationSQL, "BEGIN;", "", 1)
	migrationSQL = strings.Replace(migrationSQL, "COMMIT;", "", 1)
	for i, statement := range splitSQLStatements(migrationSQL) {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("execute migration statement %d: %v\nSQL: %s", i+1, err, statement)
		}
	}

	var artifacts []model.Artifact
	if err := db.Order("created_at ASC").Find(&artifacts).Error; err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("artifacts=%d, want 2 after merging duplicate uploader roots", len(artifacts))
	}
	var canonical model.Artifact
	if err := db.First(&canonical, "id = ?", oldest.ID).Error; err != nil {
		t.Fatalf("oldest artifact should be preserved: %v", err)
	}
	if canonical.SourceURL != "/MMDkaz" {
		t.Fatalf("canonical artifact source_url=%q, want /MMDkaz", canonical.SourceURL)
	}

	for _, expected := range []struct {
		file model.FileObject
		want string
	}{
		{file7, "/MMDkaz/download/7?token=file7"},
		{file8, "/MMDkaz/download/8?token=abc/"},
	} {
		var got model.FileObject
		if err := db.First(&got, "id = ?", expected.file.ID).Error; err != nil {
			t.Fatal(err)
		}
		if got.ArtifactID != oldest.ID || got.SourceURL == nil || *got.SourceURL != expected.want {
			t.Fatalf("file %s: artifact_id=%s source_url=%v", got.ID, got.ArtifactID, got.SourceURL)
		}
	}

	for _, expected := range []model.PublicRequest{requestWithFile, requestWithoutFile} {
		var got model.PublicRequest
		if err := db.First(&got, "id = ?", expected.ID).Error; err != nil {
			t.Fatal(err)
		}
		wantSource := "/MMDkaz/download/7?token=file7"
		if expected.ID == requestWithoutFile.ID {
			wantSource = "/MMDkaz/download/8?token=abc/"
		}
		if got.SourceURL == nil || *got.SourceURL != wantSource {
			t.Errorf("request source_url=%v, want %q", got.SourceURL, wantSource)
		}
		if got.ArtifactID == nil || *got.ArtifactID != oldest.ID {
			t.Fatalf("request %s was not linked to canonical artifact: %+v", got.ID, got.ArtifactID)
		}
	}
	var authorLinkCount int64
	if err := db.Table("author_artifact").Where("user_id = ? AND artifact_id = ?", authorID, oldest.ID).Count(&authorLinkCount).Error; err != nil {
		t.Fatal(err)
	}
	if authorLinkCount != 1 {
		t.Fatalf("author-artifact link count=%d, want 1", authorLinkCount)
	}

	var registryCount int64
	if err := db.Table("source_url_registry").Count(&registryCount).Error; err != nil {
		t.Fatal(err)
	}
	if registryCount != 6 {
		t.Fatalf("registry rows=%d, want 6 (uploader root, two distinct file-7 keys, file 8, and two unrelated URLs)", registryCount)
	}
	for _, key := range []string{"/MMDkaz", "/MMDkaz/download/7", "/MMDkaz/download/7?token=file7", "/MMDkaz/download/8?token=abc/", "https://example.uploader.jp/index/1", "https://example.uploader.jp/index/2"} {
		var count int64
		if err := db.Table("source_url_registry").Where("normalized_url = ? AND source_url = ?", key, key).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("registry key %q count=%d, want 1", key, count)
		}
	}
}
:
			if end := strings.IndexByte(sql[i+1:], '(t *testing.T) {
	db, _ := catalogTestDB(t)

	catalogExec(t, db, `CREATE TABLE author_artifact (
		user_id uuid NOT NULL,
		artifact_id uuid NOT NULL,
		created_at timestamptz NOT NULL DEFAULT now(),
		PRIMARY KEY (artifact_id, user_id)
	)`)
	catalogExec(t, db, `CREATE TABLE source_url_registry (
		normalized_url text PRIMARY KEY,
		source_url text NOT NULL,
		origin text NOT NULL,
		public_request_id uuid,
		created_at timestamptz NOT NULL DEFAULT now(),
		updated_at timestamptz NOT NULL DEFAULT now()
	)`)

	oldest := model.Artifact{CreatedAt: time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC), Title: "MMDkaz oldest", SourceURL: "https://uu.getuploader.com/MMDkaz/index/date/desc/2#artifact"}
	duplicate := model.Artifact{CreatedAt: time.Date(2026, time.November, 2, 0, 0, 0, 0, time.UTC), Title: "MMDkaz duplicate", SourceURL: "https://ux.getuploader.com:443/MMDkaz/page2#duplicate"}
	unrelated := model.Artifact{Title: "unrelated uploader", SourceURL: "https://uploader.jp/account"}
	for _, artifact := range []*model.Artifact{&oldest, &duplicate, &unrelated} {
		if err := db.Create(artifact).Error; err != nil {
			t.Fatalf("create artifact: %v", err)
		}
	}

	file7URL := "https://uu.getuploader.com:443/MMDkaz/DOWNLOAD/7/extra?token=file7#file7"
	file8URL := "https://ux.getuploader.com/MMDkaz/download/8/?token=abc/#file8"
	file7 := model.FileObject{
		ArtifactID: oldest.ID, OriginalFilename: "seven.zip", SizeBytes: 7,
		PreservationStatus: "stored", RedistributionPolicy: "unknown", SourceURL: &file7URL,
	}
	file8 := model.FileObject{
		ArtifactID: duplicate.ID, OriginalFilename: "eight.zip", SizeBytes: 8,
		PreservationStatus: "stored", RedistributionPolicy: "unknown", SourceURL: &file8URL,
	}
	for _, file := range []*model.FileObject{&file7, &file8} {
		if err := db.Create(file).Error; err != nil {
			t.Fatalf("create file: %v", err)
		}
	}

	requestWithFileURL := "https://uu.getuploader.com:443/MMDkaz/DOWNLOAD/7/extra?token=file7#request7"
	requestWithoutFileURL := "https://ux.getuploader.com/MMDkaz/download/8/?token=abc/#request8"
	requestWithFile := model.PublicRequest{
		RequestType: "preservation", Status: "pending", FileObjectID: &file7.ID,
		SourceURL: &requestWithFileURL, Details: "file-linked request",
	}
	requestWithoutFile := model.PublicRequest{
		RequestType: "preservation", Status: "pending",
		SourceURL: &requestWithoutFileURL, Details: "URL-only request",
	}
	for _, request := range []*model.PublicRequest{&requestWithFile, &requestWithoutFile} {
		if err := db.Create(request).Error; err != nil {
			t.Fatalf("create request: %v", err)
		}
	}

	authorID := uuid.NewString()
	catalogExec(t, db, "INSERT INTO author_artifact (user_id, artifact_id) VALUES (?, ?), (?, ?)", authorID, oldest.ID, authorID, duplicate.ID)

	registryRows := []struct {
		normalized string
		source     string
		origin     string
	}{
		{"https://uu.getuploader.com/MMDkaz/index/date/desc/1#registry", "https://uu.getuploader.com/MMDkaz/index/date/desc/1#registry", "catalogued"},
		{"https://ux.getuploader.com:443/MMDkaz/page2#registry", "https://ux.getuploader.com:443/MMDkaz/page2#registry", "catalogued"},
		{"https://uu.getuploader.com:443/MMDkaz/DOWNLOAD/7/extra?token=file7#registry", "https://uu.getuploader.com:443/MMDkaz/DOWNLOAD/7/extra?token=file7#registry", "catalogued"},
		{"/MMDkaz/download/7", "/MMDkaz/download/7", "catalogued"},
		{"/MMDkaz/download/8/?token=abc/", "/MMDkaz/DOWNLOAD/8/?token=abc/", "catalogued"},
		{"https://example.uploader.jp/index/1", "https://example.uploader.jp/index/1", "public_request"},
		{"https://example.uploader.jp/index/2", "https://example.uploader.jp/index/2", "public_request"},
	}
	for _, row := range registryRows {
		catalogExec(t, db,
			"INSERT INTO source_url_registry (normalized_url, source_url, origin) VALUES (?, ?, ?)",
			row.normalized, row.source, row.origin,
		)
	}

	migration, err := os.ReadFile("../../atlas/migrations/20261118090000_normalize_getuploader_source_paths.sql")
	if err != nil {
		t.Fatalf("read GetUploader migration: %v", err)
	}
	migrationLines := strings.Split(string(migration), "\n")
	sqlLines := make([]string, 0, len(migrationLines))
	for _, line := range migrationLines {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		sqlLines = append(sqlLines, line)
	}
	migrationSQL := strings.Join(sqlLines, "\n")
	migrationSQL = strings.Replace(migrationSQL, "BEGIN;", "", 1)
	migrationSQL = strings.Replace(migrationSQL, "COMMIT;", "", 1)
	for i, statement := range splitSQLStatements(migrationSQL) {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("execute migration statement %d: %v\nSQL: %s", i+1, err, statement)
		}
	}

	var artifacts []model.Artifact
	if err := db.Order("created_at ASC").Find(&artifacts).Error; err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("artifacts=%d, want 2 after merging duplicate uploader roots", len(artifacts))
	}
	var canonical model.Artifact
	if err := db.First(&canonical, "id = ?", oldest.ID).Error; err != nil {
		t.Fatalf("oldest artifact should be preserved: %v", err)
	}
	if canonical.SourceURL != "/MMDkaz" {
		t.Fatalf("canonical artifact source_url=%q, want /MMDkaz", canonical.SourceURL)
	}

	for _, expected := range []struct {
		file model.FileObject
		want string
	}{
		{file7, "/MMDkaz/download/7?token=file7"},
		{file8, "/MMDkaz/download/8?token=abc/"},
	} {
		var got model.FileObject
		if err := db.First(&got, "id = ?", expected.file.ID).Error; err != nil {
			t.Fatal(err)
		}
		if got.ArtifactID != oldest.ID || got.SourceURL == nil || *got.SourceURL != expected.want {
			t.Fatalf("file %s: artifact_id=%s source_url=%v", got.ID, got.ArtifactID, got.SourceURL)
		}
	}

	for _, expected := range []model.PublicRequest{requestWithFile, requestWithoutFile} {
		var got model.PublicRequest
		if err := db.First(&got, "id = ?", expected.ID).Error; err != nil {
			t.Fatal(err)
		}
		wantSource := "/MMDkaz/download/7?token=file7"
		if expected.ID == requestWithoutFile.ID {
			wantSource = "/MMDkaz/download/8?token=abc/"
		}
		if got.SourceURL == nil || *got.SourceURL != wantSource {
			t.Errorf("request source_url=%v, want %q", got.SourceURL, wantSource)
		}
		if got.ArtifactID == nil || *got.ArtifactID != oldest.ID {
			t.Fatalf("request %s was not linked to canonical artifact: %+v", got.ID, got.ArtifactID)
		}
	}
	var authorLinkCount int64
	if err := db.Table("author_artifact").Where("user_id = ? AND artifact_id = ?", authorID, oldest.ID).Count(&authorLinkCount).Error; err != nil {
		t.Fatal(err)
	}
	if authorLinkCount != 1 {
		t.Fatalf("author-artifact link count=%d, want 1", authorLinkCount)
	}

	var registryCount int64
	if err := db.Table("source_url_registry").Count(&registryCount).Error; err != nil {
		t.Fatal(err)
	}
	if registryCount != 6 {
		t.Fatalf("registry rows=%d, want 6 (uploader root, two distinct file-7 keys, file 8, and two unrelated URLs)", registryCount)
	}
	for _, key := range []string{"/MMDkaz", "/MMDkaz/download/7", "/MMDkaz/download/7?token=file7", "/MMDkaz/download/8?token=abc/", "https://example.uploader.jp/index/1", "https://example.uploader.jp/index/2"} {
		var count int64
		if err := db.Table("source_url_registry").Where("normalized_url = ? AND source_url = ?", key, key).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("registry key %q count=%d, want 1", key, count)
		}
	}
}
); end >= 0 {
				tag := sql[i : i+end+2]
				if strings.Trim(tag, "$") == "" || strings.Trim(tag, "$") != "" {
					dollarTag = tag
					i += len(tag) - 1
				}
			}
		case ';':
			if statement := strings.TrimSpace(sql[start:i]); statement != "" {
				statements = append(statements, statement)
			}
			start = i + 1
		}
	}
	if statement := strings.TrimSpace(sql[start:]); statement != "" {
		statements = append(statements, statement)
	}
	return statements
}

func TestGetUploaderSourcePathMigrationIntegration(t *testing.T) {
	db, _ := catalogTestDB(t)

	catalogExec(t, db, `CREATE TABLE author_artifact (
		user_id uuid NOT NULL,
		artifact_id uuid NOT NULL,
		created_at timestamptz NOT NULL DEFAULT now(),
		PRIMARY KEY (artifact_id, user_id)
	)`)
	catalogExec(t, db, `CREATE TABLE source_url_registry (
		normalized_url text PRIMARY KEY,
		source_url text NOT NULL,
		origin text NOT NULL,
		public_request_id uuid,
		created_at timestamptz NOT NULL DEFAULT now(),
		updated_at timestamptz NOT NULL DEFAULT now()
	)`)

	oldest := model.Artifact{CreatedAt: time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC), Title: "MMDkaz oldest", SourceURL: "https://uu.getuploader.com/MMDkaz/index/date/desc/2#artifact"}
	duplicate := model.Artifact{CreatedAt: time.Date(2026, time.November, 2, 0, 0, 0, 0, time.UTC), Title: "MMDkaz duplicate", SourceURL: "https://ux.getuploader.com:443/MMDkaz/page2#duplicate"}
	unrelated := model.Artifact{Title: "unrelated uploader", SourceURL: "https://uploader.jp/account"}
	for _, artifact := range []*model.Artifact{&oldest, &duplicate, &unrelated} {
		if err := db.Create(artifact).Error; err != nil {
			t.Fatalf("create artifact: %v", err)
		}
	}

	file7URL := "https://uu.getuploader.com:443/MMDkaz/DOWNLOAD/7/extra?token=file7#file7"
	file8URL := "https://ux.getuploader.com/MMDkaz/download/8/?token=abc/#file8"
	file7 := model.FileObject{
		ArtifactID: oldest.ID, OriginalFilename: "seven.zip", SizeBytes: 7,
		PreservationStatus: "stored", RedistributionPolicy: "unknown", SourceURL: &file7URL,
	}
	file8 := model.FileObject{
		ArtifactID: duplicate.ID, OriginalFilename: "eight.zip", SizeBytes: 8,
		PreservationStatus: "stored", RedistributionPolicy: "unknown", SourceURL: &file8URL,
	}
	for _, file := range []*model.FileObject{&file7, &file8} {
		if err := db.Create(file).Error; err != nil {
			t.Fatalf("create file: %v", err)
		}
	}

	requestWithFileURL := "https://uu.getuploader.com:443/MMDkaz/DOWNLOAD/7/extra?token=file7#request7"
	requestWithoutFileURL := "https://ux.getuploader.com/MMDkaz/download/8/?token=abc/#request8"
	requestWithFile := model.PublicRequest{
		RequestType: "preservation", Status: "pending", FileObjectID: &file7.ID,
		SourceURL: &requestWithFileURL, Details: "file-linked request",
	}
	requestWithoutFile := model.PublicRequest{
		RequestType: "preservation", Status: "pending",
		SourceURL: &requestWithoutFileURL, Details: "URL-only request",
	}
	for _, request := range []*model.PublicRequest{&requestWithFile, &requestWithoutFile} {
		if err := db.Create(request).Error; err != nil {
			t.Fatalf("create request: %v", err)
		}
	}

	authorID := uuid.NewString()
	catalogExec(t, db, "INSERT INTO author_artifact (user_id, artifact_id) VALUES (?, ?), (?, ?)", authorID, oldest.ID, authorID, duplicate.ID)

	registryRows := []struct {
		normalized string
		source     string
		origin     string
	}{
		{"https://uu.getuploader.com/MMDkaz/index/date/desc/1#registry", "https://uu.getuploader.com/MMDkaz/index/date/desc/1#registry", "catalogued"},
		{"https://ux.getuploader.com:443/MMDkaz/page2#registry", "https://ux.getuploader.com:443/MMDkaz/page2#registry", "catalogued"},
		{"https://uu.getuploader.com:443/MMDkaz/DOWNLOAD/7/extra?token=file7#registry", "https://uu.getuploader.com:443/MMDkaz/DOWNLOAD/7/extra?token=file7#registry", "catalogued"},
		{"/MMDkaz/download/7", "/MMDkaz/download/7", "catalogued"},
		{"/MMDkaz/download/8/?token=abc/", "/MMDkaz/DOWNLOAD/8/?token=abc/", "catalogued"},
		{"https://example.uploader.jp/index/1", "https://example.uploader.jp/index/1", "public_request"},
		{"https://example.uploader.jp/index/2", "https://example.uploader.jp/index/2", "public_request"},
	}
	for _, row := range registryRows {
		catalogExec(t, db,
			"INSERT INTO source_url_registry (normalized_url, source_url, origin) VALUES (?, ?, ?)",
			row.normalized, row.source, row.origin,
		)
	}

	migration, err := os.ReadFile("../../atlas/migrations/20261118090000_normalize_getuploader_source_paths.sql")
	if err != nil {
		t.Fatalf("read GetUploader migration: %v", err)
	}
	migrationLines := strings.Split(string(migration), "\n")
	sqlLines := make([]string, 0, len(migrationLines))
	for _, line := range migrationLines {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		sqlLines = append(sqlLines, line)
	}
	migrationSQL := strings.Join(sqlLines, "\n")
	migrationSQL = strings.Replace(migrationSQL, "BEGIN;", "", 1)
	migrationSQL = strings.Replace(migrationSQL, "COMMIT;", "", 1)
	for i, statement := range splitSQLStatements(migrationSQL) {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("execute migration statement %d: %v\nSQL: %s", i+1, err, statement)
		}
	}

	var artifacts []model.Artifact
	if err := db.Order("created_at ASC").Find(&artifacts).Error; err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("artifacts=%d, want 2 after merging duplicate uploader roots", len(artifacts))
	}
	var canonical model.Artifact
	if err := db.First(&canonical, "id = ?", oldest.ID).Error; err != nil {
		t.Fatalf("oldest artifact should be preserved: %v", err)
	}
	if canonical.SourceURL != "/MMDkaz" {
		t.Fatalf("canonical artifact source_url=%q, want /MMDkaz", canonical.SourceURL)
	}

	for _, expected := range []struct {
		file model.FileObject
		want string
	}{
		{file7, "/MMDkaz/download/7?token=file7"},
		{file8, "/MMDkaz/download/8?token=abc/"},
	} {
		var got model.FileObject
		if err := db.First(&got, "id = ?", expected.file.ID).Error; err != nil {
			t.Fatal(err)
		}
		if got.ArtifactID != oldest.ID || got.SourceURL == nil || *got.SourceURL != expected.want {
			t.Fatalf("file %s: artifact_id=%s source_url=%v", got.ID, got.ArtifactID, got.SourceURL)
		}
	}

	for _, expected := range []model.PublicRequest{requestWithFile, requestWithoutFile} {
		var got model.PublicRequest
		if err := db.First(&got, "id = ?", expected.ID).Error; err != nil {
			t.Fatal(err)
		}
		wantSource := "/MMDkaz/download/7?token=file7"
		if expected.ID == requestWithoutFile.ID {
			wantSource = "/MMDkaz/download/8?token=abc/"
		}
		if got.SourceURL == nil || *got.SourceURL != wantSource {
			t.Errorf("request source_url=%v, want %q", got.SourceURL, wantSource)
		}
		if got.ArtifactID == nil || *got.ArtifactID != oldest.ID {
			t.Fatalf("request %s was not linked to canonical artifact: %+v", got.ID, got.ArtifactID)
		}
	}
	var authorLinkCount int64
	if err := db.Table("author_artifact").Where("user_id = ? AND artifact_id = ?", authorID, oldest.ID).Count(&authorLinkCount).Error; err != nil {
		t.Fatal(err)
	}
	if authorLinkCount != 1 {
		t.Fatalf("author-artifact link count=%d, want 1", authorLinkCount)
	}

	var registryCount int64
	if err := db.Table("source_url_registry").Count(&registryCount).Error; err != nil {
		t.Fatal(err)
	}
	if registryCount != 6 {
		t.Fatalf("registry rows=%d, want 6 (uploader root, two distinct file-7 keys, file 8, and two unrelated URLs)", registryCount)
	}
	for _, key := range []string{"/MMDkaz", "/MMDkaz/download/7", "/MMDkaz/download/7?token=file7", "/MMDkaz/download/8?token=abc/", "https://example.uploader.jp/index/1", "https://example.uploader.jp/index/2"} {
		var count int64
		if err := db.Table("source_url_registry").Where("normalized_url = ? AND source_url = ?", key, key).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("registry key %q count=%d, want 1", key, count)
		}
	}
}
