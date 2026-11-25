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
	"github.com/nfmmu/uploader-archive/internal/security"
)

type Server struct {
	db                   *gorm.DB
	cfg                  config.Config
	logger               *slog.Logger
	enrollmentLimiterMu  sync.Mutex
	enrollmentAttempts   map[string]enrollmentAttempt
	publicIntakeAttempts map[string]enrollmentAttempt

	// Placement reconciliation is single-flight: the periodic pass and the
	// admin button share one gate and one status snapshot.
	placementReconcileMu    sync.Mutex
	placementReconcileGate  chan struct{}
	placementReconcileState placementReconcileSnapshot
	// lastManualReconcile keeps the admin-triggered result so a periodic run
	// cannot overwrite what the screen is waiting for.
	lastManualReconcile placementReconcileSnapshot
	// reconcileHistory answers a poll for a specific run_id even after a later
	// run has started.
	reconcileHistory []placementReconcileSnapshot
	// reconcileRunHook lets tests hold a run at a known point. Nil in production.
	reconcileRunHook func()
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
	v1.GET("/auth/oidc/:id", s.oidcAuthorizationRequest)
	v1.GET("/auth/oidc/callback/:id", s.oidcCallback)
	v1.GET("/auth/oidc/authenticate/:id", s.oidcAuthenticate)
	v1.GET("/public/config", s.publicConfig)
	v1.GET("/public/artifacts", s.listPublicArtifacts)
	v1.GET("/public/artifacts/:id", s.getPublicArtifact)
	v1.GET("/public/files/:id", s.getPublicFile)
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

	// Account endpoints always operate on the authenticated user.
	account := auth.Group("/account")
	account.PUT("", s.updateOwnAccount)
	account.PUT("/password", s.updateOwnPassword)
	account.GET("/oidc-identities", s.listOwnUserOidcIdentities)
	account.DELETE("/oidc-identities/:identityID", s.deleteOwnUserOidcIdentity)

	// Catalog endpoints. Author access is additionally restricted to assigned artifacts.
	catalog := auth.Group("/catalog")
	catalog.GET("/artifacts", s.listArtifacts)
	catalog.GET("/artifacts/:id", s.requireAuthorArtifactAccess(), s.getArtifact)
	catalog.POST("/artifacts", s.requireRoles("admin"), s.createArtifact)
	catalog.PATCH("/artifacts/:id", s.requireRoles("admin", "reviewer", "author"), s.requireAuthorArtifactAccess(), s.updateArtifactMetadata)
	catalog.POST("/artifacts/:id/files", s.requireRoles("admin", "author"), s.requireAuthorArtifactAccess(), s.createFileObject)
	catalog.POST("/files/:id/replace", s.requireRoles("admin", "author"), s.requireAuthorFileArtifactAccess(), s.replaceFileObject)
	catalog.POST("/artifacts/:id/discover-source-files", s.requireRoles("admin"), s.discoverUploaderFiles)
	catalog.GET("/files", s.listFiles)
	catalog.GET("/files/:id", s.getFileObject)
	catalog.DELETE("/files/:id", s.deleteFileObject)
	catalog.PATCH("/files/:id/metadata", s.requireRoles("admin", "reviewer", "author"), s.requireAuthorFileArtifactAccess(), s.updateFileMetadata)
	catalog.POST("/files/bulk-action", s.requireRoles("admin", "reviewer"), s.bulkFileAction)
	catalog.GET("/files/:id/placements", s.requireRoles("admin", "reviewer"), s.listPlacements)
	catalog.POST("/files/:id/replicate", s.requireRoles("admin"), s.requestReplica)
	catalog.POST("/files/:id/ingest", s.requireRoles("admin"), s.ingestFile)
	catalog.POST("/files/:id/ingest-source", s.requireRoles("admin", "author"), s.requireAuthorFileArtifactAccess(), s.ingestSourceFile)
	catalog.PATCH("/files/:id/policy", s.requireRoles("admin", "reviewer", "author"), s.requireAuthorFileArtifactAccess(), s.updateFilePolicy)
	catalog.PATCH("/files/:id/publication", s.requireRoles("admin", "author"), s.requireAuthorFileArtifactAccess(), s.updatePublicationFlag)
	catalog.POST("/files/:id/downloads", s.requireRoles("admin", "reviewer", "viewer"), s.createDownloadJob)
	catalog.GET("/downloads/:id", s.requireRoles("admin", "reviewer", "viewer"), s.getDownloadJob)
	catalog.GET("/downloads/:id/content", s.requireRoles("admin", "reviewer", "viewer"), s.downloadContent)
	catalog.GET("/export", s.requireRoles("admin", "reviewer", "viewer"), s.exportCatalog)

	// Moderation and request review.
	moderation := auth.Group("/moderation")
	moderation.Use(s.requireRoles("admin", "reviewer"))
	moderation.GET("/authors", s.listAuthors)
	moderation.GET("/public-requests", s.listPublicRequests)
	moderation.GET("/public-requests/:id", s.getPublicRequest)
	moderation.PATCH("/public-requests/:id", s.patchPublicRequest)
	moderation.GET("/public-requests/:id/intake-file", s.downloadPublicRequestIntakeFile)
	moderation.GET("/conversation-links", s.listConversationLinks)
	moderation.POST("/conversation-links", s.addConversationLink)
	moderation.DELETE("/conversation-links/:id", s.deleteConversationLink)
	moderation.POST("/public-requests/:id/ingest-upload", s.requireRoles("admin"), s.ingestPublicRequestUpload)

	// System operations are separated from catalog and account management.
	operations := auth.Group("/operations")
	operations.Use(s.requireRoles("admin", "reviewer"))
	operations.GET("/nodes", s.listNodes)
	operations.DELETE("/nodes/:id", s.deleteNodes)
	operations.GET("/tasks", s.listTasks)
	operations.POST("/tasks/:id/retry", s.requireRoles("admin"), s.retryTask)
	operations.POST("/tasks/retry-all", s.requireRoles("admin"), s.retryAll)
	operations.POST("/tasks/clear-failed", s.requireRoles("admin"), s.clearFailedTasks)

	// Administrative endpoints.
	admin := auth.Group("/admin")
	admin.Use(s.requireRoles("admin"))
	admin.POST("/users", s.createUser)
	admin.GET("/users", s.listUsers)
	admin.PUT("/users/:id", s.updateUser)
	admin.DELETE("/users/:id", s.deleteUser)
	admin.PUT("/users/:id/password", s.updatePassword)
	admin.GET("/users/:id/oidc-identities", s.listUserOidcIdentities)
	admin.DELETE("/users/:id/oidc-identities/:identityID", s.deleteAdminUserOidcIdentity)
	admin.GET("/author-assignments/:userID", s.getAuthorArtifactAssignments)
	admin.PUT("/author-assignments/:userID", s.replaceAuthorArtifactAssignments)
	admin.GET("/node-enrollment-requests", s.listNodeEnrollmentRequests)
	admin.PATCH("/node-enrollment-requests/:id", s.reviewNodeEnrollmentRequest)
	admin.POST("/url-collection/search", s.searchUploaderURLs)
	admin.POST("/url-collection/scan", s.collectUploaderURLs)
	admin.POST("/placements/reconcile", s.reconcileNow)
	admin.GET("/placements/reconcile", s.reconcileStatus)
	admin.GET("/audit-logs", s.listAuditLogs)
	admin.GET("/oidc-providers", s.listOidcProviders)
	admin.POST("/oidc-providers", s.createOidcProvider)
	admin.GET("/oidc-providers/:id", s.getOidcProvider)
	admin.PUT("/oidc-providers/:id", s.putOidcProvider)
	admin.DELETE("/oidc-providers/:id", s.deleteOidcProvider)
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
