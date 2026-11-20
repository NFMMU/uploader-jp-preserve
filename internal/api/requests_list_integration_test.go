package api

import (
	"encoding/json"
	"net/http"
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

	status = catalogResponse(t, server.listPublicRequests, "/?request_type=unknown", &struct{}{})
	if status != http.StatusBadRequest {
		t.Fatalf("invalid request_type status = %d, want %d", status, http.StatusBadRequest)
	}
	status = catalogResponse(t, server.listPublicRequests, "/?sort=sideways", &struct{}{})
	if status != http.StatusBadRequest {
		t.Fatalf("invalid sort status = %d, want %d", status, http.StatusBadRequest)
	}

	// Confirm the handler returns a valid JSON payload for the filtered list.
	var raw map[string]any
	body := catalogResponseBody(t, server.listPublicRequests, "/?q=archive")
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode response JSON: %v", err)
	}
}
