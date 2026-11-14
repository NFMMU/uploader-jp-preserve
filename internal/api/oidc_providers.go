package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"github.com/nfmmu/uploader-archive/internal/gen/query"
	"github.com/nfmmu/uploader-archive/internal/hooks"
)

func (s *Server) listOidcProvidersPublic(c *gin.Context) {
	oidcProviderQuery := query.OidcProvider
	data, err := oidcProviderQuery.WithContext(c.Request.Context()).Where(oidcProviderQuery.IsEnabled.Eq(true)).
		Select(oidcProviderQuery.ID, oidcProviderQuery.DisplayName, oidcProviderQuery.ServiceIcon).
		Find()
	if err != nil || data == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": data})
}

func (s *Server) listOidcProviders(c *gin.Context) {
	oidcProviderQuery := query.OidcProvider
	data, err := oidcProviderQuery.WithContext(c.Request.Context()).Where(oidcProviderQuery.DeletedAt.IsNull()).Find()
	if err != nil || data == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": data})
}

func (s *Server) getOidcProvider(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	oidcProviderQuery := query.OidcProvider
	data, err := oidcProviderQuery.WithContext(c.Request.Context()).Where(oidcProviderQuery.DeletedAt.IsNull(), oidcProviderQuery.ID.Eq(id)).First()
	if err != nil || data == nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (s *Server) createOidcProvider(c *gin.Context) {
	var req struct {
		DisplayName       string  `json:"display_name" binding:"required,max=255"`
		DiscoveryEndpoint string  `json:"discovery_endpoint" binding:"required,max=255"`
		ClientID          string  `json:"client_id" binding:"required,max=255"`
		ClientSecret      string  `json:"client_secret" binding:"required,max=255"`
		ServiceIcon       *string `json:"service_icon" binding:"omitempty,max=255"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "display_name, discovery_endpoint, client_id, client_secret are required"})
		return
	}
	actor, _ := hooks.CurrentUser(c)
	oidcProviderQuery := query.OidcProvider
	err := oidcProviderQuery.WithContext(c.Request.Context()).Create(&model.OidcProvider{
		DisplayName:       req.DisplayName,
		DiscoveryEndpoint: req.DiscoveryEndpoint,
		ClientID:          req.ClientID,
		ClientSecret:      req.ClientSecret,
		ServiceIcon:       req.ServiceIcon,
		UpdatedAt:         time.Now(),
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			c.JSON(http.StatusConflict, gin.H{"error": "this provider is already registered"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	data, err := oidcProviderQuery.WithContext(c.Request.Context()).
		Where(oidcProviderQuery.ClientID.Eq(req.ClientID), oidcProviderQuery.DiscoveryEndpoint.Eq(req.DiscoveryEndpoint)).First()
	if data == nil || err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	s.audit(c, &actor, "auth.oidc_provider", data.ID, "create", "success", nil)
	c.JSON(http.StatusOK, data)
}

func (s *Server) putOidcProvider(c *gin.Context) {
	var req struct {
		DisplayName       string  `json:"display_name" binding:"required,max=255"`
		DiscoveryEndpoint string  `json:"discovery_endpoint" binding:"required,max=255"`
		ClientID          string  `json:"client_id" binding:"required,max=255"`
		ClientSecret      *string `json:"client_secret" binding:"omitempty,max=255"`
		ServiceIcon       *string `json:"service_icon" binding:"omitempty,max=255"`
	}
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "display_name, discovery_endpoint, client_id, client_secret are required"})
		return
	}
	actor, _ := hooks.CurrentUser(c)
	oidcProviderQuery := query.OidcProvider
	data, err := oidcProviderQuery.WithContext(c.Request.Context()).Where(oidcProviderQuery.DeletedAt.IsNull(), oidcProviderQuery.ID.Eq(id)).First()
	if err != nil || data == nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	clientSecret := data.ClientSecret
	if req.ClientSecret != nil {
		clientSecret = *req.ClientSecret
	}
	err = oidcProviderQuery.WithContext(c.Request.Context()).Where(oidcProviderQuery.ID.Eq(id)).Save(&model.OidcProvider{
		DisplayName:       req.DisplayName,
		DiscoveryEndpoint: req.DiscoveryEndpoint,
		ClientID:          req.ClientID,
		ClientSecret:      clientSecret,
		ServiceIcon:       req.ServiceIcon,
		UpdatedAt:         time.Now(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	data, err = oidcProviderQuery.WithContext(c.Request.Context()).
		Where(oidcProviderQuery.ClientID.Eq(req.ClientID), oidcProviderQuery.DiscoveryEndpoint.Eq(req.DiscoveryEndpoint)).First()
	if data == nil || err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	s.audit(c, &actor, "auth.oidc_provider", id, "update", "success", nil)
	c.JSON(http.StatusOK, data)
}

func (s *Server) deleteOidcProvider(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	actor, _ := hooks.CurrentUser(c)
	oidcProviderQuery := query.OidcProvider
	result, err := oidcProviderQuery.WithContext(c.Request.Context()).Where(oidcProviderQuery.DeletedAt.Null(), oidcProviderQuery.ID.Eq(id)).Delete(nil)
	if err != nil || result.Error != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if result.RowsAffected < 1 {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	s.audit(c, &actor, "auth.oidc_provider", id, "delete", "success", nil)
	c.Status(http.StatusNoContent)
}
