package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
)

type publicCatalogFile struct {
	ID               string    `json:"id"`
	OriginalFilename string    `json:"original_filename"`
	SizeBytes        int64     `json:"size_bytes"`
	MediaType        *string   `json:"media_type,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type publicCatalogArtifact struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	CreatorName *string             `json:"creator_name,omitempty"`
	SourceURL   string              `json:"source_url"`
	Description *string             `json:"description,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	Files       []publicCatalogFile `json:"files"`
}

// listPublicArtifacts only returns artifacts that currently have at least one
// verified, explicitly-public file on a healthy node.
func (s *Server) listPublicArtifacts(c *gin.Context) {
	page, limit, isAll := pagination(c)
	if !isAll && limit > 100 {
		limit = 100
	}
	cutoff := time.Now().UTC().Add(-2 * time.Minute)
	query := s.db.WithContext(c.Request.Context()).Model(&model.Artifact{}).Where(`EXISTS (
		SELECT 1 FROM file_objects f
		WHERE f.artifact_id = artifacts.id
		  AND f.deleted_at IS NULL
		  AND f.public_download_enabled = TRUE
		  AND f.redistribution_policy = 'allowed'
		  AND f.preservation_status = 'verified'
		  AND f.sha256 ~ '^[0-9a-fA-F]{64}$'
		  AND f.size_bytes >= 0
		  AND EXISTS (
			SELECT 1 FROM placements p JOIN nodes n ON n.id = p.node_id
			WHERE p.file_object_id = f.id AND p.status = 'verified'
			  AND n.status = 'online' AND n.last_seen_at >= ?
		  )
	)`, cutoff)
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		pattern := "%" + q + "%"
		query = query.Where(`(
			artifacts.title ILIKE ? OR COALESCE(artifacts.creator_name, '') ILIKE ?
			OR COALESCE(artifacts.description, '') ILIKE ? OR artifacts.source_url ILIKE ?
			OR EXISTS (
				SELECT 1 FROM file_objects f
				WHERE f.artifact_id = artifacts.id AND f.original_filename ILIKE ?
				  AND f.deleted_at IS NULL
				  AND f.public_download_enabled = TRUE AND f.redistribution_policy = 'allowed'
				  AND f.preservation_status = 'verified' AND f.sha256 ~ '^[0-9a-fA-F]{64}$'
				  AND f.size_bytes >= 0
				  AND EXISTS (
					SELECT 1 FROM placements p JOIN nodes n ON n.id = p.node_id
					WHERE p.file_object_id = f.id AND p.status = 'verified'
					  AND n.status = 'online' AND n.last_seen_at >= ?
				  )
			)
		)`, pattern, pattern, pattern, pattern, pattern, cutoff)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "公開カタログを検索できませんでした。"})
		return
	}
	var artifacts []model.Artifact
	artifactQuery := query.Order("artifacts.title ASC")
	if !isAll {
		artifactQuery = artifactQuery.Offset((page - 1) * limit).Limit(limit)
	}
	if err := artifactQuery.Find(&artifacts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "公開カタログを検索できませんでした。"})
		return
	}
	items := make([]publicCatalogArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		files, err := s.publicCatalogFilesAll(c, artifact.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "公開ファイル一覧を取得できませんでした。"})
			return
		}
		if len(files) == 0 {
			continue
		}
		items = append(items, publicCatalogArtifact{
			ID: artifact.ID, Title: artifact.Title, CreatorName: artifact.CreatorName,
			SourceURL: artifact.SourceURL, Description: artifact.Description,
			CreatedAt: artifact.CreatedAt, Files: files,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "page": page, "limit": limit, "total": total})
}

func (s *Server) getPublicArtifact(c *gin.Context) {
	if _, err := uuid.Parse(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "public artifact not found",
		})
		return
	}

	page, limit, isAll := pagination(c)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if !isAll && limit > 100 {
		limit = 100
	}

	var artifact model.Artifact
	if err := s.db.WithContext(c.Request.Context()).First(
		&artifact, "id = ?", c.Param("id"),
	).Error; err != nil {
		notFoundOrError(c, err, "public artifact")
		return
	}

	total, err := s.publicCatalogFileCount(c, artifact.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "公開ファイル数を取得できませんでした。",
		})
		return
	}
	if total == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "公開成果物が見つかりません。",
		})
		return
	}

	var files []publicCatalogFile
	if isAll {
		files, err = s.publicCatalogFilesAll(c, artifact.ID)
	} else {
		files, err = s.publicCatalogFiles(c, artifact.ID, page, limit)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "公開ファイル一覧を取得できませんでした。",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"item": publicCatalogArtifact{
			ID:          artifact.ID,
			Title:       artifact.Title,
			CreatorName: artifact.CreatorName,
			SourceURL:   artifact.SourceURL,
			Description: artifact.Description,
			CreatedAt:   artifact.CreatedAt,
			Files:       files,
		},
		"files_page":  page,
		"files_limit": limit,
		"files_total": total,
	})
}

func (s *Server) publicCatalogFilesAll(c *gin.Context, artifactID string) ([]publicCatalogFile, error) {
	cutoff := time.Now().UTC().Add(-2 * time.Minute)
	var files []model.FileObject
	err := s.db.WithContext(c.Request.Context()).Raw(`
		SELECT DISTINCT f.*
		FROM file_objects f
		JOIN placements p ON p.file_object_id = f.id AND p.status = 'verified'
		JOIN nodes n ON n.id = p.node_id AND n.status = 'online' AND n.last_seen_at >= ?
		WHERE f.artifact_id = ?
		  AND f.deleted_at IS NULL
		  AND f.public_download_enabled = TRUE
		  AND f.redistribution_policy = 'allowed'
		  AND f.preservation_status = 'verified'
		  AND f.sha256 ~ '^[0-9a-fA-F]{64}$'
		  AND f.size_bytes >= 0
		ORDER BY f.created_at DESC`, cutoff, artifactID).Scan(&files).Error
	if err != nil {
		return nil, err
	}
	result := make([]publicCatalogFile, 0, len(files))
	for _, file := range files {
		result = append(result, publicCatalogFile{
			ID: file.ID, OriginalFilename: file.OriginalFilename,
			SizeBytes: file.SizeBytes, MediaType: file.MediaType, CreatedAt: file.CreatedAt,
		})
	}
	return result, nil
}

func (s *Server) publicCatalogFiles(
	c *gin.Context,
	artifactID string,
	page int,
	limit int,
) ([]publicCatalogFile, error) {
	cutoff := time.Now().UTC().Add(-2 * time.Minute)

	var files []model.FileObject
	err := s.db.WithContext(c.Request.Context()).Raw(`
		SELECT DISTINCT f.*
		FROM file_objects f
		JOIN placements p ON p.file_object_id = f.id AND p.status = 'verified'
		JOIN nodes n ON n.id = p.node_id AND n.status = 'online' AND n.last_seen_at >= ?
		WHERE f.artifact_id = ?
		  AND f.deleted_at IS NULL
		  AND f.public_download_enabled = TRUE
		  AND f.redistribution_policy = 'allowed'
		  AND f.preservation_status = 'verified'
		  AND f.sha256 ~ '^[0-9a-fA-F]{64}$'
		  AND f.size_bytes >= 0
		ORDER BY f.created_at DESC, f.id ASC
		OFFSET ? LIMIT ?`,
		cutoff, artifactID, (page-1)*limit, limit,
	).Scan(&files).Error
	if err != nil {
		return nil, err
	}

	result := make([]publicCatalogFile, 0, len(files))
	for _, file := range files {
		result = append(result, publicCatalogFile{
			ID:               file.ID,
			OriginalFilename: file.OriginalFilename,
			SizeBytes:        file.SizeBytes,
			MediaType:        file.MediaType,
			CreatedAt:        file.CreatedAt,
		})
	}

	return result, nil
}

func (s *Server) publicCatalogFileCount(
	c *gin.Context,
	artifactID string,
) (int64, error) {
	cutoff := time.Now().UTC().Add(-2 * time.Minute)

	var total int64
	err := s.db.WithContext(c.Request.Context()).Raw(`
		SELECT COUNT(DISTINCT f.id)
		FROM file_objects f
		JOIN placements p ON p.file_object_id = f.id AND p.status = 'verified'
		JOIN nodes n ON n.id = p.node_id AND n.status = 'online' AND n.last_seen_at >= ?
		WHERE f.artifact_id = ?
		  AND f.deleted_at IS NULL
		  AND f.public_download_enabled = TRUE
		  AND f.redistribution_policy = 'allowed'
		  AND f.preservation_status = 'verified'
		  AND f.sha256 ~ '^[0-9a-fA-F]{64}$'
		  AND f.size_bytes >= 0`,
		cutoff, artifactID,
	).Scan(&total).Error

	return total, err
}
