package api

import (
	"context"
	"encoding/csv"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gen/field"
	"gorm.io/gorm"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"github.com/nfmmu/uploader-archive/internal/gen/query"
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
	// contextの取得
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	// ページネーションの取得
	page, limit, isAll := pagination(c)

	// クエリの作成
	artifactQuery := query.Artifact.WithContext(ctx)
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		pattern := "%" + q + "%"
		artifactQuery = artifactQuery.
			Where(query.Artifact.Title.Like(pattern)).
			Or(query.Artifact.CreatorName.Like(pattern)).
			Or(query.Artifact.SourceURL.Like(pattern)).
			Or(query.Artifact.Description.Like(pattern))
	}
	total, err := artifactQuery.Count()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not search catalog"})
		return
	}

	// クエリの実行
	artifactQuery = artifactQuery.Order(query.Artifact.CreatedAt.Desc())
	if !isAll {
		artifactQuery = artifactQuery.Offset((page - 1) * limit).Limit(limit)
	}
	artifacts, err := artifactQuery.Find()
	if err != nil {
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

	// データのバリデーション
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title and a valid uploader.jp source_url are required"})
		return
	}
	source, err := validateUploaderURL(req.SourceURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_url must be an HTTPS URL on uploader.jp"})
		return
	}

	// データの作成
	artifact := model.Artifact{
		Title:       strings.TrimSpace(req.Title),
		CreatorName: trimOptional(req.CreatorName),
		SourceURL:   source,
		Description: trimOptional(req.Description),
	}

	// クエリ実行
	if err := query.Artifact.WithContext(c.Request.Context()).Create(&artifact); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create artifact"})
		return
	}
	actor, _ := currentUser(c)
	s.audit(c, &actor, "artifact", artifact.ID, "artifact.create", "success", nil)
	c.JSON(http.StatusCreated, artifact)
}

func (s *Server) getArtifact(c *gin.Context) {
	// contextの取得
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}

	// artifactの取得
	artifactQuery := query.Artifact.WithContext(ctx)
	artifact, err := artifactQuery.Where(query.Artifact.ID.Eq(c.Param("id"))).First()
	if err != nil {
		notFoundOrError(c, err, "artifact")
		return
	}

	// artifactに紐づくファイルの取得
	fileObjectQuery := query.FileObject.WithContext(ctx)
	files, err := fileObjectQuery.
		Where(query.FileObject.ArtifactID.Eq(artifact.ID)).
		Where(query.FileObject.DeletedAt.IsNull()).
		Order(query.FileObject.CreatedAt.Desc()).
		Find()
	if err != nil {
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

	// contextの取得
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}

	// artifactの取得
	artifactQuery := query.Artifact.WithContext(ctx)

	artifact, err := artifactQuery.Where(query.Artifact.ID.Eq(c.Param("id"))).First()
	if err != nil {
		notFoundOrError(c, err, "artifact")
		return
	}
	policy := strings.TrimSpace(req.RedistributionPolicy)
	if policy == "" {
		policy = "unknown"
	}

	// データのバリデーション
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
		_, err := query.User.WithContext(ctx).
			Where(
				query.User.ID.Eq(*req.Representative),
				query.User.DisabledAt.IsNull(),
				query.User.Role.In("reviewer", "admin"),
			).First()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "representative must be an active reviewer or administrator"})
			return
		}
	}

	// ファイルオブジェクトの作成
	file := model.FileObject{
		ArtifactID:            artifact.ID,
		OriginalFilename:      strings.TrimSpace(req.OriginalFilename),
		SizeBytes:             req.SizeBytes,
		Sha256:                req.Sha256,
		MediaType:             trimOptional(req.MediaType),
		SourceURL:             trimOptional(req.SourceURL),
		PreservationStatus:    "uncollected",
		RedistributionPolicy:  policy,
		Representative:        req.Representative,
		PublicDownloadEnabled: false,
	}
	if err := query.FileObject.WithContext(ctx).Create(&file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create file object"})
		return
	}
	actor, _ := currentUser(c)
	s.audit(c, &actor, "file_object", file.ID, "file_object.create", "success", nil)
	c.JSON(http.StatusCreated, file)
}

type fileListItem struct {
	model.FileObject
	ArtifactTitle       string  `json:"artifact_title"`
	ArtifactSourceURL   string  `json:"artifact_source_url"`
	CreatorName         *string `json:"creator_name,omitempty"`
	ArtifactDescription *string `json:"artifact_description,omitempty"`
}

func (s *Server) listFiles(c *gin.Context) {
	page, limit, isAll := pagination(c)

	// contextの取得
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}

	fileObjectQuery := query.FileObject
	targetQuery := fileObjectQuery.WithContext(ctx)
	if c.Query("include_deleted") != "true" {
		targetQuery = targetQuery.Where(fileObjectQuery.DeletedAt.IsNull(), fileObjectQuery.DeletionRequestedAt.IsNull())
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		pattern := "%" + q + "%"
		artifactQuery := query.Artifact

		/*query = query.Where(`(
		    	CAST(id AS TEXT) ILIKE ?
		    	OR original_filename ILIKE ?
		    	OR COALESCE(sha256, '') ILIKE ?
		    	OR artifact_id IN (
		    	    SELECT id FROM artifacts
		    	    WHERE CAST(id AS TEXT) ILIKE ?
		    	       OR title ILIKE ?
		    	       OR COALESCE(creator_name, '') ILIKE ?
		    	)
				)`, pattern, pattern, pattern, pattern, pattern, pattern)*/
		sub := artifactQuery.WithContext(ctx).
			Where(artifactQuery.ID.Like(pattern)).
			Or(artifactQuery.Title.Like(pattern)).
			Or(artifactQuery.CreatorName.Like(pattern)).
			Select(artifactQuery.ID).
			UnderlyingDB()

		targetQuery = targetQuery.Where(field.Or(
			fileObjectQuery.ID.Like(pattern),
			fileObjectQuery.OriginalFilename.Like(pattern),
			fileObjectQuery.Sha256.Like(pattern),
			field.ContainsSubQuery(
				[]field.Expr{fileObjectQuery.ArtifactID},
				sub,
			),
		))

	}
	if artifactID := strings.TrimSpace(c.Query("artifact_id")); artifactID != "" {
		if _, err := uuid.Parse(artifactID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid artifact_id"})
			return
		}
		targetQuery = targetQuery.Where(fileObjectQuery.ArtifactID.Eq(artifactID))
	}
	if status := c.Query("preservation_status"); status != "" {
		if !preservationStatuses[status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid preservation_status"})
			return
		}
		targetQuery = targetQuery.Where(fileObjectQuery.PreservationStatus.Eq(status))
	}
	if policy := c.Query("redistribution_policy"); policy != "" {
		if !redistributionPolicies[policy] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid redistribution_policy"})
			return
		}
		targetQuery = targetQuery.Where(fileObjectQuery.RedistributionPolicy.Eq(policy))
	}
	if publication := c.Query("publication"); publication != "" {
		switch publication {
		case "public":
			cutoff := time.Now().UTC().Add(-2 * time.Minute)
			placementQuery := query.Placement
			nodeQuery := query.Node

			// ノードとプレースメントのサブクエリを作成して、公開可能なファイルをフィルタリング
			nodeSub := nodeQuery.WithContext(ctx).
				Select(nodeQuery.ID).
				Where(
					nodeQuery.Status.Eq("online"),
					nodeQuery.LastSeenAt.Gte(cutoff),
				).
				UnderlyingDB()
			placementsSub := placementQuery.WithContext(ctx).
				Select(placementQuery.ID).
				Where(
					placementQuery.Status.Eq("verified"),
					placementQuery.FileObjectID.EqCol(query.FileObject.ID),
					field.ContainsSubQuery(
						[]field.Expr{placementQuery.NodeID},
						nodeSub,
					),
				).
				UnderlyingDB()
			targetQuery = targetQuery.Where(
				field.And(
					query.FileObject.PublicDownloadEnabled.Is(true),
					query.FileObject.RedistributionPolicy.Eq("allowed"),
					query.FileObject.PreservationStatus.Eq("verified"),
					query.FileObject.Sha256.Regexp("^[0-9a-fA-F]{64}$"),
					field.CompareSubQuery(field.ExistsOp, nil, placementsSub),
				),
			)
		case "private":
			targetQuery = targetQuery.Where(fileObjectQuery.PublicDownloadEnabled.Eq(false))
		case "restricted":
			targetQuery = targetQuery.Where(fileObjectQuery.RedistributionPolicy.Eq("restricted"))
		case "pending":
			targetQuery = targetQuery.Where(fileObjectQuery.RedistributionPolicy.Eq("pending"))
		case "unknown":
			targetQuery = targetQuery.Where(fileObjectQuery.RedistributionPolicy.Eq("unknown"))
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid publication filter"})
			return
		}
	}
	total, err := targetQuery.Count()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not search files"})
		return
	}

	// ファイル取得
	targetQuery = targetQuery.Order(fileObjectQuery.CreatedAt.Desc())
	if !isAll {
		targetQuery = targetQuery.Offset((page - 1) * limit).Limit(limit)
	}
	files, err := targetQuery.Find()
	if err != nil {
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
	artifacts := map[string]model.Artifact{}

	// バッチサイズを定義して、アーティファクトIDのリストを分割して処理
	const artifactBatchSize = 1000
	for start := 0; start < len(artifactIDs); start += artifactBatchSize {
		end := min(start+artifactBatchSize, len(artifactIDs))
		records, err := query.Artifact.WithContext(ctx).Where(query.Artifact.ID.In(artifactIDs[start:end]...)).Find()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load file metadata"})
			return
		}
		for _, artifact := range records {
			if artifact != nil {
				artifacts[artifact.ID] = *artifact
			}
		}
	}

	// ファイルリストアイテムの作成
	items := make([]fileListItem, 0, len(files))
	for _, file := range files {
		artifact := artifacts[file.ArtifactID]
		if file != nil {
			items = append(items, fileListItem{
				FileObject:          *file,
				ArtifactTitle:       artifact.Title,
				ArtifactSourceURL:   artifact.SourceURL,
				CreatorName:         artifact.CreatorName,
				ArtifactDescription: artifact.Description,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "page": page, "limit": limit, "total": total})
}

func (s *Server) getFileObject(c *gin.Context) {
	// contextの取得
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}

	file, err := query.FileObject.WithContext(ctx).Where(query.FileObject.ID.Eq(c.Param("id"))).First()
	if err != nil {
		notFoundOrError(c, err, "file object")
		return
	}

	policies, err := query.Policy.WithContext(ctx).
		Where(query.Policy.FileObjectID.Eq(file.ID)).
		Order(query.Policy.CheckedAt.Desc()).Find()
	if err != nil {
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

	// contextの取得
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}

	// ファイルオブジェクトの取得
	file, err := query.FileObject.WithContext(ctx).Where(query.FileObject.ID.Eq(c.Param("id"))).First()
	if err != nil {
		notFoundOrError(c, err, "file object")
		return
	}

	// データのバリデーション
	actor, _ := currentUser(c)
	if req.Representative != nil {
		_, err := query.User.WithContext(ctx).
			Where(
				query.User.ID.Eq(*req.Representative),
				query.User.DisabledAt.IsNull(),
				query.User.Role.In("reviewer", "admin"),
			).First()
		if err != nil {
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

	// トランザクション内でファイルオブジェクトの更新とポリシー履歴の作成を行う
	err = query.Q.Transaction(func(tx *query.Query) error {
		updates := map[string]any{"redistribution_policy": req.RedistributionPolicy}
		if req.Representative != nil {
			updates["representative"] = *req.Representative
		}
		if req.RedistributionPolicy != "allowed" {
			// public_download_enabledをfalseに設定
			updates["public_download_enabled"] = false
		}
		if _, err := tx.FileObject.WithContext(ctx).Where(query.FileObject.ID.Eq(file.ID)).Updates(updates); err != nil {
			return err
		}
		record := model.Policy{
			FileObjectID: file.ID,
			PolicyType:   req.RedistributionPolicy,
			EvidenceURL:  evidenceURL,
			EvidenceNote: trimOptional(req.EvidenceNote),
			CheckedAt:    checkedAtPtr,
			CheckedBy:    checkedBy,
		}
		return tx.Policy.WithContext(ctx).Create(&record)
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update policy"})
		return
	}

	// レスポンスの作成
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

	// contextの取得
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}

	// ファイルオブジェクトの取得
	file, err := query.FileObject.WithContext(ctx).Where(query.FileObject.ID.Eq(c.Param("id")), query.FileObject.DeletedAt.IsNull(), query.FileObject.DeletionRequestedAt.IsNull()).First()
	if err != nil {
		notFoundOrError(c, err, "file object")
		return
	}

	// 公開フラグを有効にする場合の条件チェック
	if *req.Enabled {
		// ポリシーが許可されており、保存状態が検証済みであり、有効なSHA-256が記録されているか
		if file.RedistributionPolicy != "allowed" || file.PreservationStatus != "verified" || file.Sha256 == nil || !isHexDigest(*file.Sha256) {
			c.JSON(http.StatusConflict, gin.H{"error": "cannot enable public download unless policy is allowed, preservation is verified, and a valid SHA-256 is recorded"})
			return
		}

		// 健康な検証済みプレースメントが存在するかを確認
		cutoff := time.Now().UTC().Add(-2 * time.Minute)
		var healthy int64
		placementQuery := query.Placement
		nodeQuery := query.Node
		nodeSub := nodeQuery.WithContext(ctx).
			Select(nodeQuery.ID).
			Where(
				nodeQuery.Status.Eq("online"),
				nodeQuery.LastSeenAt.Gte(cutoff),
			).
			UnderlyingDB()
		placementsSub := placementQuery.WithContext(ctx).
			Select(placementQuery.ID).
			Where(
				placementQuery.Status.Eq("verified"),
				placementQuery.FileObjectID.EqCol(query.FileObject.ID),
				field.ContainsSubQuery(
					[]field.Expr{placementQuery.NodeID},
					nodeSub,
				),
			).
			UnderlyingDB()
		healthy, err := query.FileObject.WithContext(ctx).
			Where(query.FileObject.ID.Eq(file.ID)).
			Where(field.CompareSubQuery(field.ExistsOp, nil, placementsSub)).
			Count()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check healthy placements"})
			return
		}
		if healthy == 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "cannot enable public download without a healthy verified placement"})
			return
		}
	}

	// 公開フラグの更新
	actor, _ := currentUser(c)
	if _, err := query.FileObject.WithContext(ctx).Where(query.FileObject.ID.Eq(file.ID)).Update(query.FileObject.PublicDownloadEnabled, *req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update publication flag"})
		return
	}
	file.PublicDownloadEnabled = *req.Enabled
	s.audit(c, &actor, "file_object", file.ID, "file_object.publication_flag.update", "success", &req.Reason)

	// レスポンスの作成
	c.JSON(http.StatusOK, gin.H{"file": file})
}

func (s *Server) exportCatalog(c *gin.Context) {
	format := strings.ToLower(c.DefaultQuery("format", "json"))

	// contextの取得
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}

	// アーティファクトとファイルの取得
	artifacts, err := query.Artifact.WithContext(ctx).Order(query.Artifact.CreatedAt.Asc()).Find()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not export artifacts"})
		return
	}
	files, err := query.FileObject.WithContext(ctx).Where(query.FileObject.DeletedAt.IsNull()).Order(query.FileObject.CreatedAt.Asc()).Find()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not export files"})
		return
	}

	// レスポンスの作成
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
	_ = writer.Write([]string{
		"record_type",
		"id",
		"artifact_id",
		"title",
		"creator_name",
		"source_url",
		"original_filename",
		"size_bytes",
		"sha256",
		"preservation_status",
		"redistribution_policy",
		"public_download_enabled",
		"created_at",
	})
	for _, a := range artifacts {
		_ = writer.Write([]string{
			"artifact",
			a.ID,
			"",
			csvSafe(a.Title),
			csvSafe(deref(a.CreatorName)),
			csvSafe(a.SourceURL),
			"", "", "", "", "", "",
			a.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	for _, f := range files {
		_ = writer.Write([]string{
			"file",
			f.ID,
			f.ArtifactID,
			"", "", "",
			csvSafe(f.OriginalFilename),
			strconv.FormatInt(f.SizeBytes, 10),
			deref(f.Sha256),
			f.PreservationStatus,
			f.RedistributionPolicy,
			strconv.FormatBool(f.PublicDownloadEnabled),
			f.CreatedAt.UTC().Format(time.RFC3339),
		})
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
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": label + " not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load " + label})
}

var errInvalidURL = errors.New("invalid URL")
