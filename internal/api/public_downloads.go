package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"gorm.io/gorm"
)

func (s *Server) publicEligible(c *gin.Context, id string) (model.MFileObject, model.TPlacement, error) {
	var f model.MFileObject
	if err := s.db.WithContext(c.Request.Context()).First(&f, "id = ?", id).Error; err != nil {
		return f, model.TPlacement{}, err
	}
	if !f.PublicDownloadEnabled || f.RedistributionPolicy != "allowed" || f.PreservationStatus != "verified" || f.Sha256 == nil || len(strings.TrimSpace(*f.Sha256)) != 64 || f.SizeBytes < 0 {
		return f, model.TPlacement{}, gorm.ErrRecordNotFound
	}
	var rows []model.TPlacement
	err := s.db.WithContext(c.Request.Context()).Table("placements").Select("placements.*").Joins("JOIN m_nodes ON m_nodes.id = placements.node_id").Where("placements.file_object_id = ? AND placements.status = ? AND nodes.status = ? AND nodes.last_seen_at >= ?", id, "verified", "online", time.Now().Add(-2*time.Minute)).Order("placements.created_at ASC").Find(&rows).Error
	if err != nil {
		return f, model.TPlacement{}, err
	}
	if len(rows) == 0 {
		return f, model.TPlacement{}, gorm.ErrRecordNotFound
	}
	return f, rows[0], nil
}

func (s *Server) createPublicDownloadJob(c *gin.Context) {
	fileID := c.Param("id")
	if _, err := uuid.Parse(fileID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ファイル識別子が正しくありません。"})
		return
	}
	f, _, err := s.publicEligible(c, fileID)
	if err != nil {
		s.writeDownloadAudit("", fileID, nil, "requested", "denied", "file_not_public_or_healthy", 0, 0)
		c.JSON(http.StatusNotFound, gin.H{"error": "このファイルは現在公開されていないため、ダウンロードできません。"})
		return
	}
	token, err := randomSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ダウンロード用トークンを作成できませんでした。"})
		return
	}
	job := downloadJob{FileObjectID: f.ID, Status: "queued", AccessTokenHash: tokenHash(token), ExpiresAt: time.Now().Add(30 * time.Minute)}
	if err := s.db.WithContext(c.Request.Context()).Raw(`INSERT INTO download_jobs (file_object_id,status,access_token_hash,expires_at) VALUES (?, 'queued', ?, now() + interval '30 minutes') RETURNING id,file_object_id,selected_node_id,status,access_token_hash,attempt_count,expires_at,created_at`, f.ID, job.AccessTokenHash).Scan(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ダウンロード処理を開始できませんでした。"})
		return
	}
	s.writeDownloadAudit(job.ID, f.ID, nil, "requested", "accepted", "", 0, 0)
	c.JSON(http.StatusAccepted, gin.H{"job": job, "access_token": token, "status_url": "/api/v1/public/downloads/" + job.ID, "content_url": "/api/v1/public/downloads/" + job.ID + "/content"})
}

func (s *Server) authorizedPublicDownloadJob(c *gin.Context) (downloadJob, bool) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ダウンロード識別子が正しくありません。"})
		return downloadJob{}, false
	}
	header := c.GetHeader("Download-Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "ダウンロードに必要な認証情報がありません。"})
		return downloadJob{}, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	var job downloadJob
	if err := s.db.WithContext(c.Request.Context()).First(&job, "id = ?", id).Error; err != nil || job.ExpiresAt.Before(time.Now()) || tokenHash(token) != job.AccessTokenHash {
		c.JSON(http.StatusNotFound, gin.H{"error": "ダウンロード処理が見つかりません。"})
		return downloadJob{}, false
	}
	return job, true
}

func (s *Server) getPublicDownloadJob(c *gin.Context) {
	job, ok := s.authorizedPublicDownloadJob(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": job.ID, "file_object_id": job.FileObjectID, "status": job.Status, "expires_at": job.ExpiresAt, "size_bytes": job.SizeBytes, "sha256": job.Sha256, "last_error": job.LastError})
}

func (s *Server) downloadPublicContent(c *gin.Context) {
	started := time.Now()
	job, ok := s.authorizedPublicDownloadJob(c)
	if !ok {
		return
	}
	if job.Status != "ready" {
		c.JSON(http.StatusConflict, gin.H{"error": "ダウンロードの準備がまだ完了していません。", "status": job.Status})
		return
	}
	f, _, err := s.publicEligible(c, job.FileObjectID)
	if err != nil {
		s.writeDownloadAudit(job.ID, job.FileObjectID, job.SelectedNodeID, "delivered", "denied", "publication_or_health_changed", 0, time.Since(started))
		c.JSON(http.StatusForbidden, gin.H{"error": "このファイルは公開対象外になったためダウンロードできません。"})
		return
	}
	path := filepath.Join(s.cfg.DeliveryDir, job.ID)
	file, err := os.Open(path)
	if err != nil {
		c.JSON(http.StatusGone, gin.H{"error": "ダウンロード可能な期間が終了しました。"})
		return
	}
	defer file.Close()
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", strings.ReplaceAll(urlQueryEscape(f.OriginalFilename), "+", "%20")))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("X-Content-Type-Options", "nosniff")
	if job.SizeBytes != nil {
		c.Header("Content-Length", fmt.Sprint(*job.SizeBytes))
	}
	written, copyErr := io.Copy(c.Writer, file)
	result := "success"
	reason := ""
	if copyErr != nil {
		result = "failed"
		reason = copyErr.Error()
	}
	s.writeDownloadAudit(job.ID, job.FileObjectID, job.SelectedNodeID, "delivered", result, reason, written, time.Since(started))
}
