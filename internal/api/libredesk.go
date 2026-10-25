package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type libreDeskConversationRequest struct {
	Subject          string            `json:"subject"`
	Content          string            `json:"content"`
	InboxID          int64             `json:"inbox_id"`
	TeamID           *int64            `json:"team_id,omitempty"`
	AgentID          *int64            `json:"agent_id,omitempty"`
	ContactEmail     string            `json:"contact_email"`
	FirstName        string            `json:"first_name"`
	Initiator        string            `json:"initiator"`
	ReuseContact     bool              `json:"reuse_contact"`
	CustomAttributes map[string]string `json:"custom_attributes,omitempty"`
}

type libreDeskConversationResponse struct {
	Status string `json:"status"`
	Data   struct {
		UUID string `json:"uuid"`
		ID   int64  `json:"id"`
	} `json:"data"`
	Message string `json:"message"`
}

type conversationLink struct {
	ID         string    `json:"id"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	URL        string    `json:"url"`
	Label      string    `json:"label"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *Server) libreDeskEnabled() bool {
	return s.cfg.LibreDeskBaseURL != "" && s.cfg.LibreDeskAPIKey != "" && s.cfg.LibreDeskAPISecret != "" && s.cfg.LibreDeskInboxID > 0
}

func (s *Server) createLibreDeskConversation(targetType, targetID, subject, contactName, contactValue, content string) string {
	if !s.libreDeskEnabled() {
		return ""
	}
	if targetType != "public_request" && targetType != "node_enrollment_request" {
		s.logger.Error("libredesk conversation target type is invalid", "target_type", targetType, "target_id", targetID)
		return ""
	}
	contactEmail := s.cfg.LibreDeskFallbackContactEmail
	contactMissing := true
	if address, err := mail.ParseAddress(strings.TrimSpace(contactValue)); err == nil && strings.Contains(address.Address, "@") {
		contactEmail = address.Address
		contactMissing = false
	}
	firstName := strings.TrimSpace(contactName)
	if firstName == "" {
		firstName = "申請者"
	}
	// LibreDesk accepts HTML content. Escape all user-submitted text before adding
	// line breaks so request details cannot inject markup into the helpdesk thread.
	content = strings.ReplaceAll(html.EscapeString(content), "\n", "<br>")
	inboxID := s.cfg.LibreDeskInboxID
	payload := libreDeskConversationRequest{
		Subject: subject, Content: content, InboxID: inboxID, ContactEmail: contactEmail,
		FirstName: firstName, Initiator: "contact", ReuseContact: true,
		CustomAttributes: map[string]string{"archive_target_type": targetType, "archive_target_id": targetID},
	}
	if s.cfg.LibreDeskTeamID > 0 {
		teamID := s.cfg.LibreDeskTeamID
		payload.TeamID = &teamID
	}
	if s.cfg.LibreDeskAgentID > 0 {
		agentID := s.cfg.LibreDeskAgentID
		payload.AgentID = &agentID
	}
	body, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("libredesk conversation payload could not be encoded", "target_type", targetType, "target_id", targetID, "error", err)
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.LibreDeskBaseURL+"/api/v1/conversations", bytes.NewReader(body))
	if err != nil {
		s.logger.Error("libredesk conversation request could not be built", "target_type", targetType, "target_id", targetID, "error", err)
		return ""
	}
	token := base64.StdEncoding.EncodeToString([]byte(s.cfg.LibreDeskAPIKey + ":" + s.cfg.LibreDeskAPISecret))
	req.Header.Set("Authorization", "Basic "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("libredesk conversation creation failed", "target_type", targetType, "target_id", targetID, "error", err)
		return ""
	}
	defer resp.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		s.logger.Error("libredesk response could not be read", "target_type", targetType, "target_id", targetID, "error", readErr)
		return ""
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.logger.Error("libredesk conversation creation returned an error", "target_type", targetType, "target_id", targetID, "http_status", resp.StatusCode)
		return ""
	}
	var result libreDeskConversationResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		s.logger.Error("libredesk conversation response was invalid", "target_type", targetType, "target_id", targetID, "error", err)
		return ""
	}
	if strings.TrimSpace(result.Data.UUID) == "" {
		s.logger.Error("libredesk conversation response did not include a UUID", "target_type", targetType, "target_id", targetID)
		return ""
	}
	conversationURL := strings.ReplaceAll(s.cfg.LibreDeskConversationURLTemplate, "{{base_url}}", s.cfg.LibreDeskBaseURL)
	conversationURL = strings.ReplaceAll(conversationURL, "{{uuid}}", result.Data.UUID)
	if _, err := validateHTTPURL(conversationURL); err != nil {
		s.logger.Error("configured libredesk conversation URL is invalid", "target_type", targetType, "target_id", targetID)
		return ""
	}
	if err := s.db.WithContext(context.Background()).Exec(`
		INSERT INTO request_conversation_links (target_type, target_id, url, label, created_at)
		VALUES (?, ?, ?, ?, now()) ON CONFLICT (target_type, target_id, url) DO NOTHING`, targetType, targetID, conversationURL, fmt.Sprintf("LibreDesk 会話 #%s", shortID(result.Data.UUID))).Error; err != nil {
		s.logger.Error("libredesk conversation was created but its link could not be saved", "target_type", targetType, "target_id", targetID, "error", err)
		return ""
	}
	if contactMissing {
		s.addLibreDeskPrivateNote(result.Data.UUID, "【注意】申請者がメールアドレスを入力していないため、LibreDesk の連絡先には運営窓口アドレスを設定しています。申請に記載された連絡先を確認してください。")
	}
	s.logger.Info("libredesk conversation created", "target_type", targetType, "target_id", targetID, "conversation_uuid", result.Data.UUID)
	return result.Data.UUID
}

func shortID(value string) string {
	if len(value) > 8 {
		return value[:8]
	}
	return value
}

func (s *Server) listConversationLinks(c *gin.Context) {
	targetType := c.Query("target_type")
	targetID := c.Query("target_id")
	if !validConversationTargetType(targetType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_type must be public_request or node_enrollment_request"})
		return
	}
	if _, err := uuid.Parse(targetID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid target_id is required"})
		return
	}
	actor, _ := currentUser(c)
	if targetType == "node_enrollment_request" && actor.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "administrator role is required for node enrollment conversation links"})
		return
	}
	if !s.conversationTargetExists(c, targetType, targetID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return
	}
	var items []conversationLink
	if err := s.db.WithContext(c.Request.Context()).Raw(`SELECT id, target_type, target_id, url, label, created_at FROM request_conversation_links WHERE target_type = ? AND target_id = ? ORDER BY created_at ASC`, targetType, targetID).Scan(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list conversation links"})
		return
	}
	if items == nil {
		items = []conversationLink{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) addConversationLink(c *gin.Context) {
	var req struct {
		TargetType string `json:"target_type" binding:"required,oneof=public_request node_enrollment_request"`
		TargetID   string `json:"target_id" binding:"required,uuid"`
		URL        string `json:"url" binding:"required,max=4096"`
		Label      string `json:"label" binding:"max=255"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target, URL, and optional label are required"})
		return
	}
	actor, _ := currentUser(c)
	if req.TargetType == "node_enrollment_request" && actor.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "administrator role is required for node enrollment conversation links"})
		return
	}
	if _, err := uuid.Parse(req.TargetID); err != nil || !s.conversationTargetExists(c, req.TargetType, req.TargetID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return
	}
	cleanURL, err := validateHTTPURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL must be a valid http(s) URL without embedded credentials"})
		return
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		label = "関連する会話"
	}
	id := uuid.NewString()
	if err := s.db.WithContext(c.Request.Context()).Exec(`INSERT INTO request_conversation_links (id, target_type, target_id, url, label, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?, now())`, id, req.TargetType, req.TargetID, cleanURL, label, actor.ID).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			c.JSON(http.StatusConflict, gin.H{"error": "このURLはすでに登録されています"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save conversation link"})
		return
	}
	s.audit(c, &actor, "conversation_link", id, "conversation_link.create", "success", nil)
	c.JSON(http.StatusCreated, gin.H{"id": id, "target_type": req.TargetType, "target_id": req.TargetID, "url": cleanURL, "label": label, "created_at": time.Now().UTC()})
}

func (s *Server) deleteConversationLink(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation link id"})
		return
	}
	var link conversationLink
	if err := s.db.WithContext(c.Request.Context()).Raw(`SELECT id, target_type, target_id, url, label, created_at FROM request_conversation_links WHERE id = ?`, id).Scan(&link).Error; err != nil || link.ID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation link not found"})
		return
	}
	actor, _ := currentUser(c)
	if link.TargetType == "node_enrollment_request" && actor.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "administrator role is required for node enrollment conversation links"})
		return
	}
	if err := s.db.WithContext(c.Request.Context()).Exec(`DELETE FROM request_conversation_links WHERE id = ?`, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete conversation link"})
		return
	}
	s.audit(c, &actor, "conversation_link", id, "conversation_link.delete", "success", nil)
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func validConversationTargetType(value string) bool {
	return value == "public_request" || value == "node_enrollment_request"
}

func (s *Server) conversationTargetExists(c *gin.Context, targetType, targetID string) bool {
	var count int64
	var err error
	switch targetType {
	case "public_request":
		err = s.db.WithContext(c.Request.Context()).Table("public_requests").Where("id = ?", targetID).Count(&count).Error
	case "node_enrollment_request":
		err = s.db.WithContext(c.Request.Context()).Table("node_enrollment_requests").Where("id = ?", targetID).Count(&count).Error
	default:
		return false
	}
	return err == nil && count == 1
}

func (s *Server) addLibreDeskPrivateNote(
	conversationUUID string,
	note string,
) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return nil
	}

	// Escape user-supplied text before converting newlines to HTML.
	message := strings.ReplaceAll(
		html.EscapeString(note),
		"\n",
		"<br>",
	)

	payload := struct {
		Message    string `json:"message"`
		Private    bool   `json:"private"`
		SenderType string `json:"sender_type"`
	}{
		Message:    message,
		Private:    true,
		SenderType: "agent",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode private note: %w", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		8*time.Second,
	)
	defer cancel()

	endpoint := fmt.Sprintf(
		"%s/api/v1/conversations/%s/messages",
		strings.TrimRight(s.cfg.LibreDeskBaseURL, "/"),
		conversationUUID,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("build private note request: %w", err)
	}

	token := base64.StdEncoding.EncodeToString(
		[]byte(
			s.cfg.LibreDeskAPIKey + ":" +
				s.cfg.LibreDeskAPISecret,
		),
	)

	req.Header.Set("Authorization", "Basic "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post private note: %w", err)
	}
	defer resp.Body.Close()

	// Drain a limited response body and close the connection.
	_, readErr := io.Copy(
		io.Discard,
		io.LimitReader(resp.Body, 1<<20),
	)
	if readErr != nil {
		return fmt.Errorf("read private note response: %w", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(
			"private note API returned HTTP %d",
			resp.StatusCode,
		)
	}

	return nil
}
