package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"gorm.io/gorm"
)

var errDuplicateSourceURL = errors.New("source URL has already been requested or catalogued")

const (
	maxURLCollectionSeeds      = 100
	maxURLCollectionCandidates = 300
	maxURLCollectionPages      = 100
)

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
	var scanErrors []string
	for _, seed := range seeds {
		if pagesScanned >= maxURLCollectionPages || len(candidates) >= maxURLCollectionCandidates {
			break
		}
		remainingPages := maxURLCollectionPages - pagesScanned
		remainingCandidates := maxURLCollectionCandidates - len(candidates)
		found, pages, errorsForSeed := discoverUploaderLinksFromWeb(c.Request.Context(), seed, remainingPages, remainingCandidates)
		pagesScanned += pages
		for _, scanErr := range errorsForSeed {
			scanErrors = appendCapped(scanErrors, fmt.Sprintf("%s: %s", seed, scanErr))
		}
		for _, item := range found {
			normalized, normErr := normalizeUploaderURL(item.URL)
			if normErr != nil {
				continue
			}
			if seenCandidates[normalized] {
				duplicatesInRun++
				continue
			}
			seenCandidates[normalized] = true
			item.URL = normalized
			candidates = append(candidates, item)
			candidateSeeds[normalized] = seed
			if len(candidates) >= maxURLCollectionCandidates {
				break
			}
		}
		if len(candidates) >= maxURLCollectionCandidates {
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
		"truncated": len(candidates) >= maxURLCollectionCandidates, "errors": scanErrors, "items": results,
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
