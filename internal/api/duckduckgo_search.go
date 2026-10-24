package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	xhtml "golang.org/x/net/html"
)

const (
	maxDuckDuckGoQueries         = 5
	maxDuckDuckGoSeeds           = 100
	maxDuckDuckGoHTMLBytes       = 4 << 20
	maxDuckDuckGoResultsPerQuery = 20
)

type duckDuckGoResult struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// searchUploaderURLs uses DuckDuckGo's public HTML results to find public pages
// likely to contain uploader.jp/getuploader.com links. Search results are only
// used as crawl seeds; the crawler still applies its HTTPS, SSRF and page limits.
func (s *Server) searchUploaderURLs(c *gin.Context) {
	var req struct {
		SearchTerms []string `json:"search_terms" binding:"max=5,dive,max=100"`
		AcceptTerms bool     `json:"accept_terms"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.AcceptTerms {
		c.JSON(http.StatusBadRequest, gin.H{"error": "検索対象サイトの利用条件を確認したことが必要です。検索語は任意で最大5件です。"})
		return
	}
	terms := make([]string, 0, maxDuckDuckGoQueries)
	seenTerms := map[string]bool{}
	for _, term := range req.SearchTerms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		if len(term) > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "検索語は100文字以内で指定してください。"})
			return
		}
		key := strings.ToLower(term)
		if !seenTerms[key] {
			seenTerms[key] = true
			terms = append(terms, term)
		}
	}
	if len(terms) == 0 {
		terms = []string{"MMD モデル", "MMD モーション", "MMD エフェクト", "MMD ステージ", "MMD 配布"}
	}
	if len(terms) > maxDuckDuckGoQueries {
		terms = terms[:maxDuckDuckGoQueries]
	}

	seeds := make([]duckDuckGoResult, 0, maxDuckDuckGoSeeds)
	seenURLs := map[string]bool{}
	var errorsForSearch []string
	queries := make([]string, 0, len(terms))
	for _, term := range terms {
		if len(seeds) >= maxDuckDuckGoSeeds {
			break
		}
		query := fmt.Sprintf(`("uploader.jp" OR "getuploader.com") %s`, term)
		queries = append(queries, query)
		if len(queries) > 1 {
			select {
			case <-c.Request.Context().Done():
				c.JSON(http.StatusRequestTimeout, gin.H{"error": "検索がキャンセルされました。"})
				return
			case <-time.After(350 * time.Millisecond):
			}
		}
		found, err := searchDuckDuckGoHTML(c.Request.Context(), query, maxDuckDuckGoResultsPerQuery)
		if err != nil {
			errorsForSearch = appendCapped(errorsForSearch, fmt.Sprintf("DuckDuckGo検索 (%s): %s", term, err.Error()))
			continue
		}
		for _, item := range found {
			normalized, normErr := normalizePublicCrawlURL(item.URL)
			if normErr != nil || seenURLs[normalized] {
				continue
			}
			seenURLs[normalized] = true
			item.URL = normalized
			if strings.TrimSpace(item.Title) == "" {
				item.Title = normalized
			}
			seeds = append(seeds, item)
			if len(seeds) >= maxDuckDuckGoSeeds {
				break
			}
		}
	}
	seedURLs := make([]string, 0, len(seeds))
	for _, seed := range seeds {
		seedURLs = append(seedURLs, seed.URL)
	}
	c.JSON(http.StatusOK, gin.H{
		"queries": queries, "results": seeds, "seed_urls": seedURLs,
		"seed_count": len(seedURLs), "errors": errorsForSearch,
		"message": "DuckDuckGoの検索結果から公開HTTPSページを抽出しました。候補ページを巡回し、ダウンロードURLを探します。",
	})
}

func searchDuckDuckGoHTML(ctx context.Context, query string, limit int) ([]duckDuckGoResult, error) {
	endpoint, _ := url.Parse("https://html.duckduckgo.com/html/")
	values := endpoint.Query()
	values.Set("q", query)
	endpoint.RawQuery = values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("検索要求を作成できません")
	}
	req.Header.Set("User-Agent", "uploader-jp-archive/1.0 (+link discovery; contact uploaderjp-archive@udonya.moe)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := publicCrawlHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("検索サービスに接続できませんでした")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("検索サービスがHTTP %dを返しました", resp.StatusCode)
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
		return nil, fmt.Errorf("検索サービスがHTML以外を返しました")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDuckDuckGoHTMLBytes+1))
	if err != nil || len(body) > maxDuckDuckGoHTMLBytes {
		return nil, fmt.Errorf("検索結果がサイズ上限を超えています")
	}
	return parseDuckDuckGoHTML(body, limit), nil
}

func parseDuckDuckGoHTML(body []byte, limit int) []duckDuckGoResult {
	if limit < 1 {
		return nil
	}
	doc, err := xhtml.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}
	results := make([]duckDuckGoResult, 0, limit)
	seen := map[string]bool{}
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if len(results) >= limit {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "a" && hasClass(n, "result__a") {
			href := strings.TrimSpace(attr(n, "href"))
			if target, ok := unwrapDuckDuckGoURL(href); ok {
				if normalized, normErr := normalizePublicCrawlURL(target); normErr == nil && !seen[normalized] {
					seen[normalized] = true
					title := strings.TrimSpace(htmlText(n))
					results = append(results, duckDuckGoResult{Title: title, URL: normalized})
				}
			}
		}
		for child := n.FirstChild; child != nil && len(results) < limit; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return results
}

func unwrapDuckDuckGoURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if u.Hostname() == "duckduckgo.com" || strings.HasSuffix(strings.ToLower(u.Hostname()), ".duckduckgo.com") {
		if target := u.Query().Get("uddg"); target != "" {
			return target, true
		}
		return "", false
	}
	if u.Scheme == "" || u.Hostname() == "" {
		return "", false
	}
	return u.String(), true
}

func hasClass(n *xhtml.Node, wanted string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, class := range strings.Fields(a.Val) {
				if class == wanted {
					return true
				}
			}
		}
	}
	return false
}
