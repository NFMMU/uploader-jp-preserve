package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	xhtml "golang.org/x/net/html"
	"gorm.io/gorm"
)

const maxUploaderHTMLBytes = 8 << 20

type uploaderForm struct {
	Action string
	Method string
	Values url.Values
}

func (s *Server) ingestSourceFile(c *gin.Context) {
	fileID := c.Param("id")
	if _, err := uuid.Parse(fileID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}
	var req struct {
		SourceURL       string `json:"source_url" binding:"omitempty,max=4096"`
		AcceptTerms     bool   `json:"accept_terms"`
		Priority        *int   `json:"priority" binding:"omitempty,min=0,max=5"`
		Password        string `json:"password" binding:"omitempty,max=1024"`
		PublicRequestID string `json:"public_request_id" binding:"omitempty,uuid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source-ingest request"})
		return
	}
	if !req.AcceptTerms {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirm the uploader.jp terms before starting the download"})
		return
	}
	priority := 3
	if req.Priority != nil {
		priority = *req.Priority
	}

	var file model.FileObject
	if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ? AND deleted_at IS NULL AND deletion_requested_at IS NULL", fileID).Error; err != nil {
		notFoundOrError(c, err, "file object")
		return
	}
	if file.PreservationStatus == "verified" {
		c.JSON(http.StatusConflict, gin.H{"error": "file object is already verified; create a new file object for another version"})
		return
	}
	if isUnconfirmedRedistributionPolicy(file.RedistributionPolicy) {
		c.JSON(http.StatusConflict, gin.H{"error": "redistribution policy is unconfirmed; set a policy other than pending before preservation"})
		return
	}
	passwordEncrypted := file.SourcePasswordEncrypted
	if req.Password != "" {
		encrypted, encryptErr := s.encryptSourcePassword(req.Password)
		if encryptErr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": encryptErr.Error()})
			return
		}
		passwordEncrypted = encrypted
	} else if req.PublicRequestID != "" {
		var publicRequest model.PublicRequest
		if err := s.db.WithContext(c.Request.Context()).Where("id = ? AND request_type = 'preservation'", req.PublicRequestID).First(&publicRequest).Error; err != nil {
			notFoundOrError(c, err, "preservation request")
			return
		}
		if publicRequest.ArtifactID != nil && *publicRequest.ArtifactID != file.ArtifactID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "preservation request does not belong to this file's artifact"})
			return
		}
		if publicRequest.SourcePasswordEncrypted != nil {
			passwordEncrypted = publicRequest.SourcePasswordEncrypted
		}
	}
	var activeTasks int64
	if err := s.db.WithContext(c.Request.Context()).Model(&model.Task{}).Where("file_object_id = ? AND status IN ('queued', 'running')", file.ID).Count(&activeTasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check existing tasks"})
		return
	}
	if activeTasks > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "a preservation task is already active for this file"})
		return
	}

	sourceURL := strings.TrimSpace(req.SourceURL)
	if sourceURL == "" && file.SourceURL != nil {
		sourceURL = strings.TrimSpace(*file.SourceURL)
	}
	validatedURL, err := validateUploaderURL(sourceURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide an HTTPS uploader.jp/getuploader.com download-page URL"})
		return
	}
	if _, err := s.selectNodesForCapacity(c, 0, 2); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "保存先として使えるオンラインノードが2台必要です: " + err.Error()})
		return
	}
	var node model.Node
	if err := s.db.WithContext(c.Request.Context()).Where("status = 'online' AND agent_version LIKE '0.5.%'").Order("random()").First(&node).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "取得タスクに対応したオンライン agent がありません。全ノードの agent を再ビルド・再起動してください"})
		return
	}
	taskID := uuid.NewString()
	task := model.Task{ID: taskID, TaskType: "source_fetch", FileObjectID: file.ID, TargetNodeID: &node.ID, Status: "queued", Priority: int16(priority)}
	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.FileObject{}).Where("id = ?", file.ID).Updates(map[string]any{"source_url": validatedURL, "source_password_encrypted": passwordEncrypted, "preservation_status": "queued"}).Error; err != nil {
			return err
		}
		return tx.Create(&task).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not queue source-fetch task"})
		return
	}
	actor, _ := currentUser(c)
	s.audit(c, &actor, "file_object", file.ID, "file_object.source_fetch.queued", "success", stringPtr("agent="+node.Name))
	c.JSON(http.StatusAccepted, gin.H{
		"file_object_id":    file.ID,
		"source_fetch_task": gin.H{"task_id": task.ID, "task_type": task.TaskType, "node_id": node.ID, "node_name": node.Name, "status": task.Status},
		"tasks":             []any{},
		"status":            "queued",
		"message":           "取得タスクを agent に登録しました。agent ごとの取得間隔を空けてからダウンロードし、成功後に保存タスクを作成します。",
	})
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

func findDirectDownloadLink(root *xhtml.Node, base *url.URL) string {
	best := ""
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.Data == "a" {
			href := strings.TrimSpace(attr(n, "href"))
			if href != "" {
				resolved, err := base.Parse(href)
				if err == nil && resolved.String() != base.String() {
					if _, err := validateUploaderURL(resolved.String()); err == nil {
						host := strings.ToLower(resolved.Hostname())
						label := strings.ToLower(htmlText(n))
						// getuploader commonly exposes files through download1.getuploader.com/g/...
						directHost := (strings.HasPrefix(host, "dl") || strings.HasPrefix(host, "download")) && strings.HasSuffix(host, ".getuploader.com")
						if directHost && strings.Contains(strings.ToLower(resolved.Path), "/g/") {
							best = resolved.String()
							return
						}
						// The normal hand-off endpoint is sometimes named downloadx.
						if strings.Contains(strings.ToLower(resolved.String()), "downloadx") {
							best = resolved.String()
							return
						}
						if best == "" && (strings.Contains(label, "ダウンロード") || strings.Contains(label, "download")) {
							best = resolved.String()
						}
					}
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

func findDownloadForm(root *xhtml.Node, base *url.URL, acceptTerms bool) (uploaderForm, bool) {
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
			tokenFound := false
			findControls = func(node *xhtml.Node) {
				if node.Type == xhtml.ElementNode {
					switch node.Data {
					case "input":
						name, typ, value := attr(node, "name"), strings.ToLower(attr(node, "type")), attr(node, "value")
						// Submit inputs often have no name. Their label still identifies the
						// normal download action, so inspect it before checking the name.
						if typ == "submit" || typ == "button" || typ == "image" {
							label := strings.ToLower(value + " " + attr(node, "alt") + " " + attr(node, "title") + " " + attr(node, "aria-label"))
							if strings.Contains(label, "ダウンロード") || strings.Contains(label, "download") {
								buttonFound = true
								if name != "" {
									form.Values.Add(name, value)
								}
							}
						}
						if name != "" {
							if strings.EqualFold(name, "token") && strings.TrimSpace(value) != "" {
								tokenFound = true
							}
							switch typ {
							case "submit", "button", "image", "password", "file":
								// Submit values were handled above; never send password/file inputs.
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
						label := strings.ToLower(htmlText(node) + " " + attr(node, "title") + " " + attr(node, "aria-label"))
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
			sameHostTokenForm := tokenFound && form.Method == http.MethodPost && baseErr == nil && formErr == nil && strings.EqualFold(baseURL.Hostname(), formURL.Hostname())
			if (buttonFound || sameHostTokenForm) && (form.Method == http.MethodGet || form.Method == http.MethodPost) {
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

func submitUploaderListingPassword(ctx context.Context, client *http.Client, pageURL *url.URL, doc *xhtml.Node, password string) (*url.URL, []byte, error) {
	if pageURL == nil || doc == nil || password == "" {
		return nil, nil, errors.New("一覧ページのパスワード送信情報が不正です")
	}
	var action, method, passwordField string
	values := url.Values{}
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if action != "" {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "form" {
			formValues := url.Values{}
			foundPassword := false
			var collect func(*xhtml.Node)
			collect = func(child *xhtml.Node) {
				if child.Type == xhtml.ElementNode && child.Data == "input" {
					name := strings.TrimSpace(attr(child, "name"))
					typ := strings.ToLower(strings.TrimSpace(attr(child, "type")))
					value := attr(child, "value")
					if typ == "password" {
						if name != "" {
							passwordField = name
							formValues.Set(name, password)
							foundPassword = true
						}
					} else if name != "" && (typ == "submit" || typ == "button") {
						if value != "" {
							formValues.Add(name, value)
						}
					} else if name != "" && typ != "file" && typ != "image" {
						if (typ != "checkbox" && typ != "radio") || attr(child, "checked") != "" {
							formValues.Add(name, value)
						}
					}
				}
				for c := child.FirstChild; c != nil; c = c.NextSibling {
					collect(c)
				}
			}
			collect(n)
			if foundPassword {
				formAction := strings.TrimSpace(attr(n, "action"))
				if formAction == "" {
					formAction = pageURL.String()
				} else if parsed, err := pageURL.Parse(formAction); err == nil {
					formAction = parsed.String()
				} else {
					return
				}
				action = formAction
				method = strings.ToUpper(strings.TrimSpace(attr(n, "method")))
				if method == "" {
					method = http.MethodGet
				}
				values = formValues
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if action == "" || passwordField == "" {
		return nil, nil, errors.New("一覧ページのパスワードフォームを特定できませんでした")
	}
	validated, err := validateUploaderURL(action)
	if err != nil {
		return nil, nil, errors.New("パスワードフォームの送信先が許可されたアップローダーではありません")
	}
	target, _ := url.Parse(validated)
	if target == nil || !strings.EqualFold(target.Hostname(), pageURL.Hostname()) {
		return nil, nil, errors.New("パスワードは元ページと同じホストにのみ送信できます")
	}
	var req *http.Request
	switch method {
	case http.MethodPost:
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, target.String(), strings.NewReader(values.Encode()))
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	case http.MethodGet:
		query := target.Query()
		for key, vals := range values {
			for _, value := range vals {
				query.Add(key, value)
			}
		}
		target.RawQuery = query.Encode()
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	default:
		return nil, nil, errors.New("一覧ページのパスワードフォームの送信方式に対応していません")
	}
	if err != nil {
		return nil, nil, errors.New("パスワード送信リクエストを作成できませんでした")
	}
	req.Header.Set("User-Agent", "uploader-jp-archive/1.0 (preservation listing preview)")
	req.Header.Set("Referer", pageURL.String())
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, errors.New("パスワード送信に失敗しました")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("パスワード送信後に HTTP %d が返されました", resp.StatusCode)
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
		return nil, nil, errors.New("パスワード送信後に一覧HTMLを取得できませんでした")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUploaderHTMLBytes+1))
	if err != nil || len(body) > maxUploaderHTMLBytes {
		return nil, nil, errors.New("パスワード送信後の一覧ページが大きすぎるか読み込めません")
	}
	return resp.Request.URL, body, nil
}

// uploaderListingCandidate is a public download page discovered on one uploader listing.
type uploaderListingCandidate struct {
	URL   string `json:"url"`
	Label string `json:"label"`
}

const (
	maxUploaderListingPages      = 10
	maxUploaderListingCandidates = 1000
)

// discoverUploaderFiles previews file links from a specific uploader page. It does not
// download file bodies; the administrator explicitly selects candidates for serial ingest.
func (s *Server) discoverUploaderFiles(c *gin.Context) {
	artifactID := c.Param("id")
	if _, err := uuid.Parse(artifactID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid artifact id"})
		return
	}
	var artifact model.Artifact
	if err := s.db.WithContext(c.Request.Context()).First(&artifact, "id = ?", artifactID).Error; err != nil {
		notFoundOrError(c, err, "artifact")
		return
	}
	var req struct {
		ListingURL      string `json:"listing_url" binding:"required,max=4096"`
		AcceptTerms     bool   `json:"accept_terms"`
		Password        string `json:"password" binding:"omitempty,max=1024"`
		PublicRequestID string `json:"public_request_id" binding:"omitempty,uuid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.AcceptTerms {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a listing URL and confirmation of uploader.jp terms are required"})
		return
	}
	listingURL, err := validateUploaderURL(req.ListingURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide an HTTPS uploader.jp/getuploader.com listing URL"})
		return
	}
	password := req.Password
	if password == "" && req.PublicRequestID != "" {
		var publicRequest model.PublicRequest
		if err := s.db.WithContext(c.Request.Context()).Where("id = ? AND request_type = 'preservation'", req.PublicRequestID).First(&publicRequest).Error; err != nil {
			notFoundOrError(c, err, "preservation request")
			return
		}
		password, err = s.decryptSourcePassword(publicRequest.SourcePasswordEncrypted)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
	} else if password != "" && len(s.cfg.SourcePasswordEncryptionKey) != 32 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": errSourcePasswordKeyMissing.Error()})
		return
	}
	candidates, pages, err := discoverUploaderListing(c.Request.Context(), listingURL, password)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	actor, _ := currentUser(c)
	s.audit(c, &actor, "artifact", artifact.ID, "artifact.source_discover", "success", stringPtr(fmt.Sprintf("pages=%d candidates=%d", pages, len(candidates))))
	c.JSON(http.StatusOK, gin.H{"listing_url": listingURL, "pages_scanned": pages, "limit": maxUploaderListingCandidates, "truncated": len(candidates) >= maxUploaderListingCandidates, "items": candidates})
}

func discoverUploaderListing(ctx context.Context, listingURL string, passwords ...string) ([]uploaderListingCandidate, int, error) {
	return discoverUploaderListingWithLimits(ctx, listingURL, maxUploaderListingPages, maxUploaderListingCandidates, passwords...)
}

func discoverUploaderListingWithLimits(ctx context.Context, listingURL string, pageLimit, candidateLimit int, passwords ...string) ([]uploaderListingCandidate, int, error) {
	if pageLimit < 1 || candidateLimit < 1 {
		return []uploaderListingCandidate{}, 0, nil
	}
	start, err := url.Parse(listingURL)
	if err != nil {
		return nil, 0, errors.New("invalid uploader.jp listing URL")
	}
	password := ""
	if len(passwords) > 0 {
		password = passwords[0]
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects while reading uploader listing")
			}
			if _, err := validateUploaderURL(req.URL.String()); err != nil {
				return errors.New("uploader listing redirected to an unsupported host")
			}
			return nil
		},
	}
	queue := []string{start.String()}
	seenPages := map[string]bool{}
	seenCandidates := map[string]bool{}
	items := make([]uploaderListingCandidate, 0, 32)
	pagesScanned := 0
	for len(queue) > 0 && pagesScanned < pageLimit && len(items) < candidateLimit {
		pageURL := queue[0]
		queue = queue[1:]
		parsedPage, parseErr := url.Parse(pageURL)
		if parseErr != nil || seenPages[pageURL] || parsedPage.Hostname() != start.Hostname() {
			continue
		}
		seenPages[pageURL] = true
		pageReq, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
		if reqErr != nil {
			return nil, pagesScanned, errors.New("could not create uploader listing request")
		}
		pageReq.Header.Set("User-Agent", "uploader-jp-archive/1.0 (preservation listing preview)")
		pageReq.Header.Set("Accept", "text/html,application/xhtml+xml")
		resp, doErr := client.Do(pageReq)
		if doErr != nil {
			return nil, pagesScanned, fmt.Errorf("uploader.jp の一覧を読み込めませんでした: %w", doErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, pagesScanned, fmt.Errorf("uploader.jp の一覧が HTTP %d を返しました", resp.StatusCode)
		}
		contentType := strings.ToLower(resp.Header.Get("Content-Type"))
		if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
			resp.Body.Close()
			return nil, pagesScanned, errors.New("指定URLはHTMLの一覧ページではありません。個別ファイルURLなら直接取得を選んでください")
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxUploaderHTMLBytes+1))
		resp.Body.Close()
		if readErr != nil || len(body) > maxUploaderHTMLBytes {
			return nil, pagesScanned, errors.New("一覧ページが大きすぎるか、読み込めませんでした")
		}
		doc, htmlErr := xhtml.Parse(bytes.NewReader(body))
		if htmlErr != nil {
			return nil, pagesScanned, errors.New("一覧ページを解析できませんでした")
		}
		pageText := strings.ToLower(htmlText(doc))
		baseURL := resp.Request.URL
		if hasPasswordInput(doc) {
			if password == "" {
				return nil, pagesScanned, errors.New("一覧ページにパスワードが必要です。申請時または管理画面でパスワードを設定してください")
			}
			baseURL, body, err = submitUploaderListingPassword(ctx, client, baseURL, doc, password)
			if err != nil {
				return nil, pagesScanned, err
			}
			doc, htmlErr = xhtml.Parse(bytes.NewReader(body))
			if htmlErr != nil {
				return nil, pagesScanned, errors.New("パスワード送信後の一覧ページを解析できませんでした")
			}
			pageText = strings.ToLower(htmlText(doc))
			if hasPasswordInput(doc) {
				return nil, pagesScanned, errors.New("パスワード認証に失敗したか、追加操作が必要です。CAPTCHA等は自動処理しません")
			}
		}
		if hasCaptcha(doc, pageText) || strings.Contains(pageText, "ログインしてください") {
			return nil, pagesScanned, errors.New("一覧ページに追加認証が必要です。CAPTCHA等は自動処理しません")
		}
		pagesScanned++
		var walk func(*xhtml.Node)
		walk = func(n *xhtml.Node) {
			if n.Type == xhtml.ElementNode && n.Data == "a" {
				href := strings.TrimSpace(attr(n, "href"))
				if href != "" {
					resolved, e := baseURL.Parse(href)
					if e == nil {
						resolved.Fragment = ""
						if normalized, e := validateUploaderURL(resolved.String()); e == nil {
							if isUploaderDownloadPage(normalized) && !seenCandidates[normalized] {
								seenCandidates[normalized] = true
								label := strings.TrimSpace(htmlText(n))
								if label == "" || len(label) > 300 {
									label = safeSourceFilename(path.Base(resolved.Path))
								}
								items = append(items, uploaderListingCandidate{URL: normalized, Label: label})
							}
							if isUploaderPaginationLink(normalized, attr(n, "rel"), strings.TrimSpace(htmlText(n)), baseURL) && len(queue)+pagesScanned < pageLimit && !seenPages[normalized] {
								queue = append(queue, normalized)
							}
						}
					}
				}
			}
			for child := n.FirstChild; child != nil && len(items) < candidateLimit; child = child.NextSibling {
				walk(child)
			}
		}
		walk(doc)
	}
	return items, pagesScanned, nil
}

func isUploaderDownloadPage(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	p := strings.ToLower(u.Path)
	if strings.Contains(p, "/download/") {
		return true
	}
	host := strings.ToLower(u.Hostname())
	return strings.HasPrefix(host, "dl") && strings.Contains(host, "getuploader.com") && strings.Contains(p, "/g/")
}

func isUploaderPaginationLink(raw, rel, label string, base *url.URL) bool {
	u, err := url.Parse(raw)
	if err != nil || base == nil || u.Hostname() != base.Hostname() || u.String() == base.String() || isUploaderDownloadPage(raw) {
		return false
	}
	rel = strings.ToLower(rel)
	label = strings.ToLower(strings.TrimSpace(label))
	if strings.Contains(rel, "next") || strings.Contains(label, "次へ") || strings.Contains(label, "次のページ") || strings.Contains(label, "next") || strings.Contains(label, "older") || label == ">" || label == "次" || label == "›" || label == "»" {
		return true
	}
	q := u.Query()
	for key := range q {
		key = strings.ToLower(key)
		if (key == "page" || key == "p" || key == "offset" || key == "start") && q.Get(key) != "" {
			return true
		}
	}
	return strings.Contains(strings.ToLower(u.Path), "/page/")
}
