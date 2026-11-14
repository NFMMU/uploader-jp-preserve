package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/nfmmu/uploader-archive/internal/oidc"
)

func (s *Server) oidcLink(c *gin.Context) {
	baseURL := fmt.Sprintf("%s://%s", c.Request.URL.Scheme, c.Request.URL.Host)

	// oidc parse
	data := oidc.Callback(c, "link")
	if data == nil {
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/admin/preference?linked", baseURL))
}
