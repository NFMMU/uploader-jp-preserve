package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	nodeEnrollmentWindow = time.Hour
	nodeEnrollmentLimit  = 5
	nodeEnrollmentTTL    = 72 * time.Hour
)

type enrollmentAttempt struct {
	windowStart time.Time
	count       int
}

func (s *Server) rateLimitNodeEnrollment() gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now()
		ip := c.ClientIP()
		s.enrollmentLimiterMu.Lock()
		attempt, ok := s.enrollmentAttempts[ip]
		if !ok || now.Sub(attempt.windowStart) >= nodeEnrollmentWindow {
			attempt = enrollmentAttempt{windowStart: now}
		}
		if attempt.count >= nodeEnrollmentLimit {
			s.enrollmentLimiterMu.Unlock()
			c.Header("Retry-After", "3600")
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "参加申請の回数上限に達しました。時間をおいて再度お試しください。"})
			c.Abort()
			return
		}
		attempt.count++
		s.enrollmentAttempts[ip] = attempt
		if len(s.enrollmentAttempts) > 2000 {
			for key, value := range s.enrollmentAttempts {
				if now.Sub(value.windowStart) >= nodeEnrollmentWindow {
					delete(s.enrollmentAttempts, key)
				}
			}
		}
		s.enrollmentLimiterMu.Unlock()
		c.Next()
	}
}

func (s *Server) submitNodeEnrollmentRequest(c *gin.Context) {
	var req struct {
		TurnstileToken         string `json:"turnstile_token"`
		Name                   string `json:"name" binding:"required,min=1,max=120"`
		NodeType               string `json:"node_type" binding:"required,oneof=A B"`
		RequestedCapacityBytes *int64 `json:"requested_capacity_bytes" binding:"required,gte=0"`
		ContactEmail           string `json:"contact_email" binding:"required,email,max=320"`
		Message                string `json:"message" binding:"max=1000"`
		AgreeToTerms           bool   `json:"agree_to_terms" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ノード名、種別、容量、連絡先メールアドレスを入力し、参加条件への同意を確認してください。"})
		return
	}
	if !s.verifyTurnstile(c, req.TurnstileToken) {
		return
	}
	if !req.AgreeToTerms {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参加条件への同意が必要です。"})
		return
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.ContactEmail) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ノード名と連絡先を入力してください。"})
		return
	}
	id := uuid.NewString()
	err := s.db.WithContext(c.Request.Context()).Exec(`
		INSERT INTO node_enrollment_requests
			(id, name, node_type, requested_capacity_bytes, contact_email, message, status, terms_accepted_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'pending', now(), now(), now())`, id, strings.TrimSpace(req.Name), req.NodeType, *req.RequestedCapacityBytes, strings.ToLower(strings.TrimSpace(req.ContactEmail)), strings.TrimSpace(req.Message)).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "参加申請を保存できませんでした。時間をおいて再度お試しください。"})
		return
	}
	content := "新規ストレージノード参加申請\nノード名: " + strings.TrimSpace(req.Name) + "\n種別: " + req.NodeType + "\n申告容量: " + fmt.Sprintf("%d bytes", *req.RequestedCapacityBytes) + "\n連絡先: " + strings.ToLower(strings.TrimSpace(req.ContactEmail)) + "\n補足: " + strings.TrimSpace(req.Message)
	s.createLibreDeskConversation("node_enrollment_request", id, "【uploader.jp保全】ストレージノード参加申請: "+strings.TrimSpace(req.Name), strings.TrimSpace(req.Name), strings.TrimSpace(req.ContactEmail), content)
	c.JSON(http.StatusCreated, gin.H{"id": id, "status": "pending", "message": "参加申請を受け付けました。運営による確認後、登録手順を連絡します。"})
}

func (s *Server) listNodeEnrollmentRequests(c *gin.Context) {
	type enrollmentRequestView struct {
		ID                     string     `json:"id"`
		Name                   string     `json:"name"`
		NodeType               string     `json:"node_type"`
		RequestedCapacityBytes int64      `json:"requested_capacity_bytes"`
		ContactEmail           string     `json:"contact_email"`
		Message                string     `json:"message"`
		Status                 string     `json:"status"`
		NodeID                 *string    `json:"node_id,omitempty"`
		TokenExpiresAt         *time.Time `json:"token_expires_at,omitempty"`
		CreatedAt              time.Time  `json:"created_at"`
		UpdatedAt              time.Time  `json:"updated_at"`
		ApprovedAt             *time.Time `json:"approved_at,omitempty"`
	}
	var items []enrollmentRequestView
	err := s.db.WithContext(c.Request.Context()).Table("node_enrollment_requests").
		Select("id, name, node_type, requested_capacity_bytes, contact_email, message, status, node_id, token_expires_at, created_at, updated_at, approved_at").
		Order("created_at DESC").Limit(500).Find(&items).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list node enrollment requests"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) reviewNodeEnrollmentRequest(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid enrollment request id"})
		return
	}
	var req struct {
		Status string `json:"status" binding:"required,oneof=approved rejected"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be approved or rejected"})
		return
	}
	actor, _ := currentUser(c)
	var existing struct {
		Status string
		Name   string
	}
	if err := s.db.WithContext(c.Request.Context()).Raw("SELECT status, name FROM node_enrollment_requests WHERE id = ?", id).Scan(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load enrollment request"})
		return
	}
	if existing.Name == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "enrollment request not found"})
		return
	}
	if existing.Status == "enrolled" || existing.Status == "rejected" && req.Status == "approved" {
		c.JSON(http.StatusConflict, gin.H{"error": "this enrollment request cannot be approved in its current state"})
		return
	}
	if req.Status == "rejected" {
		result := s.db.WithContext(c.Request.Context()).Exec(`UPDATE node_enrollment_requests SET status = 'rejected', enrollment_token_hash = NULL, token_expires_at = NULL, reviewed_by = ?, updated_at = now() WHERE id = ? AND status <> 'enrolled'`, actor.ID, id)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not reject enrollment request"})
			return
		}
		if result.RowsAffected != 1 {
			c.JSON(http.StatusConflict, gin.H{"error": "enrollment request state changed; refresh and try again"})
			return
		}
		s.audit(c, &actor, "node_enrollment_request", id, "node_enrollment.reject", "success", nil)
		c.JSON(http.StatusOK, gin.H{"status": "rejected"})
		return
	}
	if existing.Status != "pending" && existing.Status != "approved" {
		c.JSON(http.StatusConflict, gin.H{"error": "only pending requests can be approved"})
		return
	}
	token, err := newNodeToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not issue enrollment token"})
		return
	}
	expires := time.Now().UTC().Add(nodeEnrollmentTTL)
	result := s.db.WithContext(c.Request.Context()).Exec(`UPDATE node_enrollment_requests SET status = 'approved', enrollment_token_hash = ?, token_expires_at = ?, approved_by = ?, approved_at = now(), reviewed_by = ?, updated_at = now() WHERE id = ? AND status IN ('pending', 'approved')`, hashNodeToken(token), expires, actor.ID, actor.ID, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not approve enrollment request"})
		return
	}
	if result.RowsAffected != 1 {
		c.JSON(http.StatusConflict, gin.H{"error": "enrollment request state changed; refresh and try again"})
		return
	}
	s.audit(c, &actor, "node_enrollment_request", id, "node_enrollment.approve", "success", nil)
	c.JSON(http.StatusOK, gin.H{"status": "approved", "enrollment_token": token, "expires_at": expires, "warning": "このトークンは今回だけ表示されます。参加者へ安全な方法で共有してください。"})
}
