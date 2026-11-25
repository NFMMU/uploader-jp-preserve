package api

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
)

func TestListPublicRequestsSearchTypeAndSortIntegration(t *testing.T) {
	db, _ := catalogTestDB(t)
	gin.SetMode(gin.TestMode)

	createdAt := func(day int) time.Time {
		return time.Date(2026, time.November, day, 12, 0, 0, 0, time.UTC)
	}
	title := func(value string) *string { return &value }
	requests := []model.PublicRequest{
		{
			RequestType: "preservation",
			Status:      "pending",
			Title:       title("Night Archive"),
			Details:     "preserve this item",
			CreatedAt:   createdAt(3),
		},
		{
			RequestType: "deletion",
			Status:      "pending",
			Title:       title("Archive removal"),
			Details:     "remove this item",
			CreatedAt:   createdAt(2),
		},
		{
			RequestType: "preservation",
			Status:      "pending",
			Title:       title("Archive mirror"),
			Details:     "preserve this mirror",
			CreatedAt:   createdAt(1),
		},
		// Control: same status/type as the requests above but matches no search term.
		{
			RequestType: "preservation",
			Status:      "pending",
			Title:       title("Garden shed"),
			Details:     "keep this item",
			CreatedAt:   createdAt(4),
		},
		// Non-ASCII title used to prove case-insensitive matching on both sides.
		{
			RequestType: "takedown",
			Status:      "pending",
			Title:       title("Äpp"),
			Details:     "non-ascii probe",
			CreatedAt:   createdAt(5),
		},
	}
	for i := range requests {
		if err := db.Create(&requests[i]).Error; err != nil {
			t.Fatalf("create request %d: %v", i, err)
		}
	}

	server := &Server{db: db}
	var response struct {
		Items []publicRequestListItem `json:"items"`
		Total int64                   `json:"total"`
	}
	status := catalogResponse(
		t,
		server.listPublicRequests,
		"/?status=pending&request_type=preservation&q=archive&sort=asc&page=1&limit=20",
		&response,
	)
	if status != http.StatusOK {
		t.Fatalf("listPublicRequests status = %d, want %d", status, http.StatusOK)
	}
	if response.Total != 2 || len(response.Items) != 2 {
		t.Fatalf("filtered total/items = %d/%d, want 2/2", response.Total, len(response.Items))
	}
	if response.Items[0].ID != requests[2].ID || response.Items[1].ID != requests[0].ID {
		t.Fatalf("ascending created_at order = [%s, %s], want [%s, %s]",
			response.Items[0].ID, response.Items[1].ID, requests[2].ID, requests[0].ID)
	}

	response = struct {
		Items []publicRequestListItem `json:"items"`
		Total int64                   `json:"total"`
	}{}
	status = catalogResponse(
		t,
		server.listPublicRequests,
		"/?request_type=deletion&q=archive&sort=desc&page=1&limit=20",
		&response,
	)
	if status != http.StatusOK {
		t.Fatalf("filtered list status = %d, want %d", status, http.StatusOK)
	}
	if response.Total != 1 || len(response.Items) != 1 || response.Items[0].ID != requests[1].ID {
		t.Fatalf("deletion filter returned total/items = %d/%d", response.Total, len(response.Items))
	}
	for _, item := range response.Items {
		if item.ID == requests[3].ID {
			t.Fatalf("filtered list returned control request %s", requests[3].ID)
		}
	}

	// Regression: all=true combined with q used to fail with ambiguous columns (HTTP 500).
	response = struct {
		Items []publicRequestListItem `json:"items"`
		Total int64                   `json:"total"`
	}{}
	status = catalogResponse(
		t,
		server.listPublicRequests,
		"/?all=true&q=archive&sort=asc&page=1&limit=20",
		&response,
	)
	if status != http.StatusOK {
		t.Fatalf("all-mode search status = %d, want %d", status, http.StatusOK)
	}
	if response.Total != 3 || len(response.Items) != 3 {
		t.Fatalf("all-mode search total/items = %d/%d, want 3/3", response.Total, len(response.Items))
	}
	wantOrder := []string{requests[2].ID, requests[1].ID, requests[0].ID}
	for i, want := range wantOrder {
		if response.Items[i].ID != want {
			t.Fatalf("all-mode search order[%d] = %s, want %s", i, response.Items[i].ID, want)
		}
	}
	for _, item := range response.Items {
		if item.ID == requests[3].ID || item.ID == requests[4].ID {
			t.Fatalf("all-mode search returned unmatched request %s", item.ID)
		}
	}

	// Non-ASCII title must match a differently-cased search term.
	status = catalogResponse(
		t,
		server.listPublicRequests,
		"/?all=true&q=ÄPP&sort=asc&page=1&limit=20",
		&response,
	)
	if status != http.StatusOK {
		t.Fatalf("non-ascii search status = %d, want %d", status, http.StatusOK)
	}
	if response.Total != 1 || len(response.Items) != 1 || response.Items[0].ID != requests[4].ID {
		t.Fatalf("non-ascii search returned total/items = %d/%d, want 1/1 with request %s",
			response.Total, len(response.Items), requests[4].ID)
	}

	status = catalogResponse(t, server.listPublicRequests, "/?request_type=unknown", &struct{}{})
	if status != http.StatusBadRequest {
		t.Fatalf("invalid request_type status = %d, want %d", status, http.StatusBadRequest)
	}
	status = catalogResponse(t, server.listPublicRequests, "/?sort=sideways", &struct{}{})
	if status != http.StatusBadRequest {
		t.Fatalf("invalid sort status = %d, want %d", status, http.StatusBadRequest)
	}

}

func TestListPublicRequestsSearchSourceURLIntegration(t *testing.T) {
	db, _ := catalogTestDB(t)
	gin.SetMode(gin.TestMode)

	relative := "/mirror/download/42"
	absolute := "https://uploader.jp/mirror"
	requests := []model.PublicRequest{
		{RequestType: "preservation", Status: "pending", SourceURL: &relative},
		{RequestType: "preservation", Status: "pending", SourceURL: &absolute},
	}
	for i := range requests {
		if err := db.Create(&requests[i]).Error; err != nil {
			t.Fatalf("create request %d: %v", i, err)
		}
	}

	server := &Server{db: db}
	for _, tc := range []struct {
		query string
		want  int64
	}{
		{"uu.getuploader.com", 1},
		{"https://ux.getuploader.com/", 1},
		{"MIRROR", 2},
	} {
		t.Run(tc.query, func(t *testing.T) {
			var response struct {
				Items []publicRequestListItem
				Total int64
			}
			if status := catalogResponse(t, server.listPublicRequests, "/?q="+url.QueryEscape(tc.query), &response); status != http.StatusOK {
				t.Fatalf("listPublicRequests status = %d, want %d", status, http.StatusOK)
			}
			if response.Total != tc.want || int64(len(response.Items)) != tc.want {
				t.Fatalf("total/items = %d/%d, want %d/%d", response.Total, len(response.Items), tc.want, tc.want)
			}
			if tc.want == 1 && response.Items[0].ID != requests[0].ID {
				t.Fatalf("matched request %s, want %s", response.Items[0].ID, requests[0].ID)
			}
		})
	}
}
