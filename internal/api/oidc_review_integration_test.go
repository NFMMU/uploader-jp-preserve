package api

import (
	"bytes"
	"encoding/json"
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
