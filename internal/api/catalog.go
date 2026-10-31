package api

import (
	"encoding/csv"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"gorm.io/gorm"
)

var preservationStatuses = map[string]bool{
	"uncollected": true, "queued": true, "stored": true, "verified": true,
	"degraded": true, "missing": true, "error": true,
}

var redistributionPolicies = map[string]bool{
	"allowed": true, "restricted": true, "in_progress": true, "pending": true, "unknown": true,
}

// pending is the explicit "unconfirmed" state. Other policy states may be
// preserved, but public redistribution still requires the separate allowed
// policy and publication checks.
func isUnconfirmedRedistributionPolicy(policy string) bool {
	return policy == "pending"
}

func (s *Server) listArtifacts(c *gin.Context) {
	page, limit, isAll := pagination(c)
	query := s.db.WithContext(c.Request.Context()).Model(&model.MArtifact{})
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		pattern := "%" + q + "%"
		query = query.Where("title ILIKE ? OR creator_name ILIKE ? OR source_url ILIKE ?", pattern, pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not search catalog"})
		return
	}
	var artifacts []model.MArtifact
	artifactQuery := query.Order("created_at DESC")
	if !isAll {
		artifactQuery = artifactQuery.Offset((page - 1) * limit).Limit(limit)
	}
	if err := artifactQuery.Find(&artifacts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not search catalog"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": artifacts, "page": page, "limit": limit, "total": total})
}

func (s *Server) createArtifact(c *gin.Context) {
	var req struct {
		Title       string  `json:"title" binding:"required,min=1,max=500"`
		CreatorName *string `json:"creator_name" binding:"omitempty,max=255"`
		SourceURL   string  `json:"source_url" binding:"required,max=4096"`
		Description *string `json:"description" binding:"omitempty,max=20000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title and a valid uploader.jp source_url are required"})
		return
	}
	source, err := validateUploaderURL(req.SourceURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_url must be an HTTPS URL on uploader.jp"})
		return
	}
	artifact := model.MArtifact{Title: strings.TrimSpace(req.Title), CreatorName: trimOptional(req.CreatorName), SourceURL: source, Description: trimOptional(req.Description)}
	if err := s.db.WithContext(c.Request.Context()).Create(&artifact).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create artifact"})
		return
	}
	actor, _ := currentUser(c)
	s.audit(c, &actor, "artifact", artifact.ID, "artifact.create", "success", nil)
	c.JSON(http.StatusCreated, artifact)
}

func (s *Server) getArtifact(c *gin.Context) {
	var artifact model.MArtifact
	err := s.db.WithContext(c.Request.Context()).First(&artifact, "id = ?", c.Param("id")).Error
	if err != nil {
		notFoundOrError(c, err, "artifact")
		return
	}
	var files []model.MFileObject
	if err := s.db.WithContext(c.Request.Context()).Where("artifact_id = ? AND deleted_at IS NULL", artifact.ID).Order("created_at DESC").Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load artifact files"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"artifact": artifact, "files": files})
}

func (s *Server) createFileObject(c *gin.Context) {
	var req struct {
		OriginalFilename     string  `json:"original_filename" binding:"required,min=1,max=500"`
		SizeBytes            int64   `json:"size_bytes" binding:"gte=0"`
		Sha256               *string `json:"sha256" binding:"omitempty,len=64"`
		MediaType            *string `json:"media_type" binding:"omitempty,max=255"`
		SourceURL            *string `json:"source_url" binding:"omitempty,max=4096"`
		RedistributionPolicy string  `json:"redistribution_policy" binding:"omitempty"`
		Representative       *string `json:"representative" binding:"omitempty,uuid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file object payload"})
		return
	}
	var artifact model.MArtifact
	if err := s.db.WithContext(c.Request.Context()).First(&artifact, "id = ?", c.Param("id")).Error; err != nil {
		notFoundOrError(c, err, "artifact")
		return
	}
	policy := strings.TrimSpace(req.RedistributionPolicy)
	if policy == "" {
		policy = "unknown"
	}
	if !redistributionPolicies[policy] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid redistribution_policy"})
		return
	}
	if req.SourceURL != nil {
		validated, err := validateUploaderURL(*req.SourceURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "source_url must be an HTTPS uploader.jp/getuploader.com URL"})
			return
		}
		req.SourceURL = &validated
	}
	if req.Sha256 != nil {
		value := strings.ToLower(strings.TrimSpace(*req.Sha256))
		if !isHexDigest(value) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "sha256 must be 64 hexadecimal characters"})
			return
		}
		req.Sha256 = &value
	}
	if req.Representative != nil {
		var reviewer model.MUser
		if err := s.db.WithContext(c.Request.Context()).First(&reviewer, "id = ?", *req.Representative).Error; err != nil || reviewer.DisabledAt != nil || (reviewer.Role != "reviewer" && reviewer.Role != "admin") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "representative must be an active reviewer or administrator"})
			return
		}
	}
	file := model.MFileObject{
		ArtifactID: artifact.ID, OriginalFilename: strings.TrimSpace(req.OriginalFilename), SizeBytes: req.SizeBytes,
		Sha256: req.Sha256, MediaType: trimOptional(req.MediaType), SourceURL: trimOptional(req.SourceURL), PreservationStatus: "uncollected",
		RedistributionPolicy: policy, Representative: req.Representative, PublicDownloadEnabled: false,
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&file).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create file object"})
		return
	}
	actor, _ := currentUser(c)
	s.audit(c, &actor, "file_object", file.ID, "file_object.create", "success", nil)
	c.JSON(http.StatusCreated, file)
}

type fileListItem struct {
	model.MFileObject
	ArtifactTitle       string  `json:"artifact_title"`
	ArtifactSourceURL   string  `json:"artifact_source_url"`
	CreatorName         *string `json:"creator_name,omitempty"`
	ArtifactDescription *string `json:"artifact_description,omitempty"`
}

func (s *Server) listFiles(c *gin.Context) {
	page, limit, isAll := pagination(c)
	db := s.db.WithContext(c.Request.Context())
	query := db.Model(&model.MFileObject{})
	if c.Query("include_deleted") != "true" {
		query = query.Where("deleted_at IS NULL")
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		pattern := "%" + q + "%"
		query = query.Where(`(
    	CAST(id AS TEXT) ILIKE ?
    	OR original_filename ILIKE ?
    	OR COALESCE(sha256, '') ILIKE ?
    	OR artifact_id IN (
    	    SELECT id FROM artifacts
    	    WHERE CAST(id AS TEXT) ILIKE ?
    	       OR title ILIKE ?
    	       OR COALESCE(creator_name, '') ILIKE ?
    	)
		)`, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	if artifactID := strings.TrimSpace(c.Query("artifact_id")); artifactID != "" {
		if _, err := uuid.Parse(artifactID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid artifact_id"})
			return
		}
		query = query.Where("artifact_id = ?", artifactID)
	}
	if status := c.Query("preservation_status"); status != "" {
		if !preservationStatuses[status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid preservation_status"})
			return
		}
		query = query.Where("preservation_status = ?", status)
	}
	if policy := c.Query("redistribution_policy"); policy != "" {
		if !redistributionPolicies[policy] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid redistribution_policy"})
			return
		}
		query = query.Where("redistribution_policy = ?", policy)
	}
	if publication := c.Query("publication"); publication != "" {
		switch publication {
		case "public":
			cutoff := time.Now().UTC().Add(-2 * time.Minute)
			query = query.Where(`public_download_enabled = TRUE AND redistribution_policy = 'allowed' AND preservation_status = 'verified' AND sha256 ~ '^[0-9a-fA-F]{64}$' AND EXISTS (
				SELECT 1 FROM t_placements p JOIN m_nodes n ON n.id = p.node_id
				WHERE p.file_object_id = file_objects.id AND p.status = 'verified' AND n.status = 'online' AND n.last_seen_at >= ?
			)`, cutoff)
		case "private":
			query = query.Where("public_download_enabled = FALSE")
		case "restricted":
			query = query.Where("redistribution_policy = 'restricted'")
		case "pending":
			query = query.Where("redistribution_policy = 'pending'")
		case "unknown":
			query = query.Where("redistribution_policy = 'unknown'")
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid publication filter"})
			return
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not search files"})
		return
	}
	// ファイル取得
	var files []model.MFileObject
	fileQuery := query.Order("created_at DESC")
	if !isAll {
		fileQuery = fileQuery.Offset((page - 1) * limit).Limit(limit)
	}
	if err := fileQuery.Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not search files"})
		return
	}
	artifactIDs := make([]string, 0, len(files))
	seen := map[string]bool{}
	for _, file := range files {
		if !seen[file.ArtifactID] {
			seen[file.ArtifactID] = true
			artifactIDs = append(artifactIDs, file.ArtifactID)
		}
	}
	artifacts := map[string]model.MArtifact{}
	// Bound the number of parameters even when all files are requested.
	const artifactBatchSize = 1000
	for start := 0; start < len(artifactIDs); start += artifactBatchSize {
		end := min(start+artifactBatchSize, len(artifactIDs))
		var records []model.MArtifact
		if err := db.Where("id IN ?", artifactIDs[start:end]).Find(&records).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load file metadata"})
			return
		}
		for _, artifact := range records {
			artifacts[artifact.ID] = artifact
		}
	}
	items := make([]fileListItem, 0, len(files))
	for _, file := range files {
		artifact := artifacts[file.ArtifactID]
		items = append(items, fileListItem{MFileObject: file, ArtifactTitle: artifact.Title, ArtifactSourceURL: artifact.SourceURL, CreatorName: artifact.CreatorName, ArtifactDescription: artifact.Description})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "page": page, "limit": limit, "total": total})
}

func (s *Server) getFileObject(c *gin.Context) {
	var file model.MFileObject
	if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ? AND deleted_at IS NULL AND deletion_requested_at IS NULL", c.Param("id")).Error; err != nil {
		notFoundOrError(c, err, "file object")
		return
	}
	var policies []model.TPolicy
	if err := s.db.WithContext(c.Request.Context()).Where("file_object_id = ?", file.ID).Order("created_at DESC").Find(&policies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load policy history"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"file": file, "policy_history": policies})
}

func (s *Server) updateFilePolicy(c *gin.Context) {
	var req struct {
		RedistributionPolicy string  `json:"redistribution_policy" binding:"required"`
		Representative       *string `json:"representative" binding:"omitempty,uuid"`
		EvidenceURL          *string `json:"evidence_url" binding:"omitempty,max=4096"`
		EvidenceNote         *string `json:"evidence_note" binding:"omitempty,max=20000"`
		Reason               string  `json:"reason" binding:"required,min=3,max=2000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !redistributionPolicies[req.RedistributionPolicy] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid redistribution_policy and reason are required"})
		return
	}
	var file model.MFileObject
	if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ? AND deleted_at IS NULL AND deletion_requested_at IS NULL", c.Param("id")).Error; err != nil {
		notFoundOrError(c, err, "file object")
		return
	}
	actor, _ := currentUser(c)
	if req.Representative != nil {
		var reviewer model.MUser
		if err := s.db.WithContext(c.Request.Context()).First(&reviewer, "id = ?", *req.Representative).Error; err != nil || reviewer.DisabledAt != nil || (reviewer.Role != "reviewer" && reviewer.Role != "admin") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "representative must be an active reviewer or administrator"})
			return
		}
	}
	var evidenceURL *string
	if req.EvidenceURL != nil {
		if strings.TrimSpace(*req.EvidenceURL) != "" {
			if _, err := validateHTTPURL(*req.EvidenceURL); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "evidence_url must be an http(s) URL"})
				return
			}
		}
		evidenceURL = trimOptional(req.EvidenceURL)
	}
	checkedAt := time.Now().UTC()
	var checkedAtPtr *time.Time
	var checkedBy *string
	if req.RedistributionPolicy == "allowed" || req.RedistributionPolicy == "restricted" {
		checkedAtPtr = &checkedAt
		checkedBy = &actor.ID
	}
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"redistribution_policy": req.RedistributionPolicy}
		if req.Representative != nil {
			updates["representative"] = *req.Representative
		}
		if req.RedistributionPolicy != "allowed" {
			// Fail closed immediately when a policy is no longer explicitly allowed.
			updates["public_download_enabled"] = false
		}
		if err := tx.Model(&model.MFileObject{}).Where("id = ?", file.ID).Updates(updates).Error; err != nil {
			return err
		}
		record := model.TPolicy{FileObjectID: file.ID, PolicyType: req.RedistributionPolicy, EvidenceURL: evidenceURL, EvidenceNote: trimOptional(req.EvidenceNote), CheckedAt: checkedAtPtr, CheckedBy: checkedBy}
		return tx.Create(&record).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update policy"})
		return
	}
	file.RedistributionPolicy = req.RedistributionPolicy
	if req.Representative != nil {
		file.Representative = req.Representative
	}
	if req.RedistributionPolicy != "allowed" {
		file.PublicDownloadEnabled = false
	}
	s.audit(c, &actor, "file_object", file.ID, "file_object.policy.update", "success", &req.Reason)
	c.JSON(http.StatusOK, gin.H{"file": file, "policy": req.RedistributionPolicy})
}

func (s *Server) updatePublicationFlag(c *gin.Context) {
	var req struct {
		Enabled *bool  `json:"enabled" binding:"required"`
		Reason  string `json:"reason" binding:"required,min=3,max=2000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enabled and reason are required"})
		return
	}
	var file model.MFileObject
	if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ? AND deleted_at IS NULL AND deletion_requested_at IS NULL", c.Param("id")).Error; err != nil {
		notFoundOrError(c, err, "file object")
		return
	}
	if *req.Enabled {
		if file.RedistributionPolicy != "allowed" || file.PreservationStatus != "verified" || file.Sha256 == nil || !isHexDigest(*file.Sha256) {
			c.JSON(http.StatusConflict, gin.H{"error": "cannot enable public download unless policy is allowed, preservation is verified, and a valid SHA-256 is recorded"})
			return
		}
		cutoff := time.Now().UTC().Add(-2 * time.Minute)
		var healthy int64
		if err := s.db.WithContext(c.Request.Context()).Table("placements AS p").Joins("JOIN m_nodes AS n ON n.id = p.node_id").Where("p.file_object_id = ? AND p.status = 'verified' AND n.status = 'online' AND n.last_seen_at >= ?", file.ID, cutoff).Count(&healthy).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check healthy placements"})
			return
		}
		if healthy == 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "cannot enable public download without a healthy verified placement"})
			return
		}
	}
	actor, _ := currentUser(c)
	if err := s.db.WithContext(c.Request.Context()).Model(&model.MFileObject{}).Where("id = ?", file.ID).Update("public_download_enabled", *req.Enabled).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update publication flag"})
		return
	}
	file.PublicDownloadEnabled = *req.Enabled
	s.audit(c, &actor, "file_object", file.ID, "file_object.publication_flag.update", "success", &req.Reason)
	c.JSON(http.StatusOK, gin.H{"file": file})
}

func (s *Server) exportCatalog(c *gin.Context) {
	format := strings.ToLower(c.DefaultQuery("format", "json"))
	var artifacts []model.MArtifact
	if err := s.db.WithContext(c.Request.Context()).Order("created_at ASC").Find(&artifacts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not export artifacts"})
		return
	}
	var files []model.MFileObject
	if err := s.db.WithContext(c.Request.Context()).Where("deleted_at IS NULL").Order("created_at ASC").Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not export files"})
		return
	}
	actor, _ := currentUser(c)
	s.audit(c, &actor, "catalog", "all", "catalog.export", "success", nil)
	if format == "json" {
		c.JSON(http.StatusOK, gin.H{"artifacts": artifacts, "files": files, "exported_at": time.Now().UTC()})
		return
	}
	if format != "csv" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format must be json or csv"})
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=archive-catalog.csv")
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"record_type", "id", "artifact_id", "title", "creator_name", "source_url", "original_filename", "size_bytes", "sha256", "preservation_status", "redistribution_policy", "public_download_enabled", "created_at"})
	for _, a := range artifacts {
		_ = writer.Write([]string{"artifact", a.ID, "", csvSafe(a.Title), csvSafe(deref(a.CreatorName)), csvSafe(a.SourceURL), "", "", "", "", "", "", a.CreatedAt.UTC().Format(time.RFC3339)})
	}
	for _, f := range files {
		_ = writer.Write([]string{"file", f.ID, f.ArtifactID, "", "", "", csvSafe(f.OriginalFilename), strconv.FormatInt(f.SizeBytes, 10), deref(f.Sha256), f.PreservationStatus, f.RedistributionPolicy, strconv.FormatBool(f.PublicDownloadEnabled), f.CreatedAt.UTC().Format(time.RFC3339)})
	}
	writer.Flush()
}

func pagination(c *gin.Context) (int, int, bool) {
	isAll, _ := strconv.ParseBool(c.DefaultQuery("all", "false"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if isAll {
		return 0, 0, true
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit, false
}

func validateUploaderURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil ||
		u.Scheme != "https" ||
		u.User != nil ||
		u.Hostname() == "" ||
		(u.Port() != "" && u.Port() != "443") {
		return "", errInvalidURL
	}

	if !isAllowedUploaderHost(u.Hostname()) {
		return "", errInvalidURL
	}

	return u.String(), nil
}

func validateHTTPURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Hostname() == "" {
		return "", errInvalidURL
	}
	return u.String(), nil
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func csvSafe(value string) string {
	trimmed := strings.TrimLeft(value, "\t\r\n ")
	if len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func isHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func notFoundOrError(c *gin.Context, err error, label string) {
	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": label + " not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load " + label})
}

var errInvalidURL = errors.New("invalid URL")
