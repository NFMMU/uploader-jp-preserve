package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"gorm.io/gorm"
)

var errDuplicateSourceURL = errors.New("source URL has already been requested or catalogued")

const (
	maxURLCollectionSeeds      = 10
	maxURLCollectionCandidates = 100
	maxURLCollectionChecks     = 20
	maxURLCollectionPages      = 20
)

func uploaderCandidateHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if _, err := validateUploaderURL(req.URL.String()); err != nil {
				return errors.New("redirect target is not an uploader URL")
			}
			return nil
		},
	}
}

func checkUploaderCandidate(ctx context.Context, raw string, client *http.Client) (int, error) {
	validated, err := validateUploaderURL(raw)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validated, nil)
	if err != nil {
		return 0, fmt.Errorf("確認リクエストを作成できません: %w", err)
	}
	req.Header.Set("User-Agent", "uploader-jp-archive/1.0 (preservation link check)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/octet-stream,*/*;q=0.8")
	req.Header.Set("Range", "bytes=0-0")
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("接続エラー: %w", err)
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

func checkUploaderListingCandidates(ctx context.Context, found []uploaderListingCandidate, seen map[string]bool, checksPerformed int, client *http.Client) ([]uploaderListingCandidate, int, int, []string, bool) {
	candidates := make([]uploaderListingCandidate, 0, len(found))
	duplicates := 0
	var checkErrors []string
	for _, item := range found {
		normalized, err := normalizeUploaderURL(item.URL)
		if err != nil {
			continue
		}
		if seen[normalized] {
			duplicates++
			continue
		}
		if checksPerformed >= maxURLCollectionChecks {
			return candidates, checksPerformed, duplicates, checkErrors, true
		}
		seen[normalized] = true
		checksPerformed++
		status, checkErr := checkUploaderCandidate(ctx, normalized, client)
		if checkErr != nil {
			checkErrors = appendCapped(checkErrors, fmt.Sprintf("%s: リンク先を確認できませんでした: %s", normalized, checkErr))
			continue
		}
		if status < 200 || status >= 300 {
			checkErrors = appendCapped(checkErrors, fmt.Sprintf("%s: リンク先がHTTP %dを返したためスキップしました", normalized, status))
			continue
		}
		item.URL = normalized
		candidates = append(candidates, item)
	}
	return candidates, checksPerformed, duplicates, checkErrors, false
}

type collectedSourceURL struct {
	URL       string `json:"url"`
	Label     string `json:"label"`
	SeedURL   string `json:"seed_url"`
	Status    string `json:"status"`
	RequestID string `json:"request_id,omitempty"`
}

// normalizeUploaderURL removes URL fragments and normalizes the host/default port.
// Query parameters are preserved because uploader URLs may use them as download tokens.
func normalizeUploaderURL(raw string) (string, error) {
	validated, err := validateUploaderURL(raw)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(validated)
	if err != nil {
		return "", errInvalidURL
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if u.Port() != "" && u.Port() != "443" {
		return "", errInvalidURL
	}
	u.Host = host
	u.Fragment = ""
	u.RawFragment = ""
	return u.String(), nil
}

// collectUploaderURLs scans bounded public web pages for uploader.jp/getuploader.com links.
// It creates preservation requests for newly discovered URLs but never downloads files
// or enables public distribution. Authentication/CAPTCHA pages are not bypassed.
func (s *Server) collectUploaderURLs(c *gin.Context) {
	var req struct {
		SeedURLs    []string `json:"seed_urls" binding:"required,min=1,max=10,dive,required,max=4096"`
		AcceptTerms bool     `json:"accept_terms"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.AcceptTerms {
		c.JSON(http.StatusBadRequest, gin.H{"error": "1〜10件の開始URLと、対象サイトの利用規約を確認したことが必要です。"})
		return
	}

	seeds := make([]string, 0, len(req.SeedURLs))
	seenSeeds := map[string]bool{}
	for _, raw := range req.SeedURLs {
		seed, err := normalizePublicCrawlURL(strings.TrimSpace(raw))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "開始URLは公開された HTTPS のWebページに限ります。"})
			return
		}
		if !seenSeeds[seed] {
			seenSeeds[seed] = true
			seeds = append(seeds, seed)
		}
	}

	candidates := make([]uploaderListingCandidate, 0, 64)
	candidateSeeds := make(map[string]string)
	seenCandidates := make(map[string]bool)
	pagesScanned := 0
	duplicatesInRun := 0
	checksPerformed := 0
	checksTruncated := false
	var scanErrors []string
	candidateClient := uploaderCandidateHTTPClient()
	for _, seed := range seeds {
		if pagesScanned >= maxURLCollectionPages || len(candidates) >= maxURLCollectionCandidates || checksTruncated {
			break
		}
		remainingPages := maxURLCollectionPages - pagesScanned
		remainingCandidates := maxURLCollectionCandidates - len(candidates)
		found, pages, errorsForSeed := discoverUploaderLinksFromWeb(c.Request.Context(), seed, remainingPages, remainingCandidates)
		pagesScanned += pages
		for _, scanErr := range errorsForSeed {
			scanErrors = appendCapped(scanErrors, fmt.Sprintf("%s: %s", seed, scanErr))
		}
		checked, totalChecks, duplicates, checkErrors, truncated := checkUploaderListingCandidates(
			c.Request.Context(), found, seenCandidates, checksPerformed, candidateClient,
		)
		checksPerformed = totalChecks
		duplicatesInRun += duplicates
		checksTruncated = truncated
		for _, checkErr := range checkErrors {
			scanErrors = appendCapped(scanErrors, fmt.Sprintf("%s: %s", seed, checkErr))
		}
		for _, item := range checked {
			candidates = append(candidates, item)
			candidateSeeds[item.URL] = seed
			if len(candidates) >= maxURLCollectionCandidates {
				break
			}
		}
		if len(candidates) >= maxURLCollectionCandidates || checksTruncated {
			break
		}
	}

	created, duplicates, failed := 0, duplicatesInRun, 0
	results := make([]collectedSourceURL, 0, len(candidates))
	for _, item := range candidates {
		if len(results) >= maxURLCollectionCandidates {
			break
		}
		seed := candidateSeeds[item.URL]
		title := candidateURLTitle(item.URL)
		label := strings.TrimSpace(item.Label)
		if label != "" && !isGenericDownloadLabel(label) {
			title = label
		}
		if len(title) > 500 {
			title = title[:500]
		}
		requestID := uuid.NewString()
		duplicate := false
		var requestCreated bool
		err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			reserve := tx.Exec(`INSERT INTO source_url_registry (normalized_url, source_url, origin, created_at, updated_at)
				VALUES (?, ?, 'auto_collection', now(), now()) ON CONFLICT (normalized_url) DO NOTHING`, item.URL, item.URL)
			if reserve.Error != nil {
				return reserve.Error
			}
			if reserve.RowsAffected == 0 {
				duplicate = true
				return nil
			}

			var existingFile model.FileObject
			fileErr := tx.Where("split_part(trim(source_url), '#', 1) = ? AND deleted_at IS NULL", item.URL).First(&existingFile).Error
			if fileErr == nil {
				duplicate = true
				return tx.Exec("UPDATE source_url_registry SET origin = 'catalogued', updated_at = now() WHERE normalized_url = ?", item.URL).Error
			}
			if !errors.Is(fileErr, gorm.ErrRecordNotFound) {
				return fileErr
			}

			var existingRequest model.PublicRequest
			requestErr := tx.Where("request_type = 'preservation' AND split_part(trim(source_url), '#', 1) = ?", item.URL).Order("created_at ASC").First(&existingRequest).Error
			if requestErr == nil {
				duplicate = true
				return tx.Exec("UPDATE source_url_registry SET public_request_id = ?, origin = 'catalogued', updated_at = now() WHERE normalized_url = ?", existingRequest.ID, item.URL).Error
			}
			if !errors.Is(requestErr, gorm.ErrRecordNotFound) {
				return requestErr
			}

			preserveRequest := model.PublicRequest{
				ID: requestID, RequestType: "preservation", Status: "pending", Title: stringPtr(title),
				SourceURL: stringPtr(item.URL), Details: fmt.Sprintf("Web上から自動収集した候補です。開始URL: %s。内容・権利情報を確認し、必要な場合のみ保全を開始してください。自動収集ではダウンロードや一般公開を行いません。", seed),
			}
			if err := tx.Create(&preserveRequest).Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE source_url_registry SET public_request_id = ?, updated_at = now() WHERE normalized_url = ?", preserveRequest.ID, item.URL).Error; err != nil {
				return err
			}
			requestCreated = true
			return nil
		})
		if err != nil {
			failed++
			results = append(results, collectedSourceURL{URL: item.URL, Label: title, SeedURL: seed, Status: "failed"})
			continue
		}
		status := "duplicate"
		if requestCreated {
			created++
			status = "request_created"
			results = append(results, collectedSourceURL{URL: item.URL, Label: title, SeedURL: seed, Status: status, RequestID: requestID})
		} else {
			duplicates++
			if !duplicate {
				status = "skipped"
			}
			results = append(results, collectedSourceURL{URL: item.URL, Label: title, SeedURL: seed, Status: status})
		}
	}

	actor, _ := currentUser(c)
	s.audit(c, &actor, "url_collection", "batch", "url_collection.scan", "success", stringPtr(fmt.Sprintf("seeds=%d pages=%d candidates=%d created=%d duplicates=%d failed=%d", len(seeds), pagesScanned, len(candidates), created, duplicates, failed)))
	c.JSON(http.StatusOK, gin.H{
		"seed_count": len(seeds), "pages_scanned": pagesScanned, "candidate_count": len(candidates),
		"requests_created": created, "duplicates_discarded": duplicates, "failed": failed,
		"truncated": len(candidates) >= maxURLCollectionCandidates || checksTruncated, "errors": scanErrors, "items": results,
		"message": "公開サイトから検出した新しいURLは申請・保全レビューに登録しました。権利確認後に取得を開始できます。",
	})
}

func candidateURLTitle(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "保全候補 " + raw
	}
	name := safeSourceFilename(path.Base(u.Path))
	if name == "" || name == "." || name == "/" {
		name = u.Hostname() + u.Path
	}
	if len(name) > 500 {
		name = name[:500]
	}
	return name
}

func isGenericDownloadLabel(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "download", "download now", "ダウンロード", "こちら", "詳細", "file", "ファイル":
		return true
	default:
		return false
	}
}
