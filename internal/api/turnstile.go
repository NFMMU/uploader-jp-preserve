package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const turnstileSiteVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// publicConfig exposes only the public site key. The secret key is never returned.
func (s *Server) publicConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"turnstile_enabled":  s.cfg.TurnstileSiteKey != "" && s.cfg.TurnstileSecretKey != "",
		"turnstile_site_key": s.cfg.TurnstileSiteKey,
	})
}

type turnstileVerifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// verifyTurnstile validates a token server-side. In local development, Turnstile
// may be omitted; production configuration rejects missing keys at startup.
func (s *Server) verifyTurnstile(c *gin.Context, token string) bool {
	if s.cfg.TurnstileSecretKey == "" {
		if s.cfg.Environment != "production" {
			return true
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "CAPTCHA 認証が設定されていません。運営者にお問い合わせください。"})
		return false
	}

	token = strings.TrimSpace(token)
	if len(token) > 4096 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CAPTCHA 認証情報が正しくありません。再度認証してください。"})
		return false
	}
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "送信前に Cloudflare Turnstile の認証を完了してください。"})
		return false
	}

	form := url.Values{}
	form.Set("secret", s.cfg.TurnstileSecretKey)
	form.Set("response", token)
	if ip := strings.TrimSpace(c.ClientIP()); ip != "" {
		form.Set("remoteip", ip)
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileSiteVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "CAPTCHA 認証を確認できませんでした。もう一度お試しください。"})
		return false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "CAPTCHA 認証サービスに接続できませんでした。時間をおいて再度お試しください。"})
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "CAPTCHA 認証サービスから正常な応答を得られませんでした。"})
		return false
	}
	var result turnstileVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || !result.Success {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cloudflare Turnstile の認証に失敗しました。再度認証してから送信してください。"})
		return false
	}
	return true
}
