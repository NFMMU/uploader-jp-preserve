package main

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	xhtml "golang.org/x/net/html"
)

func TestValidateAgentUploaderURL(t *testing.T) {
	valid := []string{
		"https://www.uploader.jp/file/123",
		"https://uu.getuploader.com/name/download/123",
		"https://dl.getuploader.com/g/name/123",
	}
	for _, raw := range valid {
		if _, err := validateAgentUploaderURL(raw); err != nil {
			t.Errorf("expected URL to be allowed: %s: %v", raw, err)
		}
	}
	invalid := []string{
		"http://www.uploader.jp/file/123",
		"https://example.com/file/123",
		"https://uploader.jp.example.com/file/123",
		"https://user:pass@uploader.jp/file/123",
	}
	for _, raw := range invalid {
		if _, err := validateAgentUploaderURL(raw); err == nil {
			t.Errorf("expected URL to be rejected: %s", raw)
		}
	}
}

func TestFindDirectDownloadLinkRecognizesGetUploaderHosts(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/blazeraptor/download/9")
	doc, err := xhtml.Parse(strings.NewReader(`<html><body><a href="https://download1.getuploader.com/g/token/blazeraptor/9/sample.zip">Download Now</a><a href="https://uu.getuploader.com/blazeraptor/downloadx?token=abc">continue</a></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	got := findDirectDownloadLink(doc, base)
	if got != "https://download1.getuploader.com/g/token/blazeraptor/9/sample.zip" {
		t.Fatalf("unexpected direct link: %q", got)
	}
}

func TestFindDownloadXLink(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/blazeraptor/download/9")
	doc, err := xhtml.Parse(strings.NewReader(`<html><body><a href="https://uu.getuploader.com/blazeraptor/downloadx?token=abc">continue</a></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	got := findDirectDownloadLink(doc, base)
	if got != "https://uu.getuploader.com/blazeraptor/downloadx?token=abc" {
		t.Fatalf("unexpected downloadx link: %q", got)
	}
}

func TestFindDownloadFormAcceptsGetUploaderTokenHandshake(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/blazeraptor/download/9")
	doc, err := xhtml.Parse(strings.NewReader(`<html><body><form method="post" action="/blazeraptor/download/9"><input type="hidden" name="md5" value="abc123"><input type="hidden" name="token" value="secret-token"><input type="submit" value="Download"></form></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	form, ok := findDownloadForm(doc, base, true)
	if !ok {
		t.Fatal("expected standard getuploader token form to be detected")
	}
	if form.Method != "POST" || form.Values.Get("token") != "secret-token" || form.Values.Get("md5") != "abc123" {
		t.Fatalf("unexpected token form: method=%q values=%v", form.Method, form.Values)
	}
}

func TestFindDownloadFormIncludesProvidedPassword(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/name/download/123")
	doc, err := xhtml.Parse(strings.NewReader(`<html><body><form method="post" action="/name/download/123"><input type="hidden" name="token" value="abc"><input type="password" name="download_pass"><button type="submit">認証</button></form></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	formWithoutPassword, ok := findDownloadForm(doc, base, true)
	if !ok || formWithoutPassword.Values.Get("download_pass") != "" {
		t.Fatal("password field should remain unset when no password was supplied")
	}
	form, ok := findDownloadForm(doc, base, true, "correct horse")
	if !ok {
		t.Fatal("expected password-protected download form to be detected")
	}
	if form.Values.Get("download_pass") != "correct horse" {
		t.Fatalf("password field was not populated: %v", form.Values)
	}
	values := uploaderFormSubmissionValues(form)
	if values.Get("token") != "abc" || values.Get("download_pass") != "correct horse" {
		t.Fatalf("expected token and supplied password, got %v", values)
	}
}

func TestUploaderFormSubmissionUsesOnlyToken(t *testing.T) {
	form := uploaderForm{Values: url.Values{"token": []string{"secret-token"}, "md5": []string{"abc123"}, "agree": []string{"yes"}}}
	got := uploaderFormSubmissionValues(form)
	if len(got) != 1 || got.Get("token") != "secret-token" {
		t.Fatalf("expected only token to be submitted, got %v", got)
	}
}

func TestFindDownloadFormAcceptsTokenFormWithoutNamedSubmit(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/blazeraptor/download/9")
	doc, err := xhtml.Parse(strings.NewReader(`<html><body><form method="post"><input type="hidden" name="token" value="secret-token"><input type="hidden" name="md5" value="abc123"></form></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findDownloadForm(doc, base, true); !ok {
		t.Fatal("expected token-only POST form to be detected")
	}
}

func TestFindScriptDownloadURL(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/blazeraptor/download/9")
	page := []byte(`<html><script>location.href = "\/blazeraptor/downloadx?token=abc";</script></html>`)
	got := findScriptDownloadURL(page, base)
	if got != "https://uu.getuploader.com/blazeraptor/downloadx?token=abc" {
		t.Fatalf("unexpected script download URL: %q", got)
	}
}

func TestFindDownloadFormAcceptsTokenLikeFieldWithoutSubmitLabel(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/blazeraptor/download/9")
	doc, err := xhtml.Parse(strings.NewReader(`<html><body><form method="post" action="/blazeraptor/download/9"><input type="hidden" name="download_token" value="secret-token"><input type="submit" value=""></form></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	form, ok := findDownloadForm(doc, base, true)
	if !ok {
		t.Fatal("expected token-like form to be detected without a readable submit label")
	}
	got := uploaderFormSubmissionValues(form)
	if len(got) != 1 || got.Get("download_token") != "secret-token" {
		t.Fatalf("expected only the token-like field to be submitted, got %v", got)
	}
}

func TestFindDownloadFormAcceptsMetadataFormWithoutSubmitLabel(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/blazeraptor/download/9")
	doc, err := xhtml.Parse(strings.NewReader(`<html><body><form method="post" action="/blazeraptor/download/9"><input type="hidden" name="md5" value="abc123"><input type="submit" value=""></form></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findDownloadForm(doc, base, true); !ok {
		t.Fatal("expected same-host download form with md5 metadata to be detected")
	}
}

func TestFindDownloadFormAcceptsSameHostDownloadFormWithUnknownHiddenField(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/blazeraptor/download/9")
	doc, err := xhtml.Parse(strings.NewReader(`<html><body><form method="post" action="/blazeraptor/download/9"><input type="hidden" name="dl_key" value="secret"><input type="submit" value=""></form></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	form, ok := findDownloadForm(doc, base, true)
	if !ok {
		t.Fatal("expected same-host download form with an unknown hidden field to be detected")
	}
	got := uploaderFormSubmissionValues(form)
	if got.Get("dl_key") != "secret" {
		t.Fatalf("expected hidden field to be preserved, got %v", got)
	}
}

func TestFindMetaRefreshURL(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/blazeraptor/download/9")
	doc, err := xhtml.Parse(strings.NewReader(`<html><head><meta http-equiv="refresh" content="0; url=/blazeraptor/downloadx?token=abc"></head></html>`))
	if err != nil {
		t.Fatal(err)
	}
	got := findMetaRefreshURL(doc, base)
	if got != "https://uu.getuploader.com/blazeraptor/downloadx?token=abc" {
		t.Fatalf("unexpected meta refresh URL: %q", got)
	}
}

func TestFindScriptDownloadURLDoesNotFollowAnotherLandingPage(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/blazeraptor/download/9")
	page := []byte(`<script>window.location = "/blazeraptor/download/9?step=2";</script>`)
	if got := findScriptDownloadURL(page, base); got != "" {
		t.Fatalf("must not treat another landing page as a direct download target: %q", got)
	}
}

func TestFindDirectDownloadLinkIgnoresGenericDownloadLabel(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/blazeraptor/download/9")
	doc, err := xhtml.Parse(strings.NewReader(`<html><body><a href="/blazeraptor/download/9?step=2">ダウンロード</a></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	if got := findDirectDownloadLink(doc, base); got != "" {
		t.Fatalf("must ignore generic landing-page links to avoid loops: %q", got)
	}
}

func TestNormalizedUploaderLoopKeyDropsFragmentOnly(t *testing.T) {
	got := normalizedUploaderLoopKey("https://uu.getuploader.com/blazeraptor/download/9?step=2#section")
	if got != "https://uu.getuploader.com/blazeraptor/download/9?step=2" {
		t.Fatalf("unexpected normalized URL: %q", got)
	}
}

func TestUploaderPageDiagnosticDoesNotExposeHiddenValues(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/blazeraptor/download/9")
	doc, err := xhtml.Parse(strings.NewReader(`<html><head><title>Download</title></head><body><form method="post"><input type="hidden" name="token" value="do-not-log-this-secret"></form></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	diagnostic := uploaderPageDiagnostic(doc, base, "text/html", []byte(`<script>download</script>`))
	if strings.Contains(diagnostic, "do-not-log-this-secret") {
		t.Fatal("diagnostic leaked a hidden field value")
	}
	if !strings.Contains(diagnostic, "fields=token") || !strings.Contains(diagnostic, "title=\"Download") {
		t.Fatalf("diagnostic missing useful context: %s", diagnostic)
	}
}

func TestUploaderRateLimitPersistsPerAgent(t *testing.T) {
	state := filepath.Join(t.TempDir(), "node-state.json")
	a := &agent{cfg: config{StatePath: state, UploaderMinInterval: 30 * time.Millisecond}, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	start := time.Now()
	if err := a.waitForUploaderInterval(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.waitForUploaderInterval(context.Background()); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 25*time.Millisecond {
		t.Fatal("second fetch did not respect the per-agent interval")
	}
	if _, err := os.Stat(state + ".uploader-last-fetch"); err != nil {
		t.Fatalf("rate-limit timestamp was not persisted: %v", err)
	}
	b := &agent{cfg: config{StatePath: state, UploaderMinInterval: 30 * time.Millisecond}, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	b.loadUploaderFetchTimestamp()
	if b.lastUploaderFetch.IsZero() {
		t.Fatal("new agent instance did not load persisted fetch timestamp")
	}
}
