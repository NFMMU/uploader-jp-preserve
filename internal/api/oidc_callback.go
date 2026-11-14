package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) oidcCallback(c *gin.Context) {
	mode, err := c.Request.Cookie("oidc_mode")
	baseURL := fmt.Sprintf("%s://%s", c.Request.URL.Scheme, c.Request.URL.Host)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, "login"))
		return
	}
	if mode.Value != "authenticate" && mode.Value != "link" {
	}
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/api/v1/auth/oidc/%s/%s?code=%s", baseURL, mode.Value, c.Param("id"), c.Query("code")))
}
