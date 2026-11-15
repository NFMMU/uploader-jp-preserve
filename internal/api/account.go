package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"github.com/nfmmu/uploader-archive/internal/hooks"
)

func (s *Server) updateOwnAccount(c *gin.Context) {
	var req struct {
		DisplayName string `json:"display_name" binding:"required,min=1,max=120"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.DisplayName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "display_name is required"})
		return
	}
	actor, ok := hooks.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	name := strings.TrimSpace(req.DisplayName)
	result := s.db.WithContext(c.Request.Context()).Model(&model.User{}).
		Where("id = ?", actor.ID).
		Updates(map[string]any{"display_name": name, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update account"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	s.audit(c, &actor, "user", actor.ID, "user.account.update", "success", nil)
	c.JSON(http.StatusOK, gin.H{"id": actor.ID, "display_name": name})
}

func (s *Server) updateOwnPassword(c *gin.Context) {
	actor, ok := hooks.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	c.Params = append(c.Params, gin.Param{Key: "id", Value: actor.ID})
	s.updatePassword(c)
}

func (s *Server) listOwnUserOidcIdentities(c *gin.Context) {
	actor, ok := hooks.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	c.Params = append(c.Params, gin.Param{Key: "id", Value: actor.ID})
	s.listUserOidcIdentities(c)
}

func (s *Server) deleteOwnUserOidcIdentity(c *gin.Context) {
	actor, ok := hooks.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	c.Params = append(c.Params,
		gin.Param{Key: "user_id", Value: actor.ID},
		gin.Param{Key: "identity_id", Value: c.Param("identityID")},
	)
	s.deleteUserOidcIdentity(c)
}

func (s *Server) deleteAdminUserOidcIdentity(c *gin.Context) {
	c.Params = append(c.Params,
		gin.Param{Key: "user_id", Value: c.Param("id")},
		gin.Param{Key: "identity_id", Value: c.Param("identityID")},
	)
	s.deleteUserOidcIdentity(c)
}
