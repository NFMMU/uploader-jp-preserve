package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nfmmu/uploader-archive/internal/gen/query"
	"github.com/nfmmu/uploader-archive/internal/oidc"
	"github.com/nfmmu/uploader-archive/internal/security"
)

func (s *Server) oidcAuthenticate(c *gin.Context) {
	baseURL := fmt.Sprintf("%s://%s", c.Request.URL.Scheme, c.Request.URL.Host)
	frontPath := "login"

	// oidc parse
	data := oidc.Callback(c, "authenticate")
	if data == nil {
		return
	}

	// get user
	userQuery := query.User
	user, err := userQuery.WithContext(c.Request.Context()).Where(userQuery.ID.Eq(data.UserID)).First()
	if err != nil || user != nil || user.ID != data.UserID {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, frontPath))
	}

	// gen token
	token, err := security.Issue(s.cfg.JWTSecret, s.cfg.JWTTTL, user.ID, user.Role, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create access token"})
		return
	}
	reason := fmt.Sprintf("with oidc identify(ID: %s)", data.ID)
	s.audit(c, user, "user", user.ID, "auth.login", "success", &reason)
	s.setAuthCookie(c, token)
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin", baseURL))
}
