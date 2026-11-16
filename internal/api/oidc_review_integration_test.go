package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/nfmmu/uploader-archive/internal/config"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"github.com/nfmmu/uploader-archive/internal/gen/query"
)

func TestUpdateUserRoleAndAuthorAssignmentsAreAtomic(t *testing.T) {
	db, _ := catalogTestDB(t)
	if err := db.AutoMigrate(&model.User{}, &model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE author_artifact (
		user_id uuid NOT NULL,
		artifact_id uuid NOT NULL,
		PRIMARY KEY (user_id, artifact_id)
	)`).Error; err != nil {
		t.Fatal(err)
	}

	actor := model.User{ID: uuid.NewString(), Email: "admin@example.test", DisplayName: "admin", PasswordHash: "unused", Role: "admin"}
	target := model.User{ID: uuid.NewString(), Email: "author@example.test", DisplayName: "before", PasswordHash: "unused", Role: "reviewer"}
	if err := db.Create(&actor).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}

	artifact := model.Artifact{ID: uuid.NewString(), Title: "assigned", SourceURL: "https://example.test/source"}
	if err := db.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}

	s := &Server{db: db, logger: slog.Default()}
	router := gin.New()
	router.PUT("/:id", func(c *gin.Context) {
		c.Set("currentUser", actor)
		s.updateUser(c)
	})

	responseWithoutAssignments := httptest.NewRecorder()
	router.ServeHTTP(responseWithoutAssignments, httptest.NewRequest(http.MethodPut, "/"+target.ID, strings.NewReader(`{"display_name":"promotion","role":"author","disabled":false}`)))
	if responseWithoutAssignments.Code != http.StatusBadRequest {
		t.Fatalf("promotion without assignments should fail: %d %s", responseWithoutAssignments.Code, responseWithoutAssignments.Body.String())
	}

	missingArtifactID := uuid.NewString()
	body := `{"display_name":"after","role":"author","disabled":false,"artifact_ids":["` + missingArtifactID + `"]}`
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/"+target.ID, strings.NewReader(body)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid assignment to fail: %d %s", response.Code, response.Body.String())
	}
	var stored model.User
	if err := db.First(&stored, "id = ?", target.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Role != "reviewer" || stored.DisplayName != "before" {
		t.Fatalf("user update was not rolled back: role=%s name=%s", stored.Role, stored.DisplayName)
	}

	const callbackName = "test:fail-user-update-after-assignment-write"
	if err := db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			tx.AddError(errors.New("forced update failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	body = `{"display_name":"after","role":"author","disabled":false,"artifact_ids":["` + artifact.ID + `"]}`
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/"+target.ID, strings.NewReader(body)))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected forced update failure: %d %s", response.Code, response.Body.String())
	}
	db.Callback().Update().Remove(callbackName)
	if err := db.First(&stored, "id = ?", target.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Role != "reviewer" || stored.DisplayName != "before" {
		t.Fatalf("role update escaped transaction rollback: role=%s name=%s", stored.Role, stored.DisplayName)
	}
	var rolledBackAssignments int64
	if err := db.Table("author_artifact").Where("user_id = ?", target.ID).Count(&rolledBackAssignments).Error; err != nil {
		t.Fatal(err)
	}
	if rolledBackAssignments != 0 {
		t.Fatalf("assignment write escaped transaction rollback: %d rows", rolledBackAssignments)
	}

	body = `{"display_name":"after","role":"author","disabled":false,"artifact_ids":["` + artifact.ID + `"]}`
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/"+target.ID, strings.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("combined update failed: %d %s", response.Code, response.Body.String())
	}
	if err := db.First(&stored, "id = ?", target.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Role != "author" || stored.DisplayName != "after" {
		t.Fatalf("user fields were not updated: role=%s name=%s", stored.Role, stored.DisplayName)
	}
	var assignedIDs []string
	if err := db.Table("author_artifact").Where("user_id = ?", target.ID).Pluck("artifact_id", &assignedIDs).Error; err != nil {
		t.Fatal(err)
	}
	if len(assignedIDs) != 1 || assignedIDs[0] != artifact.ID {
		t.Fatalf("unexpected author assignments: %v", assignedIDs)
	}
	for index, body := range []string{
		`{"display_name":"renamed","role":"author","disabled":false}`,
		`{"display_name":"renamed again"}`,
	} {
		if index == 1 {
			actor = stored // Self updates may omit role and disabled.
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/"+target.ID, strings.NewReader(body)))
		if response.Code != http.StatusOK {
			t.Fatalf("existing author update failed: %d %s", response.Code, response.Body.String())
		}
		assignedIDs = nil
		if err := db.Table("author_artifact").Where("user_id = ?", target.ID).Pluck("artifact_id", &assignedIDs).Error; err != nil {
			t.Fatal(err)
		}
		if len(assignedIDs) != 1 || assignedIDs[0] != artifact.ID {
			t.Fatalf("existing assignments changed: %v", assignedIDs)
		}
	}

}

func TestOidcProviderUpdatePreservesIdentityAndSecret(t *testing.T) {
	db, _ := catalogTestDB(t)
	if err := db.AutoMigrate(&model.OidcProvider{}, &model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	query.SetDefault(db)
	s := &Server{db: db, logger: slog.Default(), cfg: config.Config{FrontendURL: "http://canonical.example:8080", SourcePasswordEncryptionKey: bytes.Repeat([]byte{0x42}, 32)}}
	id, other := uuid.NewString(), uuid.NewString()
	for _, row := range []model.OidcProvider{
		{ID: id, DisplayName: "original", DiscoveryEndpoint: "https://issuer.example", ClientID: "client", ClientSecret: "keep-secret"},
		{ID: other, DisplayName: "other", DiscoveryEndpoint: "https://issuer.example", ClientID: "other", ClientSecret: "other-secret"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	router.PUT("/:id", func(c *gin.Context) { c.Set("currentUser", model.User{ID: uuid.NewString()}); s.putOidcProvider(c) })
	router.GET("/", s.listOidcProviders)
	for _, secret := range []string{"", `,"client_secret":"replacement"`} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/"+id, strings.NewReader(`{"display_name":"updated","discovery_endpoint":"https://issuer.example","client_id":"client"`+secret+`}`)))
		if rec.Code != 200 {
			t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "keep-secret") || strings.Contains(rec.Body.String(), "replacement") {
			t.Fatal("secret exposed")
		}
		var row model.OidcProvider
		if err := db.First(&row, "id = ?", id).Error; err != nil {
			t.Fatal(err)
		}
		expected := "keep-secret"
		if secret != "" {
			expected = "replacement"
		}
		stored := row.ClientSecret
		if secret != "" {
			var err error
			stored, err = s.decryptSourcePassword(&row.ClientSecret)
			if err != nil || row.ClientSecret == expected {
				t.Fatal("secret was not encrypted")
			}
		}
		if stored != expected || row.DisplayName != "updated" {
			t.Fatalf("unexpected provider: %s", row.ID)
		}
	}
	var count int64
	db.Model(&model.OidcProvider{}).Count(&count)
	if count != 2 {
		t.Fatalf("provider count %d", count)
	}
	var untouched model.OidcProvider
	db.First(&untouched, "id = ?", other)
	if untouched.DisplayName != "other" || untouched.ClientSecret != "other-secret" {
		t.Fatal("other provider changed")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	var result struct {
		Items []struct {
			ID          string `json:"id"`
			CallbackURL string `json:"callback_url"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("list: %s", rec.Body.String())
	}
	for _, item := range result.Items {
		if item.CallbackURL != "http://canonical.example:8080/api/v1/auth/oidc/callback/"+item.ID {
			t.Fatal("incorrect canonical callback")
		}
	}
}

func TestSelfPasswordChangeRejectsConcurrentReset(t *testing.T) {
	db, _ := catalogTestDB(t)
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("original-password"), bcrypt.MinCost)
	resetHash, _ := bcrypt.GenerateFromPassword([]byte("reset-password"), bcrypt.MinCost)
	user := model.User{ID: uuid.NewString(), Email: "local@example.invalid", DisplayName: "test", PasswordHash: string(hash), Role: "admin"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	const callback = "test:concurrent-reset"
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			tx.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(resetHash), user.ID)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Update().Remove(callback) })
	s := &Server{db: db}
	router := gin.New()
	router.PUT("/:id", func(c *gin.Context) { c.Set("currentUser", user); s.updatePassword(c) })
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/"+user.ID, strings.NewReader(`{"current_password":"original-password","new_password":"new-password-123","confirm_password":"new-password-123"}`)))
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "current password") {
		t.Fatalf("expected conflict: %d %s", rec.Code, rec.Body.String())
	}
	var current model.User
	db.First(&current, "id = ?", user.ID)
	if current.PasswordHash != string(resetHash) {
		t.Fatal("concurrent password reset overwritten")
	}
}

func TestSelfProfileUpdatePreservesAdministrativeFields(t *testing.T) {
	db, _ := catalogTestDB(t)
	if err := db.AutoMigrate(&model.User{}, &model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db, logger: slog.Default()}
	for _, role := range []string{"admin", "reviewer", "viewer"} {
		user := model.User{ID: uuid.NewString(), Email: role + "@example.test", DisplayName: "before", Role: role, PasswordHash: "unused"}
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		router := gin.New()
		router.PUT("/:id", func(c *gin.Context) { c.Set("currentUser", user); s.updateUser(c) })
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/"+user.ID, strings.NewReader(`{"display_name":"after"}`)))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s profile update: %d %s", role, rec.Code, rec.Body.String())
		}
		var stored model.User
		if err := db.First(&stored, "id = ?", user.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.DisplayName != "after" || stored.Role != role || stored.DisabledAt != nil {
			t.Fatal("profile update changed administrative fields")
		}
	}
}

func TestOidcProviderCreateEncryptsSecretAndHidesIt(t *testing.T) {
	db, _ := catalogTestDB(t)
	if err := db.AutoMigrate(&model.OidcProvider{}, &model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	query.SetDefault(db)
	s := &Server{db: db, logger: slog.Default(), cfg: config.Config{SourcePasswordEncryptionKey: bytes.Repeat([]byte{0x42}, 32)}}
	router := gin.New()
	router.POST("/", func(c *gin.Context) { c.Set("currentUser", model.User{ID: uuid.NewString()}); s.createOidcProvider(c) })
	router.GET("/:id", s.getOidcProvider)
	router.GET("/", s.listOidcProviders)
	secret := strings.Repeat("s", 255)
	body, _ := json.Marshal(map[string]string{"display_name": "test", "discovery_endpoint": "https://issuer.example", "client_id": "test", "client_secret": secret})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(body))))
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var row model.OidcProvider
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	plain, err := s.decryptSourcePassword(&row.ClientSecret)
	if err != nil || plain != secret || row.ClientSecret == secret {
		t.Fatal("stored secret did not round trip through encryption")
	}
	for _, path := range []string{"/", "/" + row.ID} {
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "client_secret") || strings.Contains(rec.Body.String(), row.ClientSecret) {
			t.Fatalf("unsafe provider response: %d", rec.Code)
		}
	}
}

func TestOidcMissingProviderResponses(t *testing.T) {
	db, _ := catalogTestDB(t)
	if err := db.AutoMigrate(&model.OidcProvider{}); err != nil {
		t.Fatal(err)
	}
	query.SetDefault(db)
	s := &Server{db: db, logger: slog.Default()}
	router := gin.New()
	router.GET("/:id", s.getOidcProvider)
	router.DELETE("/:id", s.deleteOidcProvider)
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, "/"+uuid.NewString(), nil))
		if rec.Code != http.StatusNotFound || rec.Body.String() != `{"error":"provider not found"}` {
			t.Fatalf("%s: %d %s", method, rec.Code, rec.Body.String())
		}
	}
}
