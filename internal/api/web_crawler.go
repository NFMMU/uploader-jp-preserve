package api

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
)

const maxCrawlHTMLBytes = 8 << 20

// Only HTTPS public pages are crawled. The crawler stays on each seed hostname
// and extracts uploader.jp/getuploader.com download URLs without fetching them.
func normalizePublicCrawlURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err == nil {
		u.Scheme = strings.ToLower(u.Scheme)
	}
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" {
		return "", errInvalidURL
	}
	if u.Port() != "" && u.Port() != "443" {
		return "", errInvalidURL
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".test") || strings.HasSuffix(host, ".invalid") {
		return "", errInvalidURL
	}
	if ip := net.ParseIP(host); ip != nil && !isPublicCrawlIP(ip) {
		return "", errInvalidURL
	}
	u.Scheme = "https"
	if strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	} else {
		u.Host = host
	}
	u.Fragment = ""
	u.RawFragment = ""
	return u.String(), nil
}

var blockedCrawlNetworks = func() []*net.IPNet {
	blocks := []string{
		"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24",
		"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
		"2001:db8::/32", "2001:10::/28",
	}
	out := make([]*net.IPNet, 0, len(blocks))
	for _, block := range blocks {
		_, network, err := net.ParseCIDR(block)
		if err == nil {
			out = append(out, network)
		}
	}
	return out
}()

func isPublicCrawlIP(ip net.IP) bool {
	if ip != nil && ip.To4() != nil {
		ip = ip.To4()
	}
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	for _, block := range blockedCrawlNetworks {
		if block.Contains(ip) {
			return false
		}
	}
	return true
}

func dialPublicCrawlHost(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" {
		return nil, fmt.Errorf("crawler only allows public HTTPS endpoints")
	}
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		resolved, resolveErr := net.DefaultResolver.LookupIPAddr(ctx, host)
		if resolveErr != nil {
			return nil, resolveErr
		}
		for _, item := range resolved {
			ips = append(ips, item.IP)
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("hostname has no IP addresses")
	}
	for _, ip := range ips {
		if !isPublicCrawlIP(ip) {
			return nil, fmt.Errorf("crawler refused a non-public IP address")
		}
	}
	dialer := &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, ip := range ips {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	return nil, lastErr
}

func publicCrawlHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy: nil, DialContext: dialPublicCrawlHost,
			TLSHandshakeTimeout: 8 * time.Second, ResponseHeaderTimeout: 10 * time.Second,
			IdleConnTimeout: 15 * time.Second, MaxIdleConns: 10, MaxIdleConnsPerHost: 2,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			if _, err := normalizePublicCrawlURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect target is not an allowed public HTTPS URL")
			}
			return nil
		},
	}
}

type webCrawlPage struct {
	URL   string
	Depth int
}

// discoverUploaderLinksFromWeb scans a bounded number of pages on one public
// website and extracts only uploader download links. It never downloads files.
func discoverUploaderLinksFromWeb(ctx context.Context, seedURL string, pageLimit, candidateLimit int) ([]uploaderListingCandidate, int, []string) {
	if pageLimit < 1 || candidateLimit < 1 {
		return []uploaderListingCandidate{}, 0, nil
	}
	seed, err := url.Parse(seedURL)
	if err != nil {
		return nil, 0, []string{"開始URLが不正です"}
	}
	if uploaderURL, err := normalizeUploaderURL(seedURL); err == nil && isUploaderDownloadPage(uploaderURL) {
		return []uploaderListingCandidate{{URL: uploaderURL, Label: candidateURLTitle(uploaderURL)}}, 0, nil
	}
	client := publicCrawlHTTPClient()
	queue := []webCrawlPage{{URL: seed.String(), Depth: 0}}
	seenPages := map[string]bool{}
	seenCandidates := map[string]bool{}
	items := make([]uploaderListingCandidate, 0, 32)
	var pageErrors []string
	pagesScanned := 0

	for len(queue) > 0 && pagesScanned < pageLimit && len(items) < candidateLimit {
		page := queue[0]
		queue = queue[1:]
		pageURL, normErr := normalizePublicCrawlURL(page.URL)
		if normErr != nil || seenPages[pageURL] {
			continue
		}
		parsedPage, parseErr := url.Parse(pageURL)
		if parseErr != nil || !strings.EqualFold(parsedPage.Hostname(), seed.Hostname()) {
			continue
		}
		seenPages[pageURL] = true
		if pagesScanned > 0 {
			select {
			case <-ctx.Done():
				return items, pagesScanned, append(pageErrors, "収集がキャンセルされました")
			case <-time.After(250 * time.Millisecond):
			}
		}
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
		if reqErr != nil {
			pageErrors = appendCapped(pageErrors, pageURL+": リクエストを作成できません")
			continue
		}
		req.Header.Set("User-Agent", "uploader-jp-archive/1.0 (+preservation link discovery; contact uploaderjp-archive@udonya.moe)")
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		pagesScanned++ // Count attempts as well, so failed pages cannot bypass the crawl limit.
		resp, fetchErr := client.Do(req)
		if fetchErr != nil {
			pageErrors = appendCapped(pageErrors, pageURL+": 公開ページを読み込めませんでした")
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			status := resp.StatusCode
			resp.Body.Close()
			pageErrors = appendCapped(pageErrors, fmt.Sprintf("%s: HTTP %d", pageURL, status))
			continue
		}
		contentType := strings.ToLower(resp.Header.Get("Content-Type"))
		if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
			resp.Body.Close()
			pageErrors = appendCapped(pageErrors, pageURL+": HTMLページではないためスキップしました")
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxCrawlHTMLBytes+1))
		finalURL := resp.Request.URL
		resp.Body.Close()
		if readErr != nil || len(body) > maxCrawlHTMLBytes {
			pageErrors = appendCapped(pageErrors, pageURL+": ページサイズが上限を超えています")
			continue
		}
		doc, htmlErr := xhtml.Parse(strings.NewReader(string(body)))
		if htmlErr != nil {
			pageErrors = appendCapped(pageErrors, pageURL+": HTMLを解析できません")
			continue
		}
		pageText := strings.ToLower(htmlText(doc))
		if hasPasswordInput(doc) || hasCaptcha(doc, pageText) || strings.Contains(pageText, "ログインしてください") {
			pageErrors = appendCapped(pageErrors, pageURL+": ログイン・パスワード・CAPTCHAが必要なためスキップしました")
			continue
		}
		baseURL := finalURL
		if baseURL == nil {
			baseURL = parsedPage
		}
		var walk func(*xhtml.Node)
		walk = func(n *xhtml.Node) {
			if n.Type == xhtml.ElementNode && n.Data == "a" {
				href := strings.TrimSpace(attr(n, "href"))
				if href != "" {
					resolved, resolveErr := baseURL.Parse(href)
					if resolveErr == nil {
						resolved.Fragment = ""
						if uploaderURL, e := normalizeUploaderURL(resolved.String()); e == nil && isUploaderDownloadPage(uploaderURL) {
							if !seenCandidates[uploaderURL] {
								seenCandidates[uploaderURL] = true
								label := strings.TrimSpace(htmlText(n))
								if label == "" || len(label) > 300 || isGenericDownloadLabel(label) {
									label = candidateURLTitle(uploaderURL)
								}
								items = append(items, uploaderListingCandidate{URL: uploaderURL, Label: label})
							}
						} else if page.Depth < 2 && len(queue)+pagesScanned < pageLimit {
							if crawlURL, e := normalizePublicCrawlURL(resolved.String()); e == nil {
								crawlParsed, _ := url.Parse(crawlURL)
								if crawlParsed != nil && strings.EqualFold(crawlParsed.Hostname(), seed.Hostname()) && isLikelyHTMLPage(crawlParsed.Path) && !seenPages[crawlURL] {
									alreadyQueued := false
									for _, queued := range queue {
										if queued.URL == crawlURL {
											alreadyQueued = true
											break
										}
									}
									if !alreadyQueued {
										queue = append(queue, webCrawlPage{URL: crawlURL, Depth: page.Depth + 1})
									}
								}
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
	return items, pagesScanned, pageErrors
}

func appendCapped(items []string, value string) []string {
	if len(items) >= 10 {
		return items
	}
	return append(items, value)
}

func isLikelyHTMLPage(pathname string) bool {
	ext := strings.ToLower(path.Ext(pathname))
	switch ext {
	case ".html", ".htm", ".php", ".asp", ".aspx", ".jsp", ".cgi", ".shtml", ".xhtml":
		return true
	case "":
		return true
	default:
		return false
	}
}
