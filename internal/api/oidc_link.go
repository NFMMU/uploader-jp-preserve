package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/nfmmu/uploader-archive/internal/oidc"
)

func (s *Server) oidcLink(c *gin.Context) {
	baseURL := s.cfg.FrontendURL

	// oidc parse
	data := oidc.Callback(c, s.cfg.FrontendURL, "link", s.decryptSourcePassword)
	if data == nil {
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/preference?linked", baseURL))
}
