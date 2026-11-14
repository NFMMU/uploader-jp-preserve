package oidc

import (
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"github.com/nfmmu/uploader-archive/internal/gen/query"
)

type OidcRequestMode string

func AuthorizationRequest(c *gin.Context, baseURL string) {
	ctx := c.Request.Context()

	mode := c.Query("mode")

	var frontPath string
	switch mode {
	case "authenticate":
		frontPath = "login"
		break
	case "link":
		frontPath = "preference"
		break
	default:
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, "login"))
		return
	}

	providerID := c.Param("id")
	if providerID == "" {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return
	}

	providerQuery := query.OidcProvider
	providerDbData, err := providerQuery.WithContext(ctx).Where(providerQuery.ID.Eq(providerID)).First()
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return
	}
	if providerDbData == nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return
	}

	providerData, err := OidcDiscovery(ctx, providerDbData.DiscoveryEndpoint)
	if err != nil || providerData == nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return
	}

	provider, err := oidc.NewProvider(ctx, providerData.Issuer)
	if err != nil || provider == nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return
	}

	config := oauth2.Config{
		ClientID:    providerDbData.ClientID,
		Endpoint:    provider.Endpoint(),
		RedirectURL: fmt.Sprintf("%s/api/v1/auth/oidc/callback/%s", baseURL, providerID),
		Scopes:      []string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail},
	}

	state, err := randString(16)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return
	}
	nonce, err := randString(16)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
		return
	}
	setCallbackCookie(c.Writer, c.Request, "state", state)
	setCallbackCookie(c.Writer, c.Request, "nonce", nonce)
	setCallbackCookie(c.Writer, c.Request, "oidc_mode", string(mode))
	c.Redirect(http.StatusFound, config.AuthCodeURL(state, oidc.Nonce(nonce)))
}
