package api

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/nfmmu/uploader-archive/internal/config"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"github.com/nfmmu/uploader-archive/internal/hooks"
	"github.com/nfmmu/uploader-archive/internal/oidc"
	"github.com/nfmmu/uploader-archive/internal/security"
)

type Server struct {
	db                   *gorm.DB
	cfg                  config.Config
	logger               *slog.Logger
	enrollmentLimiterMu  sync.Mutex
	enrollmentAttempts   map[string]enrollmentAttempt
	publicIntakeAttempts map[string]enrollmentAttempt
}

func New(db *gorm.DB, cfg config.Config, logger *slog.Logger) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	s := &Server{db: db, cfg: cfg, logger: logger, enrollmentAttempts: make(map[string]enrollmentAttempt), publicIntakeAttempts: make(map[string]enrollmentAttempt)}
	r := gin.New()
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		logger.Error("invalid TRUSTED_PROXIES configuration", "error", err)
		panic(err)
	}
	r.Use(gin.Recovery(), requestLogger(logger), securityHeaders())
	r.GET("/healthz", s.health)
	v1 := r.Group("/api/v1")
	v1.POST("/auth/login", s.login)
	v1.GET("/auth/oidc/providers", s.listOidcProvidersPublic)
	v1.GET("/auth/oidc/:id", oidc.AuthorizationRequest)
	v1.GET("/auth/oidc/callback/:id", s.oidcCallback)
	v1.GET("/auth/oidc/authenticate/:id", s.oidcAuthenticate)
	v1.GET("/public/config", s.publicConfig)
	v1.GET("/public/artifacts", s.listPublicArtifacts)
	v1.GET("/public/artifacts/:id", s.getPublicArtifact)
	v1.POST("/public-requests", s.submitPublicRequest)
	v1.POST("/public/redistribution-assistance", s.rateLimitPublicIntake(), s.submitRedistributionAssistance)
	v1.POST("/public/node-enrollment-requests", s.rateLimitNodeEnrollment(), s.submitNodeEnrollmentRequest)
	v1.POST("/public/files/:id/downloads", s.createPublicDownloadJob)
	v1.GET("/public/downloads/:id", s.getPublicDownloadJob)
	v1.GET("/public/downloads/:id/content", s.downloadPublicContent)
	v1.POST("/nodes/:id/delivery-jobs/claim", s.authenticateNode(), s.claimDeliveryJob)
	v1.POST("/nodes/:id/delivery-jobs/:jobID/content", s.authenticateNode(), s.receiveDeliveryContent)
	v1.POST("/nodes/register", s.registerNode)
	v1.POST("/nodes/:id/heartbeat", s.authenticateNode(), s.nodeHeartbeat)
	v1.POST("/nodes/:id/tasks/claim", s.authenticateNode(), s.claimNodeTask)
	v1.GET("/nodes/:id/tasks/:taskID/content", s.authenticateNode(), s.nodeTaskContent)
	v1.POST("/nodes/:id/tasks/:taskID/result", s.authenticateNode(), s.nodeTaskResult)
	v1.POST("/nodes/:id/tasks/:taskID/source-content", s.authenticateNode(), s.receiveSourceFetchContent)
	v1.POST("/nodes/:id/tasks/:taskID/replica-content", s.authenticateNode(), s.replicaContent)
	v1.GET("/delete-cookie", s.clearCookie)

	auth := v1.Group("")
	auth.Use(s.authenticate())
	auth.GET("/auth/oidc/link/:id", s.oidcLink)
	auth.GET("/me", s.me)
	auth.GET("/artifacts", s.listArtifacts)
	auth.POST("/artifacts", s.requireRoles("admin"), s.createArtifact)
	auth.PATCH("/artifacts/:id", s.requireRoles("admin", "reviewer"), s.updateArtifactMetadata)
	auth.GET("/artifacts/:id", s.getArtifact)
	auth.POST("/artifacts/:id/files", s.requireRoles("admin"), s.createFileObject)
	auth.GET("/files", s.listFiles)
	auth.GET("/files/:id", s.getFileObject)
	auth.PATCH("/files/:id/metadata", s.requireRoles("admin", "reviewer"), s.updateFileMetadata)
	auth.POST("/files/bulk-action", s.requireRoles("admin", "reviewer"), s.bulkFileAction)
	auth.POST("/admin/deletion-requests", s.requireRoles("admin"), s.createAdminDeletionRequests)
	auth.GET("/files/:id/placements", s.requireRoles("admin", "reviewer"), s.listPlacements)
	auth.POST("/files/:id/replicate", s.requireRoles("admin"), s.requestReplica)
	auth.POST("/files/:id/ingest", s.requireRoles("admin"), s.ingestFile)
	auth.POST("/files/:id/ingest-source", s.requireRoles("admin"), s.ingestSourceFile)
	auth.POST("/artifacts/:id/discover-source-files", s.requireRoles("admin"), s.discoverUploaderFiles)
	auth.POST("/admin/url-collection/search", s.requireRoles("admin"), s.searchUploaderURLs)
	auth.POST("/admin/url-collection/scan", s.requireRoles("admin"), s.collectUploaderURLs)
	auth.GET("/nodes", s.requireRoles("admin", "reviewer"), s.listNodes)
	auth.DELETE("/nodes/:id", s.requireRoles("admin", "reviewer"), s.deleteNodes)
	auth.GET("/admin/node-enrollment-requests", s.requireRoles("admin"), s.listNodeEnrollmentRequests)
	auth.GET("/admin/conversation-links", s.requireRoles("admin", "reviewer"), s.listConversationLinks)
	auth.POST("/admin/conversation-links", s.requireRoles("admin", "reviewer"), s.addConversationLink)
	auth.DELETE("/admin/conversation-links/:id", s.requireRoles("admin", "reviewer"), s.deleteConversationLink)
	auth.PATCH("/admin/node-enrollment-requests/:id", s.requireRoles("admin"), s.reviewNodeEnrollmentRequest)
	auth.GET("/tasks", s.requireRoles("admin", "reviewer"), s.listTasks)
	auth.POST("/admin/placements/reconcile", s.requireRoles("admin"), s.reconcileNow)
	auth.POST("/tasks/retry-all", s.requireRoles("admin"), s.retryAll)
	auth.POST("/tasks/clear-failed", s.requireRoles("admin"), s.clearFailedTasks)
	auth.POST("/tasks/:id/retry", s.requireRoles("admin"), s.retryTask)
	auth.PATCH("/files/:id/policy", s.requireRoles("admin", "reviewer"), s.updateFilePolicy)
	auth.PATCH("/files/:id/publication", s.requireRoles("admin"), s.updatePublicationFlag)
	auth.POST("/files/:id/downloads", s.createDownloadJob)
	auth.GET("/downloads/:id", s.getDownloadJob)
	auth.GET("/downloads/:id/content", s.downloadContent)
	auth.GET("/export", s.exportCatalog)
	auth.GET("/public-requests", s.requireRoles("admin", "reviewer"), s.listPublicRequests)
	auth.GET("/public-requests/:id", s.requireRoles("admin", "reviewer"), s.getPublicRequest)
	auth.PATCH("/public-requests/:id", s.requireRoles("admin", "reviewer"), s.patchPublicRequest)
	auth.GET("/public-requests/:id/intake-file", s.requireRoles("admin", "reviewer"), s.downloadPublicRequestIntakeFile)
	auth.POST("/public-requests/:id/ingest-upload", s.requireRoles("admin"), s.ingestPublicRequestUpload)
	auth.POST("/admin/users", s.requireRoles("admin"), s.createUser)
	auth.GET("/admin/users", s.requireRoles("admin"), s.listUsers)
	auth.PUT("/admin/users/:id", s.updateUser)
	auth.PUT("/admin/users/:id/password", s.updatePassword)
	auth.GET("/users/:id/oidc_identities", s.listUserOidcIdentities)
	auth.DELETE("/users/:user_id/oidc_identities/:identity_id", s.deleteUserOidcIdentity)
	auth.GET("/admin/audit-logs", s.requireRoles("admin"), s.listAuditLogs)
	auth.GET("/admin/oidc_providers", s.requireRoles("admin"), s.listOidcProviders)
	auth.POST("/admin/oidc_providers", s.requireRoles("admin"), s.createOidcProvider)
	auth.GET("/admin/oidc_providers/:id", s.requireRoles("admin"), s.getOidcProvider)
	auth.PUT("/admin/oidc_providers/:id", s.requireRoles("admin"), s.putOidcProvider)
	auth.DELETE("/admin/oidc_providers/:id", s.requireRoles("admin"), s.deleteOidcProvider)
	go s.runPlacementReconciler()
	go s.runDownloadCleanup()
	return r
}

func (s *Server) health(c *gin.Context) {
	sqlDB, err := s.db.DB()
	if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email,max=320"`
		Password string `json:"password" binding:"required,min=1,max=1024"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}
	var user model.User
	err := s.db.WithContext(c.Request.Context()).Where("email = ?", strings.ToLower(strings.TrimSpace(req.Email))).First(&user).Error
	if err != nil || user.DisabledAt != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	token, err := security.Issue(s.cfg.JWTSecret, s.cfg.JWTTTL, user.ID, user.Role, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create access token"})
		return
	}
	s.audit(c, &user, "user", user.ID, "auth.login", "success", nil)
	s.setAuthCookie(c, token)
	c.JSON(http.StatusOK, gin.H{"access_token": token, "token_type": "Bearer", "expires_in": int(s.cfg.JWTTTL.Seconds()), "user": gin.H{"id": user.ID, "email": user.Email, "display_name": user.DisplayName, "role": user.Role}})
}

func (s *Server) authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ""

		if cookie, err := c.Cookie("archive_token"); err == nil {
			token = cookie
		}
		if token == "" {
			header := c.GetHeader("Authorization")
			if after, ok := strings.CutPrefix(header, "Bearer "); ok {
				token = strings.TrimSpace(after)
			}
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			return
		}
		claims, err := security.Parse(s.cfg.JWTSecret, token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		var user model.User
		err = s.db.WithContext(c.Request.Context()).Where("id = ?", claims.UserID).First(&user).Error
		if err != nil || user.DisabledAt != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "account unavailable"})
			return
		}
		c.Set("currentUser", user)
		c.Next()
	}
}

func (s *Server) requireRoles(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(c *gin.Context) {
		user, ok := hooks.CurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if _, ok := allowed[user.Role]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient role"})
			return
		}
		c.Next()
	}
}

func (s *Server) me(c *gin.Context) {
	user, _ := hooks.CurrentUser(c)
	c.JSON(http.StatusOK, gin.H{"id": user.ID, "email": user.Email, "display_name": user.DisplayName, "role": user.Role, "created_at": user.CreatedAt})
}
