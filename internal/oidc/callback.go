package oidc

import (
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"github.com/nfmmu/uploader-archive/internal/gen/query"
	"github.com/nfmmu/uploader-archive/internal/hooks"
)

type OidcClaims struct {
	Issuer            string  `json:"issuer"`
	Subject           string  `json:"subject"`
	Email             string  `json:"email"`
	EmailVerified     bool    `json:"email_verified"`
	Picture           *string `json:"picture" binding:"omitempty"`
	PreferredUsername *string `json:"preferred_username" binding:"omitempty"`
}

func Callback(c *gin.Context, baseURL, mode string, decryptSecret func(*string) (string, error)) *model.OidcIdentity {
	ctx := c.Request.Context()
	r := c.Request

	// モード検証
	var frontPath string
	switch mode {
	case "authenticate":
		frontPath = "login"
	case "link":
		frontPath = "preference"
	default:
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, "login"))
		return nil
	}

	// プロバイダ設定の取得
	providerID := c.Param("id")
	if providerID == "" {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}

	providerQuery := query.OidcProvider
	providerDbData, err := providerQuery.WithContext(ctx).Where(providerQuery.ID.Eq(providerID)).First()
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}
	if providerDbData == nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}

	// OIDCディスカバリ
	providerData, err := OidcDiscovery(ctx, providerDbData.DiscoveryEndpoint)
	if err != nil || providerData == nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}

	// プロバイダ作成
	provider, err := oidc.NewProvider(ctx, providerData.Issuer)
	if err != nil || provider == nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}

	// config作成
	oidcConfig := &oidc.Config{
		ClientID: providerDbData.ClientID,
	}
	verifier := provider.Verifier(oidcConfig)

	config := oauth2.Config{
		ClientID:    providerDbData.ClientID,
		Endpoint:    provider.Endpoint(),
		RedirectURL: fmt.Sprintf("%s/api/v1/auth/oidc/callback/%s", baseURL, providerID),
		Scopes:      []string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail},
	}

	// state検証
	state, err := r.Cookie("state")
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}
	if r.URL.Query().Get("state") != state.Value {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}

	clientSecret, err := decryptSecret(&providerDbData.ClientSecret)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}
	config.ClientSecret = clientSecret

	// token取得
	oauth2Token, err := config.Exchange(ctx, c.Query("code"))
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}

	// ID token検証
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}

	// nonce検証
	nonce, err := r.Cookie("nonce")
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}
	if idToken.Nonce != nonce.Value {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}

	// AccessToken無効化
	oauth2Token.AccessToken = "*REDACTED*"

	idTokenData := OidcClaims{}
	if err = idToken.Claims(&idTokenData); err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}

	if !idTokenData.EmailVerified {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
	}

	// モードに合わせた処理
	switch mode {
	case "authenticate":
		return getOidcIdentity(c, idTokenData, providerID, baseURL, frontPath)
	case "link":
		return linkOidcIdentity(c, idTokenData, providerID, baseURL, frontPath)
	default:
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}
}

func linkOidcIdentity(c *gin.Context, data OidcClaims, providerID, baseURL, frontPath string) *model.OidcIdentity {
	user, ok := hooks.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}
	oidcIdentityQuery := query.OidcIdentity
	err := oidcIdentityQuery.WithContext(c.Request.Context()).Create(&model.OidcIdentity{
		ProviderID:        providerID,
		Issuer:            data.Issuer,
		Subject:           data.Subject,
		Email:             data.Email,
		UserID:            user.ID,
		PreferredUsername: data.PreferredUsername,
		Picture:           data.Picture,
	})
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}
	resp, err := oidcIdentityQuery.WithContext(c.Request.Context()).Where(
		oidcIdentityQuery.UserID.Eq(user.ID),
		oidcIdentityQuery.Subject.Eq(data.Subject),
	).First()
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}
	return resp
}

func getOidcIdentity(c *gin.Context, data OidcClaims, providerID, baseURL, frontPath string) *model.OidcIdentity {
	oidcIdentityQuery := query.OidcIdentity
	resp, err := oidcIdentityQuery.WithContext(c.Request.Context()).Where(
		oidcIdentityQuery.ProviderID.Eq(providerID),
		oidcIdentityQuery.Subject.Eq(data.Subject),
	).First()
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return nil
	}
	return resp
}
