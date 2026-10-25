package api

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNormalizeUploaderURL(t *testing.T) {
	got, err := normalizeUploaderURL(" HTTPS://UU.GETUPLOADER.COM:443/example/download/12?token=abc#section ")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://uu.getuploader.com/example/download/12?token=abc"
	if got != want {
		t.Fatalf("normalizeUploaderURL() = %q, want %q", got, want)
	}
}

func TestCheckUploaderCandidateStatus(t *testing.T) {
	for _, tc := range []struct {
		name       string
		statusCode int
		wantStatus int
	}{
		{name: "not found", statusCode: http.StatusNotFound, wantStatus: http.StatusNotFound},
		{name: "available", statusCode: http.StatusPartialContent, wantStatus: http.StatusPartialContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Header.Get("Range") != "bytes=0-0" {
					t.Errorf("Range header = %q, want bytes=0-0", req.Header.Get("Range"))
				}
				return &http.Response{
					StatusCode: tc.statusCode,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
					Request:    req,
				}, nil
			})}
			got, err := checkUploaderCandidate(context.Background(), "https://uu.getuploader.com/example/download/12", client)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.wantStatus {
				t.Fatalf("checkUploaderCandidate() status = %d, want %d", got, tc.wantStatus)
			}
		})
	}
}

func TestCheckUploaderListingCandidatesDeduplicatesAndCapsRequests(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	found := []uploaderListingCandidate{
		{URL: "https://uu.getuploader.com/example/download/0"},
		{URL: "https://uu.getuploader.com/example/download/0"},
	}
	for i := 1; i <= maxURLCollectionChecks; i++ {
		found = append(found, uploaderListingCandidate{
			URL: fmt.Sprintf("https://uu.getuploader.com/example/download/%d", i),
		})
	}

	candidates, checks, duplicates, _, truncated := checkUploaderListingCandidates(
		context.Background(), found, make(map[string]bool), 0, client,
	)
	if requests != maxURLCollectionChecks {
		t.Fatalf("request count = %d, want %d", requests, maxURLCollectionChecks)
	}
	if checks != maxURLCollectionChecks {
		t.Fatalf("checks = %d, want %d", checks, maxURLCollectionChecks)
	}
	if duplicates != 1 {
		t.Fatalf("duplicates = %d, want 1", duplicates)
	}
	if len(candidates) != maxURLCollectionChecks {
		t.Fatalf("candidate count = %d, want %d", len(candidates), maxURLCollectionChecks)
	}
	if !truncated {
		t.Fatal("expected remaining candidates to mark the scan as truncated")
	}
}

func TestNormalizeUploaderURLRejectsUnsupportedHost(t *testing.T) {
	if _, err := normalizeUploaderURL("https://example.com/download/12"); err == nil {
		t.Fatal("expected unsupported host to be rejected")
	}
}

func TestCandidateURLTitle(t *testing.T) {
	got := candidateURLTitle("https://uu.getuploader.com/example/download/12")
	if got != "12" {
		t.Fatalf("candidateURLTitle() = %q, want %q", got, "12")
	}
}

func TestIsGenericDownloadLabel(t *testing.T) {
	if !isGenericDownloadLabel(" ダウンロード ") || isGenericDownloadLabel("モデル配布") {
		t.Fatal("generic download label detection failed")
	}
}

func TestNormalizePublicCrawlURL(t *testing.T) {
	got, err := normalizePublicCrawlURL(" HTTPS://Example.COM:443/posts/1#section ")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://example.com/posts/1"; got != want {
		t.Fatalf("normalizePublicCrawlURL() = %q, want %q", got, want)
	}
}

func TestNormalizePublicCrawlURLRejectsUnsafeURLs(t *testing.T) {
	for _, raw := range []string{
		"http://example.com/post", "https://127.0.0.1/admin", "https://10.0.0.5/",
		"https://169.254.169.254/latest/meta-data/", "https://localhost/", "https://example.com:8443/",
		"https://user:password@example.com/",
	} {
		if _, err := normalizePublicCrawlURL(raw); err == nil {
			t.Errorf("normalizePublicCrawlURL(%q) should be rejected", raw)
		}
	}
}

func TestIsPublicCrawlIP(t *testing.T) {
	for _, raw := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		if !isPublicCrawlIP(net.ParseIP(raw)) {
			t.Errorf("expected %s to be public", raw)
		}
	}
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "100.64.0.1", "169.254.169.254", "192.0.2.1", "::1", "fd00::1", "2001:db8::1"} {
		if isPublicCrawlIP(net.ParseIP(raw)) {
			t.Errorf("expected %s to be blocked", raw)
		}
	}
}

func TestUnwrapDuckDuckGoURL(t *testing.T) {
	target := "https://example.org/article?q=1"
	wrapper := "https://duckduckgo.com/l/?uddg=" + url.QueryEscape(target) + "&rut=abc"
	got, ok := unwrapDuckDuckGoURL(wrapper)
	if !ok || got != target {
		t.Fatalf("unwrapDuckDuckGoURL() = %q, %v; want %q, true", got, ok, target)
	}
	if _, ok := unwrapDuckDuckGoURL("https://duckduckgo.com/about"); ok {
		t.Fatal("expected DuckDuckGo internal URL to be rejected")
	}
}

func TestParseDuckDuckGoHTML(t *testing.T) {
	body := []byte(`<html><body>
	<a class="result__a" href="https://duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.org%2Fpost">MMD 配布記事</a>
	<a class="result__a" href="https://duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.org%2Fpost">duplicate</a>
	<a class="result__a" href="https://duckduckgo.com/about">internal</a>
	<a class="result__a" href="https://example.net/page">別記事</a>
	</body></html>`)
	got := parseDuckDuckGoHTML(body, 10)
	if len(got) != 2 {
		t.Fatalf("parseDuckDuckGoHTML() returned %d results, want 2: %#v", len(got), got)
	}
	if got[0].URL != "https://example.org/post" || got[0].Title != "MMD 配布記事" {
		t.Fatalf("unexpected first result: %#v", got[0])
	}
	if got[1].URL != "https://example.net/page" {
		t.Fatalf("unexpected second result: %#v", got[1])
	}
}
