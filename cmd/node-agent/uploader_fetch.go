package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
)

const uploaderFetcherVersion = "2026-10-24-password-form-1"
const maxAgentUploaderHTMLBytes = 8 << 20
const maxAgentSourceBytes int64 = 5 << 30

type fetchedUploaderFile struct {
	Body      io.ReadCloser
	Filename  string
	MediaType string
	Size      int64
	FinalURL  string
}

type uploaderForm struct {
	Action             string
	Method             string
	Values             url.Values
	PasswordFieldNames []string
}

func validateAgentUploaderURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || (u.Port() != "" && u.Port() != "443") {
		return "", errors.New("provide an HTTPS uploader.jp/getuploader.com URL")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if !(host == "uploader.jp" || strings.HasSuffix(host, ".uploader.jp") || host == "getuploader.com" || strings.HasSuffix(host, ".getuploader.com")) {
		return "", errors.New("unsupported source host")
	}
	return u.String(), nil
}

func safeSourceFilename(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = filepath.Base(value)
	value = strings.TrimSpace(strings.Trim(value, "."))
	if value == "" || value == ".." || len(value) > 500 || strings.ContainsRune(value, 0) {
		return ""
	}
	return value
}

func mustParseURL(raw string) *url.URL {
	u, _ := url.Parse(raw)
	if u == nil {
		return &url.URL{}
	}
	return u
}

func fetchUploaderFile(ctx context.Context, sourceURL string, acceptTerms bool) (*fetchedUploaderFile, error) {
	return fetchUploaderFileWithLimiter(ctx, sourceURL, acceptTerms, nil)
}

// fetchUploaderFileWithLimiter spaces every uploader request, including form
// submissions and redirects, using the calling agent's independent limiter.
func fetchUploaderFileWithLimiter(ctx context.Context, sourceURL string, acceptTerms bool, beforeRequest func(context.Context) error, passwords ...string) (*fetchedUploaderFile, error) {
	password := ""
	if len(passwords) > 0 {
		password = passwords[0]
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 2 * time.Hour,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 8 {
				return errors.New("too many redirects from uploader.jp")
			}
			if _, err := validateAgentUploaderURL(req.URL.String()); err != nil {
				return errors.New("uploader.jp redirected to an unsupported host; download stopped")
			}
			if beforeRequest != nil {
				if err := beforeRequest(req.Context()); err != nil {
					return err
				}
			}
			return nil
		},
	}
	currentURL := sourceURL
	var referer string
	seenURLs := make(map[string]struct{})
	for step := 0; step < 4; step++ {
		loopKey := normalizedUploaderLoopKey(currentURL)
		if _, seen := seenURLs[loopKey]; seen {
			return nil, errors.New("ダウンロードページが同じURLを繰り返しました。ページ側の遷移方式が変わった可能性があります。自動取得を停止しました")
		}
		seenURLs[loopKey] = struct{}{}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
		if err != nil {
			return nil, errors.New("invalid uploader.jp URL")
		}
		req.Header.Set("User-Agent", "uploader-jp-archive/1.0 (preservation request)")
		req.Header.Set("Accept", "text/html,application/octet-stream,*/*;q=0.8")
		if referer != "" {
			req.Header.Set("Referer", referer)
		}
		if beforeRequest != nil {
			if err := beforeRequest(ctx); err != nil {
				return nil, err
			}
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("uploader.jpへの接続に失敗しました: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("uploader.jpがHTTP %dを返しました。時間をおいて再試行してください", resp.StatusCode)
		}
		contentType := strings.ToLower(resp.Header.Get("Content-Type"))
		contentDisposition := resp.Header.Get("Content-Disposition")
		if strings.Contains(strings.ToLower(contentDisposition), "attachment") || (!strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml")) {
			filename := filenameFromContentDisposition(contentDisposition)
			if filename == "" {
				filename = safeSourceFilename(path.Base(resp.Request.URL.Path))
			}
			return &fetchedUploaderFile{Body: resp.Body, Filename: filename, MediaType: contentType, Size: resp.ContentLength, FinalURL: resp.Request.URL.String()}, nil
		}

		pageURL := resp.Request.URL
		page, readErr := io.ReadAll(io.LimitReader(resp.Body, maxAgentUploaderHTMLBytes+1))
		resp.Body.Close()
		if readErr != nil || len(page) > maxAgentUploaderHTMLBytes {
			return nil, errors.New("uploader.jpのダウンロードページが大きすぎるか、読み込めませんでした")
		}
		doc, parseErr := xhtml.Parse(bytes.NewReader(page))
		if parseErr != nil {
			return nil, errors.New("uploader.jpのダウンロードページを解析できませんでした")
		}
		pageText := strings.ToLower(htmlText(doc))
		hasPassword := hasPasswordInput(doc) || strings.Contains(pageText, "ダウンロードパスワードの認証が必要") || strings.Contains(pageText, "パスワードを入力")
		if hasPassword && password == "" {
			return nil, errors.New("このファイルはダウンロードパスワードが必要です。申請時または管理画面でパスワードを設定してください")
		}
		if hasCaptcha(doc, pageText) {
			return nil, errors.New("このページは画像認証などを要求しています。認証の回避は行いません。管理者がブラウザーで取得してください")
		}
		if !acceptTerms && (strings.Contains(pageText, "利用規約に同意した上で") || strings.Contains(pageText, "同意の上で")) {
			return nil, errors.New("ダウンロード前に利用規約への同意が必要です")
		}

		if nextURL := findMetaRefreshURL(doc, pageURL); nextURL != "" {
			referer = pageURL.String()
			currentURL = nextURL
			continue
		}
		if nextURL := findDirectDownloadLink(doc, pageURL); nextURL != "" {
			referer = pageURL.String()
			currentURL = nextURL
			continue
		}
		// Some uploader templates render the hand-off URL in an onclick/data
		// attribute or inline script instead of a normal anchor. Extract only
		// URLs that still pass the strict uploader-host allowlist below.
		if nextURL := findScriptDownloadURL(page, pageURL); nextURL != "" {
			referer = pageURL.String()
			currentURL = nextURL
			continue
		}
		form, ok := findDownloadForm(doc, pageURL, acceptTerms, password)
		if !ok {
			if hasPassword {
				return nil, errors.New("パスワードフォームを安全に特定できませんでした。フォームの形式が未対応のため自動取得を停止しました")
			}
			return nil, fmt.Errorf("ダウンロード先を自動判定できませんでした（診断: %s）。CAPTCHAやJavaScript認証は自動処理しません", uploaderPageDiagnostic(doc, pageURL, contentType, page))
		}
		formURL, err := validateAgentUploaderURL(form.Action)
		if err != nil {
			return nil, errors.New("ダウンロードフォームの送信先が許可されたドメインではありません")
		}
		method := strings.ToUpper(form.Method)
		formValues := uploaderFormSubmissionValues(form)
		var formReq *http.Request
		if method == http.MethodPost {
			formReq, err = http.NewRequestWithContext(ctx, http.MethodPost, formURL, strings.NewReader(formValues.Encode()))
			if err == nil {
				formReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
		} else {
			parsed, _ := url.Parse(formURL)
			query := parsed.Query()
			for key, values := range formValues {
				for _, value := range values {
					query.Add(key, value)
				}
			}
			parsed.RawQuery = query.Encode()
			formReq, err = http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
		}
		if err != nil {
			return nil, errors.New("ダウンロード要求を作成できませんでした")
		}
		formReq.Header.Set("User-Agent", "uploader-jp-archive/1.0 (preservation request)")
		formReq.Header.Set("Referer", pageURL.String())
		formReq.Header.Set("Accept", "text/html,application/octet-stream,*/*;q=0.8")
		if beforeRequest != nil {
			if err := beforeRequest(ctx); err != nil {
				return nil, err
			}
		}
		resp, err = client.Do(formReq)
		if err != nil {
			return nil, fmt.Errorf("ダウンロード要求に失敗しました: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("ダウンロード要求がHTTP %dを返しました", resp.StatusCode)
		}
		contentType = strings.ToLower(resp.Header.Get("Content-Type"))
		contentDisposition = resp.Header.Get("Content-Disposition")
		if strings.Contains(strings.ToLower(contentDisposition), "attachment") || (!strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml")) {
			filename := filenameFromContentDisposition(contentDisposition)
			if filename == "" {
				filename = safeSourceFilename(path.Base(resp.Request.URL.Path))
			}
			return &fetchedUploaderFile{Body: resp.Body, Filename: filename, MediaType: contentType, Size: resp.ContentLength, FinalURL: resp.Request.URL.String()}, nil
		}
		// Some public pages redirect to another confirmation page. Parse that page
		// in the next iteration, but never solve passwords, CAPTCHAs, or JS challenges.
		page, readErr = io.ReadAll(io.LimitReader(resp.Body, maxAgentUploaderHTMLBytes+1))
		resp.Body.Close()
		if readErr != nil || len(page) > maxAgentUploaderHTMLBytes {
			return nil, errors.New("ダウンロード確認ページを読み込めませんでした")
		}
		currentURL = resp.Request.URL.String()
		referer = pageURL.String()
		// Re-enter the normal page parser without an extra request by following a
		// same-site refresh/anchor when present. If no such link exists, fail closed.
		nextDoc, _ := xhtml.Parse(bytes.NewReader(page))
		if nextDoc == nil {
			return nil, errors.New("ダウンロード確認ページを解析できませんでした")
		}
		if hasPasswordInput(nextDoc) {
			return nil, errors.New("パスワード認証後も入力画面が表示されました。パスワードが誤っているか、追加操作が必要です")
		}
		if hasCaptcha(nextDoc, strings.ToLower(htmlText(nextDoc))) {
			return nil, errors.New("CAPTCHA等の追加認証は自動処理しません")
		}
		if nextURL := findDirectDownloadLink(nextDoc, resp.Request.URL); nextURL != "" {
			referer = resp.Request.URL.String()
			currentURL = nextURL
			continue
		}
		return nil, errors.New("追加の確認操作が必要です。管理者がブラウザーで取得してください")
	}
	return nil, errors.New("ダウンロードページの遷移回数が上限を超えました")
}

func filenameFromContentDisposition(value string) string {
	if value == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(value)
	if err != nil {
		return ""
	}
	return safeSourceFilename(params["filename"])
}

func htmlText(node *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return b.String()
}

func attr(node *xhtml.Node, key string) string {
	for _, a := range node.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func hasPasswordInput(root *xhtml.Node) bool {
	found := false
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.Data == "input" && strings.EqualFold(attr(n, "type"), "password") {
			found = true
		}
		for child := n.FirstChild; child != nil && !found; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

func hasCaptcha(root *xhtml.Node, pageText string) bool {
	if strings.Contains(pageText, "recaptcha") || strings.Contains(pageText, "hcaptcha") || strings.Contains(pageText, "captcha") || strings.Contains(pageText, "文字認証") || strings.Contains(pageText, "画像認証") {
		return true
	}
	found := false
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			v := strings.ToLower(attr(n, "name") + " " + attr(n, "id") + " " + attr(n, "class"))
			if strings.Contains(v, "captcha") {
				found = true
			}
		}
		for child := n.FirstChild; child != nil && !found; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

func findMetaRefreshURL(root *xhtml.Node, base *url.URL) string {
	if root == nil || base == nil {
		return ""
	}
	var found string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if found != "" {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "meta" && strings.EqualFold(strings.TrimSpace(attr(n, "http-equiv")), "refresh") {
			content := attr(n, "content")
			if idx := strings.Index(strings.ToLower(content), "url="); idx >= 0 {
				raw := strings.Trim(strings.TrimSpace(content[idx+4:]), "\\\"'")
				resolved, err := base.Parse(raw)
				if err == nil {
					if isAgentUploaderHandoffURL(resolved) {
						found = resolved.String()
					}
				}
			}
		}
		for child := n.FirstChild; child != nil && found == ""; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

func uploaderPageDiagnostic(root *xhtml.Node, pageURL *url.URL, contentType string, page []byte) string {
	title := ""
	formSummaries := make([]string, 0, 4)
	anchorCount := 0
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			switch n.Data {
			case "title":
				if title == "" {
					title = strings.Join(strings.Fields(htmlText(n)), " ")
				}
			case "a":
				if strings.TrimSpace(attr(n, "href")) != "" {
					anchorCount++
				}
			case "form":
				if len(formSummaries) < 4 {
					method := strings.ToUpper(strings.TrimSpace(attr(n, "method")))
					if method == "" {
						method = "GET"
					}
					action := attr(n, "action")
					if action == "" {
						action = pageURL.Path
					} else if parsed, err := pageURL.Parse(action); err == nil {
						action = parsed.Path
					} else {
						action = "<invalid>"
					}
					fields := make([]string, 0, 8)
					var fieldWalk func(*xhtml.Node)
					fieldWalk = func(child *xhtml.Node) {
						if child.Type == xhtml.ElementNode && (child.Data == "input" || child.Data == "button" || child.Data == "select" || child.Data == "textarea") {
							name := strings.TrimSpace(attr(child, "name"))
							if name != "" && len(fields) < 8 {
								fields = append(fields, name)
							}
						}
						for c := child.FirstChild; c != nil; c = c.NextSibling {
							fieldWalk(c)
						}
					}
					fieldWalk(n)
					formSummaries = append(formSummaries, fmt.Sprintf("%s %s fields=%s", method, action, strings.Join(fields, ",")))
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	if len(title) > 100 {
		title = title[:100]
	}
	title = strings.ReplaceAll(strings.ReplaceAll(title, "\n", " "), "\r", " ")
	if title == "" {
		title = "(no-title)"
	}
	urlLabel := "unknown"
	if pageURL != nil {
		urlLabel = pageURL.Hostname() + pageURL.EscapedPath()
	}
	scriptHasDownload := strings.Contains(strings.ToLower(string(page)), "download")
	return fmt.Sprintf("url=%s content_type=%q title=%q anchors=%d forms=[%s] script_mentions_download=%t", urlLabel, contentType, title, anchorCount, strings.Join(formSummaries, "; "), scriptHasDownload)
}

func findDirectDownloadLink(root *xhtml.Node, base *url.URL) string {
	if root == nil || base == nil {
		return ""
	}
	best := ""
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if best != "" {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "a" {
			href := strings.TrimSpace(attr(n, "href"))
			if href != "" {
				resolved, err := base.Parse(href)
				if err == nil && resolved.String() != base.String() && isAgentUploaderHandoffURL(resolved) {
					best = resolved.String()
					return
				}
			}
		}
		for child := n.FirstChild; child != nil && best == ""; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return best
}

// Only follow actual download hand-off targets. A generic /download/<id> URL
// is another landing page, not a file URL; following it based on an anchor or
// script label caused loops and repeated requests against the same page.
func isAgentUploaderHandoffURL(target *url.URL) bool {
	if target == nil {
		return false
	}
	if _, err := validateAgentUploaderURL(target.String()); err != nil {
		return false
	}
	host := strings.ToLower(target.Hostname())
	pathLower := strings.ToLower(target.Path)
	if strings.Contains(pathLower, "downloadx") {
		return true
	}
	directHost := (strings.HasPrefix(host, "dl") || strings.HasPrefix(host, "download")) && strings.HasSuffix(host, ".getuploader.com")
	return directHost && strings.Contains(pathLower, "/g/")
}

func normalizedUploaderLoopKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Fragment = ""
	return u.String()
}

func uploaderFormSubmissionValues(form uploaderForm) url.Values {
	// The common getuploader handshake posts only the hidden token; md5 is page
	// metadata and is not submitted by the standard browser flow. A few template
	// variants use a more explicit token field name, so preserve that single field.
	for key, values := range form.Values {
		keyLower := strings.ToLower(key)
		if (keyLower == "token" || keyLower == "download_token" || keyLower == "downloadtoken") && len(values) > 0 && strings.TrimSpace(values[0]) != "" {
			if len(form.PasswordFieldNames) > 0 {
				return form.Values
			}
			return url.Values{key: []string{values[0]}}
		}
	}
	return form.Values
}

var uploaderScriptURLPattern = regexp.MustCompile(`(?i)(?:https?:)?//[a-z0-9.-]*(?:uploader\.jp|getuploader\.com)[^"'\s<>\x60]+|/(?:[a-z0-9_./%+-]*download(?:x|/|\.php)[a-z0-9_./%+-]*)(?:\?[^"'\s<>\x60]*)?`)

func findScriptDownloadURL(page []byte, base *url.URL) string {
	if base == nil {
		return ""
	}
	// JSON/JS sometimes escapes URL slashes as \/.
	searchable := []byte(strings.ReplaceAll(string(page), `\/`, `/`))
	for _, match := range uploaderScriptURLPattern.FindAll(searchable, 64) {
		raw := strings.TrimRight(strings.TrimSpace(string(match)), ");,]}\\")
		resolved, err := base.Parse(raw)
		if err != nil {
			continue
		}
		if _, err := validateAgentUploaderURL(resolved.String()); err != nil {
			continue
		}
		if resolved.String() == base.String() {
			continue
		}
		if isAgentUploaderHandoffURL(resolved) {
			return resolved.String()
		}
	}
	return ""
}

func findDownloadForm(root *xhtml.Node, base *url.URL, acceptTerms bool, passwords ...string) (uploaderForm, bool) {
	password := ""
	if len(passwords) > 0 {
		password = passwords[0]
	}
	var result uploaderForm
	var walk func(*xhtml.Node) bool
	walk = func(n *xhtml.Node) bool {
		if n.Type == xhtml.ElementNode && n.Data == "form" {
			form := uploaderForm{Action: attr(n, "action"), Method: strings.ToUpper(strings.TrimSpace(attr(n, "method"))), Values: url.Values{}}
			if form.Method == "" {
				form.Method = http.MethodGet
			}
			if form.Action == "" {
				form.Action = base.String()
			} else if parsed, err := base.Parse(form.Action); err == nil {
				form.Action = parsed.String()
			}
			var findControls func(*xhtml.Node)
			buttonFound := false
			tokenLikeFound := false
			md5Found := false
			passwordFieldFound := false
			hiddenFieldCount := 0
			findControls = func(node *xhtml.Node) {
				if node.Type == xhtml.ElementNode {
					switch node.Data {
					case "input":
						name, typ, value := attr(node, "name"), strings.ToLower(attr(node, "type")), attr(node, "value")
						if typ == "hidden" && strings.TrimSpace(name) != "" && strings.TrimSpace(value) != "" {
							hiddenFieldCount++
						}
						// Submit inputs often have no name. Their label still identifies the
						// normal download action, so inspect it before checking the name.
						if typ == "submit" || typ == "button" || typ == "image" {
							label := strings.ToLower(value + " " + name + " " + attr(node, "id") + " " + attr(node, "alt") + " " + attr(node, "title") + " " + attr(node, "aria-label"))
							if strings.Contains(label, "ダウンロード") || strings.Contains(label, "download") {
								buttonFound = true
								if name != "" {
									form.Values.Add(name, value)
								}
							}
						}
						if name != "" {
							nameLower := strings.ToLower(name)
							if strings.Contains(nameLower, "token") && strings.TrimSpace(value) != "" {
								tokenLikeFound = true
							}
							if nameLower == "md5" && strings.TrimSpace(value) != "" {
								md5Found = true
							}
							switch typ {
							case "password":
								if password != "" {
									form.Values.Set(name, password)
									form.PasswordFieldNames = append(form.PasswordFieldNames, name)
									passwordFieldFound = true
								}
							case "submit", "button", "image", "file":
								// Submit values were handled above; file inputs are never sent.
							case "checkbox", "radio":
								fieldHint := strings.ToLower(name + " " + attr(node, "id") + " " + value)
								termsField := strings.Contains(fieldHint, "agree") || strings.Contains(fieldHint, "kiyaku") || strings.Contains(fieldHint, "terms") || strings.Contains(fieldHint, "rule")
								if attr(node, "checked") != "" || (acceptTerms && termsField) {
									if value == "" {
										value = "on"
									}
									form.Values.Add(name, value)
								}
							default:
								form.Values.Add(name, value)
							}
						}
					case "button":
						label := strings.ToLower(htmlText(node) + " " + attr(node, "name") + " " + attr(node, "id") + " " + attr(node, "title") + " " + attr(node, "aria-label"))
						if strings.Contains(label, "ダウンロード") || strings.Contains(label, "download") {
							name := attr(node, "name")
							if name != "" {
								form.Values.Add(name, attr(node, "value"))
							}
							buttonFound = true
						}
					}
				}
				for child := node.FirstChild; child != nil; child = child.NextSibling {
					findControls(child)
				}
			}
			findControls(n)
			// The standard getuploader handshake can be a same-host POST form with a
			// hidden token and an unnamed submit input. Password/CAPTCHA pages are
			// rejected before this helper is called; no authentication is bypassed.
			baseURL, baseErr := url.Parse(base.String())
			formURL, formErr := url.Parse(form.Action)
			sameHost := baseErr == nil && formErr == nil && strings.EqualFold(baseURL.Hostname(), formURL.Hostname())
			formPath := ""
			if formURL != nil {
				formPath = strings.ToLower(formURL.Path)
			}
			downloadAction := strings.Contains(formPath, "/download/") || strings.Contains(formPath, "downloadx")
			// A subset of templates has no readable submit label (or has a JS-bound
			// submit button). Accept a same-host download action with a token, md5, or
			// other hidden fields. Field values are never written to diagnostic logs.
			supportedMethod := form.Method == http.MethodGet || form.Method == http.MethodPost
			sameHostTokenForm := sameHost && downloadAction && tokenLikeFound && supportedMethod
			sameHostMetadataForm := sameHost && downloadAction && md5Found && supportedMethod
			sameHostHiddenForm := sameHost && downloadAction && hiddenFieldCount > 0 && supportedMethod
			sameHostPasswordForm := sameHost && downloadAction && passwordFieldFound && supportedMethod
			if (buttonFound || sameHostTokenForm || sameHostMetadataForm || sameHostHiddenForm || sameHostPasswordForm) && supportedMethod {
				if form.Action != "" {
					result = form
					return true
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if walk(child) {
				return true
			}
		}
		return false
	}
	ok := walk(root)
	return result, ok
}
