package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Server) setAuthCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "archive_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.Environment == "production",
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(s.cfg.JWTTTL),
		MaxAge:   int(s.cfg.JWTTTL.Seconds()),
	})
}

func (s *Server) clearCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:    "archive_token",
		Expires: time.Now().Add(-s.cfg.JWTTTL),
		Path:    "/",
	})
	c.Status(http.StatusNoContent)
}
