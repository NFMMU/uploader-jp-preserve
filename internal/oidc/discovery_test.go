package oidc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDiscoveryCancellationDuringBody(t *testing.T) {
	for _, cancelEarly := range []bool{false, true} {
		t.Run(map[bool]string{false: "deadline", true: "cancel"}[cancelEarly], func(t *testing.T) {
			released := make(chan struct{})
			started := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"issuer":`))
				w.(http.Flusher).Flush()
				close(started)
				<-r.Context().Done()
				close(released)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if cancelEarly {
				go func() { <-started; cancel() }()
			}
			_, err := OidcDiscovery(ctx, server.URL)
			expected := context.DeadlineExceeded
			if cancelEarly {
				expected = context.Canceled
			}
			if !errors.Is(err, expected) {
				t.Fatalf("expected %v, got %v", expected, err)
			}
			select {
			case <-released:
			case <-time.After(time.Second):
				t.Fatal("connection was not released")
			}
		})
	}
}

func TestDiscoveryHasDefaultDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"issuer":`))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	started := time.Now()
	_, err := OidcDiscovery(context.Background(), server.URL)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
	if time.Since(started) > 12*time.Second {
		t.Fatal("discovery exceeded deadline")
	}
}

func TestDiscoveryStatus(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusBadRequest, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				w.Write([]byte(`{"issuer":"https://issuer.example"}`))
			}))
			defer server.Close()
			data, err := OidcDiscovery(context.Background(), server.URL)
			if status >= 200 && status < 300 {
				if err != nil || data == nil || data.Issuer != "https://issuer.example" {
					t.Fatalf("unexpected discovery: %v, %v", data, err)
				}
			} else if err == nil || data != nil {
				t.Fatalf("expected rejection for HTTP %d", status)
			}
		})
	}
}
