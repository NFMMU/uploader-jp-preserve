package api

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nfmmu/uploader-archive/internal/config"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"github.com/nfmmu/uploader-archive/internal/security"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
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

	auth := v1.Group("")
	auth.Use(s.authenticate())
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
	auth.GET("/admin/node-enrollment-requests", s.requireRoles("admin"), s.listNodeEnrollmentRequests)
	auth.GET("/admin/conversation-links", s.requireRoles("admin", "reviewer"), s.listConversationLinks)
	auth.POST("/admin/conversation-links", s.requireRoles("admin", "reviewer"), s.addConversationLink)
	auth.DELETE("/admin/conversation-links/:id", s.requireRoles("admin", "reviewer"), s.deleteConversationLink)
	auth.PATCH("/admin/node-enrollment-requests/:id", s.requireRoles("admin"), s.reviewNodeEnrollmentRequest)
	auth.GET("/tasks", s.requireRoles("admin", "reviewer"), s.listTasks)
	auth.POST("/admin/placements/reconcile", s.requireRoles("admin"), s.reconcileNow)
	auth.POST("/tasks/:id/retry", s.requireRoles("admin"), s.retryTask)
	auth.PATCH("/files/:id/policy", s.requireRoles("admin", "reviewer"), s.updateFilePolicy)
	auth.PATCH("/files/:id/publication", s.requireRoles("admin"), s.updatePublicationFlag)
	auth.POST("/files/:id/downloads", s.createDownloadJob)
	auth.GET("/downloads/:id", s.getDownloadJob)
	auth.GET("/downloads/:id/content", s.downloadContent)
	auth.GET("/export", s.exportCatalog)
	auth.GET("/public-requests", s.requireRoles("admin", "reviewer"), s.listPublicRequests)
	auth.PATCH("/public-requests/:id", s.requireRoles("admin", "reviewer"), s.reviewPublicRequest)
	auth.GET("/public-requests/:id/intake-file", s.requireRoles("admin", "reviewer"), s.downloadPublicRequestIntakeFile)
	auth.POST("/public-requests/:id/ingest-upload", s.requireRoles("admin"), s.ingestPublicRequestUpload)
	auth.POST("/admin/users", s.requireRoles("admin"), s.createUser)
	auth.GET("/admin/users", s.requireRoles("admin"), s.listUsers)
	auth.PATCH("/admin/users/:id", s.requireRoles("admin"), s.updateUser)
	auth.GET("/admin/audit-logs", s.requireRoles("admin"), s.listAuditLogs)
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
	c.JSON(http.StatusOK, gin.H{"access_token": token, "token_type": "Bearer", "expires_in": int(s.cfg.JWTTTL.Seconds()), "user": gin.H{"id": user.ID, "email": user.Email, "display_name": user.DisplayName, "role": user.Role}})
}

func (s *Server) authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bearer token required"})
			return
		}
		claims, err := security.Parse(s.cfg.JWTSecret, strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
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
		user, ok := currentUser(c)
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
	user, _ := currentUser(c)
	c.JSON(http.StatusOK, gin.H{"id": user.ID, "email": user.Email, "display_name": user.DisplayName, "role": user.Role, "created_at": user.CreatedAt})
}

func (s *Server) createUser(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email,max=320"`
		Password    string `json:"password" binding:"required,min=12,max=1024"`
		DisplayName string `json:"display_name" binding:"required,min=1,max=120"`
		Role        string `json:"role" binding:"required,oneof=admin reviewer viewer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email, display_name, password (12+ chars), and valid role are required"})
		return
	}
	actor, _ := currentUser(c)
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not hash password"})
		return
	}
	user := model.User{Email: strings.ToLower(strings.TrimSpace(req.Email)), PasswordHash: string(hash), DisplayName: strings.TrimSpace(req.DisplayName), Role: req.Role}
	if err := s.db.WithContext(c.Request.Context()).Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "user could not be created; email may already exist"})
		return
	}
	s.audit(c, &actor, "user", user.ID, "user.create", "success", nil)
	c.JSON(http.StatusCreated, gin.H{"id": user.ID, "email": user.Email, "display_name": user.DisplayName, "role": user.Role, "created_at": user.CreatedAt})
}

func (s *Server) listUsers(c *gin.Context) {
	var users []model.User
	if err := s.db.WithContext(c.Request.Context()).Select("id", "email", "display_name", "role", "disabled_at", "created_at", "updated_at").Order("created_at DESC").Limit(500).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list users"})
		return
	}
	type userView struct {
		ID          string     `json:"id"`
		Email       string     `json:"email"`
		DisplayName string     `json:"display_name"`
		Role        string     `json:"role"`
		DisabledAt  *time.Time `json:"disabled_at,omitempty"`
		CreatedAt   time.Time  `json:"created_at"`
		UpdatedAt   time.Time  `json:"updated_at"`
	}
	items := make([]userView, 0, len(users))
	for _, user := range users {
		items = append(items, userView{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName, Role: user.Role, DisabledAt: user.DisabledAt, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
