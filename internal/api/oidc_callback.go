package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) oidcCallback(c *gin.Context) {
	mode, err := c.Request.Cookie("oidc_mode")
	baseURL := s.cfg.FrontendURL
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/%s?error=oidc", baseURL, "login"))
		return
	}
	if mode.Value != "authenticate" && mode.Value != "link" {
	}
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/api/v1/auth/oidc/%s/%s?%s", baseURL, mode.Value, c.Param("id"), c.Request.URL.RawQuery))
}
