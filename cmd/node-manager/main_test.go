package main

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseConfigRequiresHTTPSForNonLoopbackByDefault(t *testing.T) {
	values := url.Values{
		"central_api_url":  {"https://archive.example.org"},
		"enrollment_token": {"test-token"},
		"name":             {"test-node"},
		"capacity_bytes":   {"10737418240"},
	}
	req := httptest.NewRequest("POST", "/", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseConfig(req, false)
	if err != nil {
		t.Fatalf("valid HTTPS config rejected: %v", err)
	}
	if cfg.Name != "test-node" || cfg.CapacityBytes != 10737418240 {
		t.Fatalf("unexpected config: %+v", cfg)
	}

	values.Set("central_api_url", "http://archive.example.org")
	req = httptest.NewRequest("POST", "/", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}
	if _, err := parseConfig(req, false); err == nil {
		t.Fatal("non-loopback HTTP URL should be rejected by default")
	}
	if _, err := parseConfig(req, true); err != nil {
		t.Fatalf("development override should permit HTTP: %v", err)
	}
}

func TestParseConfigRejectsNegativeCapacityAndUnsafeName(t *testing.T) {
	values := url.Values{
		"central_api_url":  {"https://archive.example.org"},
		"enrollment_token": {"test-token"},
		"name":             {"test-node"},
		"capacity_bytes":   {"-1"},
	}
	req := httptest.NewRequest("POST", "/", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}
	if _, err := parseConfig(req, false); err == nil {
		t.Fatal("negative capacity should be rejected")
	}

	values.Set("capacity_bytes", "0")
	values.Set("name", "../unsafe")
	req = httptest.NewRequest("POST", "/", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}
	if _, err := parseConfig(req, false); err == nil {
		t.Fatal("unsafe node name should be rejected")
	}
}

func TestIsLocalHost(t *testing.T) {
	for _, host := range []string{"localhost:8780", "127.0.0.1:8780", "[::1]:8780", "localhost"} {
		if !isLocalHost(host) {
			t.Errorf("expected local host %q to pass", host)
		}
	}
	for _, host := range []string{"example.org:8780", "192.0.2.10:8780"} {
		if isLocalHost(host) {
			t.Errorf("expected non-local host %q to fail", host)
		}
	}
}

func TestOriginMatchesLocalHost(t *testing.T) {
	tests := []struct {
		name        string
		origin      string
		requestHost string
		want        bool
	}{
		{"same localhost", "http://localhost:8780", "localhost:8780", true},
		{"localhost alias", "http://localhost:8780", "127.0.0.1:8780", true},
		{"ipv4 alias", "http://127.0.0.1:8780", "localhost:8780", true},
		{"different port", "http://localhost:8781", "localhost:8780", false},
		{"remote origin", "http://example.org:8780", "localhost:8780", false},
		{"remote request host", "http://localhost:8780", "example.org:8780", false},
		{"invalid scheme", "javascript://localhost:8780", "localhost:8780", false},
		{"origin path", "http://localhost:8780/evil", "localhost:8780", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := originMatchesLocalHost(tt.origin, tt.requestHost); got != tt.want {
				t.Fatalf("originMatchesLocalHost(%q, %q) = %v, want %v", tt.origin, tt.requestHost, got, tt.want)
			}
		})
	}
}

func TestOriginHeaderAllowed(t *testing.T) {
	tests := []struct {
		name        string
		origin      string
		requestHost string
		want        bool
	}{
		{"opaque origin uses CSRF validation", "null", "localhost:8780", true},
		{"missing origin uses CSRF validation", "", "localhost:8780", true},
		{"local origin", "http://localhost:8780", "localhost:8780", true},
		{"remote origin rejected", "https://example.org", "localhost:8780", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := originHeaderAllowed(tt.origin, tt.requestHost); got != tt.want {
				t.Fatalf("originHeaderAllowed(%q, %q) = %v, want %v", tt.origin, tt.requestHost, got, tt.want)
			}
		})
	}
}
