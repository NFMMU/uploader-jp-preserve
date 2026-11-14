package api

import (
	"github.com/gin-gonic/gin"
	"github.com/nfmmu/uploader-archive/internal/oidc"
)

func (s *Server) oidcAuthorizationRequest(c *gin.Context) {
	oidc.AuthorizationRequest(c, s.cfg.FrontendURL)
}
