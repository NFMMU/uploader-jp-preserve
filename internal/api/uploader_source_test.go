package api

import (
	"net/url"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

func TestValidateUploaderURL(t *testing.T) {
	valid := []string{
		"https://uploader.jp/",
		"https://uu.getuploader.com/example/download/12",
		"https://dl1.getuploader.com/g/example/file.zip",
	}
	for _, raw := range valid {
		if _, err := validateUploaderURL(raw); err != nil {
			t.Errorf("expected URL to be accepted: %s (%v)", raw, err)
		}
	}
	invalid := []string{
		"http://uu.getuploader.com/example/download/12",
		"https://getuploader.com.evil.example/file.zip",
		"https://evilgetuploader.com/file.zip",
		"https://user:pass@uu.getuploader.com/file.zip",
		"https://uu.getuploader.com:444/file.zip",
	}
	for _, raw := range invalid {
		if _, err := validateUploaderURL(raw); err == nil {
			t.Errorf("expected URL to be rejected: %s", raw)
		}
	}
}

func TestFindDirectDownloadLink(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/example/download/12")
	doc, err := xhtml.Parse(strings.NewReader(`<html><body><a href="https://dl1.getuploader.com/g/example/abc.zip">Download</a></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	got := findDirectDownloadLink(doc, base)
	if got != "https://dl1.getuploader.com/g/example/abc.zip" {
		t.Fatalf("unexpected direct link: %q", got)
	}
}

func TestFindDirectDownloadLinkRecognizesDownloadHostAndDownloadX(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/blazeraptor/download/9")
	doc, err := xhtml.Parse(strings.NewReader(`<html><body><a href="https://download1.getuploader.com/g/token/blazeraptor/9/sample.zip">Download Now</a></body></html>`))
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

func TestFindDownloadForm(t *testing.T) {
	base, _ := url.Parse("https://uu.getuploader.com/example/download/12")
	doc, err := xhtml.Parse(strings.NewReader(`<html><body><form method="post" action="/example/download/12"><input type="hidden" name="token" value="abc"><input type="checkbox" name="agree" value="yes"><button type="submit" name="download" value="1">ダウンロード</button></form></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	form, ok := findDownloadForm(doc, base, true)
	if !ok {
		t.Fatal("expected download form")
	}
	if form.Method != "POST" || form.Action != "https://uu.getuploader.com/example/download/12" {
		t.Fatalf("unexpected form: %#v", form)
	}
	if form.Values.Get("token") != "abc" || form.Values.Get("download") != "1" || form.Values.Get("agree") != "yes" {
		t.Fatalf("missing form fields: %#v", form.Values)
	}
}

func TestPasswordAndCaptchaDetection(t *testing.T) {
	passwordDoc, _ := xhtml.Parse(strings.NewReader(`<html><body><input type="password" name="pass"></body></html>`))
	if !hasPasswordInput(passwordDoc) {
		t.Fatal("expected password input detection")
	}
	captchaDoc, _ := xhtml.Parse(strings.NewReader(`<html><body><div id="recaptcha-widget"></div></body></html>`))
	if !hasCaptcha(captchaDoc, strings.ToLower(htmlText(captchaDoc))) {
		t.Fatal("expected captcha detection")
	}
}

func TestIsUploaderDownloadPage(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://uu.getuploader.com/example/download/123", true},
		{"https://dl1.getuploader.com/g/example/abc.zip", true},
		{"https://uu.getuploader.com/example/", false},
		{"https://uploader.jp/", false},
		{"https://example.com/download/123", true}, // Host allowlist is checked before this helper is used.
	}
	for _, tc := range cases {
		if got := isUploaderDownloadPage(tc.url); got != tc.want {
			t.Errorf("isUploaderDownloadPage(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestIsUnconfirmedRedistributionPolicy(t *testing.T) {
	cases := []struct {
		policy string
		want   bool
	}{
		{"pending", true},
		{"in_progress", false},
		{"allowed", false},
		{"restricted", false},
		{"unknown", false},
	}
	for _, tc := range cases {
		if got := isUnconfirmedRedistributionPolicy(tc.policy); got != tc.want {
			t.Errorf("isUnconfirmedRedistributionPolicy(%q) = %v, want %v", tc.policy, got, tc.want)
		}
	}
}

func TestSourceFetchFailureIsPermanent(t *testing.T) {
	permanent := []string{
		"このファイルはダウンロードパスワードが必要です",
		"画像認証が必要です",
		"uploader.jp が HTTP 404 を返しました",
		`source content upload returned 409: {"error":"need 2 online nodes"}`,
	}
	for _, message := range permanent {
		if !sourceFetchFailureIsPermanent(message) {
			t.Errorf("expected permanent source-fetch failure: %q", message)
		}
	}
	if sourceFetchFailureIsPermanent("uploader.jpへの接続に失敗しました: connection reset") {
		t.Fatal("transient network failures should remain eligible for automatic retry")
	}
}
