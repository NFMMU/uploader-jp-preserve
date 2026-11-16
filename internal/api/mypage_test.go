package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
)

// A nil database makes these tests also catch accidental database access before
// validation or authorization has completed.
func mypageRequest(t *testing.T, method, route, path, body string, actor *model.User, handlers ...gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if actor != nil {
			c.Set("currentUser", *actor)
		}
		c.Next()
	})
	r.Handle(method, route, handlers...)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	return rec
}

func mypageError(t *testing.T, rec *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if rec.Code != status {
		t.Fatalf("status=%d, want %d: %s", rec.Code, status, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != message {
		t.Fatalf("error=%q, want %q", body.Error, message)
	}
}

func TestMypageAccountValidation(t *testing.T) {
	s := &Server{}
	actor := &model.User{ID: "actor", Role: "author"}
	for _, body := range []string{`{`, `{}`, `{"display_name":null}`, `{"display_name":12}`, `{"display_name":""}`, `{"display_name":" \t\n "}`, `{"display_name":"` + strings.Repeat("名", 121) + `"}`} {
		t.Run(body, func(t *testing.T) {
			rec := mypageRequest(t, http.MethodPut, "/", "/", body, actor, s.updateOwnAccount)
			mypageError(t, rec, http.StatusBadRequest, "display_name is required")
		})
	}
}

func TestMypageAccountRequiresAuthentication(t *testing.T) {
	s := &Server{}
	for name, handler := range map[string]gin.HandlerFunc{
		"profile": s.updateOwnAccount, "password": s.updateOwnPassword,
		"identities": s.listOwnUserOidcIdentities, "unlink": s.deleteOwnUserOidcIdentity,
	} {
		t.Run(name, func(t *testing.T) {
			rec := mypageRequest(t, http.MethodPut, "/", "/", `{"display_name":"valid"}`, nil, handler)
			mypageError(t, rec, http.StatusUnauthorized, "authentication required")
		})
	}
}

func TestMypageAssignmentValidation(t *testing.T) {
	s := &Server{}
	const id = "11111111-1111-4111-8111-111111111111"
	cases := []struct{ name, userID, body, message string }{
		{"malformed", id, `{`, "artifact_ids must be an array of UUIDs"},
		{"missing", id, `{}`, "artifact_ids must be an array of UUIDs"},
		{"null", id, `{"artifact_ids":null}`, "artifact_ids must be an array of UUIDs"},
		{"scalar", id, `{"artifact_ids":"` + id + `"}`, "artifact_ids must be an array of UUIDs"},
		{"invalid element", id, `{"artifact_ids":["invalid"]}`, "artifact_ids must be an array of UUIDs"},
		{"null element", id, `{"artifact_ids":[null]}`, "artifact_ids must be an array of UUIDs"},
		{"too many", id, `{"artifact_ids":[` + strings.TrimSuffix(strings.Repeat(`"`+id+`",`, 1001), ",") + `]}`, "artifact_ids must be an array of UUIDs"},
		{"invalid user", "invalid", `{"artifact_ids":[]}`, "invalid user_id"},
		{"duplicates", id, `{"artifact_ids":["` + id + `","` + id + `"]}`, "artifact_ids must be unique"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := mypageRequest(t, http.MethodPut, "/:userID", "/"+tc.userID, tc.body, nil, s.replaceAuthorArtifactAssignments)
			mypageError(t, rec, http.StatusBadRequest, tc.message)
		})
	}
}

func TestMypageArtifactMiddlewareAuthenticationAndRoles(t *testing.T) {
	s := &Server{}
	for _, role := range []string{"", "admin", "reviewer", "viewer"} {
		t.Run(role, func(t *testing.T) {
			var actor *model.User
			if role != "" {
				actor = &model.User{ID: "actor", Role: role}
			}
			called := false
			rec := mypageRequest(t, http.MethodGet, "/:id", "/artifact", "", actor, s.requireAuthorArtifactAccess(), func(c *gin.Context) { called = true; c.Status(http.StatusNoContent) })
			if role == "" {
				mypageError(t, rec, http.StatusUnauthorized, "authentication required")
				if called {
					t.Fatal("unauthenticated request reached handler")
				}
				return
			}
			if rec.Code != http.StatusNoContent || !called {
				t.Fatalf("role %s did not reach handler: %d", role, rec.Code)
			}
		})
	}
}

func TestMypageFileDeletionRejectsUnauthorizedRoles(t *testing.T) {
	s := &Server{}
	for _, role := range []string{"", "viewer", "reviewer"} {
		t.Run(role, func(t *testing.T) {
			var actor *model.User
			if role != "" {
				actor = &model.User{ID: "actor", Role: role}
			}
			rec := mypageRequest(t, http.MethodDelete, "/:id", "/file", "", actor, s.deleteFileObject)
			if role == "" {
				mypageError(t, rec, http.StatusUnauthorized, "authentication required")
			} else {
				mypageError(t, rec, http.StatusForbidden, "insufficient role")
			}
		})
	}
}

func TestMypageBulkActionRejectsRemovedDeleteAction(t *testing.T) {
	s := &Server{}
	rec := mypageRequest(t, http.MethodPost, "/", "/", `{"file_ids":["11111111-1111-4111-8111-111111111111"],"action":"delete","reason":"approved deletion"}`, &model.User{Role: "admin"}, s.bulkFileAction)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("removed action accepted: %d %s", rec.Code, rec.Body.String())
	}
}

func TestMypageOwnPasswordUsesAuthenticatedUser(t *testing.T) {
	s := &Server{}
	for _, role := range []string{"admin", "reviewer", "viewer", "author"} {
		t.Run(role, func(t *testing.T) {
			// A valid new password without the current password must take the self-change
			// validation path, including for administrators.
			rec := mypageRequest(t, http.MethodPut, "/", "/", `{"new_password":"new-password-123","confirm_password":"new-password-123","id":"another-user"}`, &model.User{ID: "actor", Role: role}, s.updateOwnPassword)
			mypageError(t, rec, http.StatusBadRequest, "current_password is required")
		})
	}
}
