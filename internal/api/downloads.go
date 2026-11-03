package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"github.com/nfmmu/uploader-archive/internal/gen/query"
)

type downloadJob struct {
	ID              string     `gorm:"column:id" json:"id"`
	FileObjectID    string     `gorm:"column:file_object_id" json:"file_object_id"`
	SelectedNodeID  *string    `gorm:"column:selected_node_id" json:"selected_node_id,omitempty"`
	Status          string     `gorm:"column:status" json:"status"`
	AccessTokenHash string     `gorm:"column:access_token_hash" json:"-"`
	LeaseToken      *string    `gorm:"column:lease_token" json:"lease_token,omitempty"`
	LeaseUntil      *time.Time `gorm:"column:lease_until" json:"lease_until,omitempty"`
	AttemptCount    int        `gorm:"column:attempt_count" json:"attempt_count"`
	SizeBytes       *int64     `gorm:"column:size_bytes" json:"size_bytes,omitempty"`
	Sha256          *string    `gorm:"column:sha256" json:"sha256,omitempty"`
	LastError       *string    `gorm:"column:last_error" json:"last_error,omitempty"`
	ExpiresAt       time.Time  `gorm:"column:expires_at" json:"expires_at"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"created_at"`
}

func tokenHash(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }

func randomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Server) writeDownloadAudit(jobID, fileID string, nodeID *string, action, result, reason string, bytes int64, duration time.Duration) {
	// contextの取得
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var jobIDPtr *string
	if jobID != "" {
		jobIDPtr = &jobID
	}
	var fileIDPtr *string
	if fileID != "" {
		fileIDPtr = &fileID
	}
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	_ = query.DownloadAuditLog.WithContext(ctx).
		Create(&model.DownloadAuditLog{
			DownloadJobID:  jobIDPtr,
			FileObjectID:   fileIDPtr,
			SelectedNodeID: nodeID,
			Action:         action,
			Result:         result,
			Reason:         reasonPtr,
			BytesSent:      bytes,
			DurationMs:     duration.Milliseconds(),
			CreatedAt:      time.Now(),
		})
}

func (s *Server) fileDownloadIsAvailable(c *gin.Context, id string) (*model.FileObject, *model.Placement, error) {
	// contextの取得
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}

	file, err := query.FileObject.WithContext(ctx).Where(query.FileObject.ID.Eq(id)).First()
	if err != nil {
		return file, nil, err
	}
	if file.PreservationStatus != "verified" || file.Sha256 == nil || len(strings.TrimSpace(*file.Sha256)) != 64 || file.SizeBytes < 0 {
		return file, nil, gorm.ErrRecordNotFound
	}
	placements, err := query.Placement.WithContext(ctx).Where(query.Placement.FileObjectID.Eq(id), query.Placement.Status.Eq("verified")).
		Join(query.Node, query.Node.ID.EqCol(query.Placement.NodeID)).
		Where(query.Node.Status.Eq("online"), query.Node.LastSeenAt.Gte(time.Now().Add(-2*time.Minute))).
		Order(query.Placement.CreatedAt.Asc()).
		Find()
	if err != nil {
		return file, nil, err
	}
	if len(placements) == 0 {
		return file, nil, gorm.ErrRecordNotFound
	}
	return file, placements[0], nil
}

func (s *Server) createDownloadJob(c *gin.Context) {
	fileID := c.Param("id")
	if _, err := uuid.Parse(fileID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}

	// ダウンロード可能かどうかを確認する
	f, _, err := s.fileDownloadIsAvailable(c, fileID)
	if err != nil {
		s.writeDownloadAudit("", fileID, nil, "requested", "denied", "file_not_healthy", 0, 0)
		c.JSON(http.StatusNotFound, gin.H{"error": "file is not available for download"})
		return
	}

	// ダウンロードジョブを作成する
	token, err := randomSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create download token"})
		return
	}
	job := model.DownloadJob{
		FileObjectID:    f.ID,
		Status:          "queued",
		AccessTokenHash: tokenHash(token),
		ExpiresAt:       time.Now().Add(30 * time.Minute),
	}
	err = query.DownloadJob.WithContext(c.Request.Context()).Create(&job)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create download job"})
		return
	}
	s.writeDownloadAudit(job.ID, f.ID, nil, "requested", "accepted", "", 0, 0)
	c.JSON(http.StatusAccepted, gin.H{
		"job":          job,
		"access_token": token,
		"status_url":   "/api/v1/downloads/" + job.ID,
		"content_url":  "/api/v1/downloads/" + job.ID + "/content",
	})
}

func (s *Server) authorizedDownloadJob(c *gin.Context) (*model.DownloadJob, bool) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid download id"})
		return nil, false
	}
	header := c.GetHeader("Download-Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "download bearer token required"})
		return nil, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	job, err := query.DownloadJob.WithContext(c.Request.Context()).
		Where(
			query.DownloadJob.ID.Eq(id),
			query.DownloadJob.AccessTokenHash.Eq(tokenHash(token)),
			query.DownloadJob.ExpiresAt.Gt(time.Now())).
		First()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "download job unavailable"})
		return nil, false
	}
	return job, true
}

func (s *Server) getDownloadJob(c *gin.Context) {
	job, ok := s.authorizedDownloadJob(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": job.ID, "file_object_id": job.FileObjectID, "status": job.Status, "expires_at": job.ExpiresAt, "size_bytes": job.SizeBytes, "sha256": job.Sha256, "last_error": job.LastError})
}

func (s *Server) downloadContent(c *gin.Context) {
	started := time.Now()

	// ダウンロードジョブの認証
	job, ok := s.authorizedDownloadJob(c)
	if !ok {
		return
	}

	// ダウンロードジョブの状態を確認する
	if job.Status != "ready" {
		c.JSON(http.StatusConflict, gin.H{"error": "download is not ready", "status": job.Status})
		return
	}

	// ダウンロード可能かどうかを確認する
	f, _, err := s.fileDownloadIsAvailable(c, job.FileObjectID)
	if err != nil {
		s.writeDownloadAudit(job.ID, job.FileObjectID, job.SelectedNodeID, "delivered", "denied", "health_changed", 0, time.Since(started))
		c.JSON(http.StatusForbidden, gin.H{"error": "file is no longer available"})
		return
	}

	// 配信ファイルの存在を確認する
	path := filepath.Join(s.cfg.DeliveryDir, job.ID)
	file, err := os.Open(path)
	if err != nil {
		c.JSON(http.StatusGone, gin.H{"error": "prepared content expired"})
		return
	}
	defer file.Close()

	// 配信レスポンスを返す
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

func urlQueryEscape(v string) string {
	return url.PathEscape(v)
}

func (s *Server) claimDeliveryJob(c *gin.Context) {
	// ノードの認証
	node, ok := currentNode(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var job model.DownloadJob
	err := query.Q.Transaction(func(tx *query.Query) error {
		var candidate struct {
			ID           string
			FileObjectID string
			StorageKey   string
		}

		now := time.Now()
		lastSeenAfter := now.Add(-2 * time.Minute)

		downloadJobQuery := tx.DownloadJob
		placementQuery := tx.Placement
		nodeQuery := tx.Node
		fileObjectQuery := tx.FileObject

		candidateQuery := downloadJobQuery.WithContext(c.Request.Context()).
			Join(
				fileObjectQuery,
				fileObjectQuery.ID.EqCol(downloadJobQuery.FileObjectID),
			).
			Join(
				placementQuery,
				placementQuery.FileObjectID.EqCol(downloadJobQuery.FileObjectID),
				placementQuery.NodeID.Eq(node.ID),
				placementQuery.Status.Eq("verified"),
			).
			Join(
				nodeQuery,
				nodeQuery.ID.EqCol(placementQuery.NodeID),
				nodeQuery.Status.Eq("online"),
				nodeQuery.LastSeenAt.Gte(lastSeenAfter),
			).
			Where(
				field.Or(
					downloadJobQuery.Status.Eq("queued"),
					field.And(
						downloadJobQuery.Status.Eq("running"),
						downloadJobQuery.LeaseUntil.Lt(now),
						downloadJobQuery.AttemptCount.Lt(3),
					),
				),
				downloadJobQuery.ExpiresAt.Gt(now),
				field.Or(
					downloadJobQuery.SelectedNodeID.IsNull(),
					downloadJobQuery.SelectedNodeID.Neq(node.ID),
				),
				fileObjectQuery.PreservationStatus.Eq("verified"),
			).
			Order(downloadJobQuery.CreatedAt.Asc()).
			Limit(1).
			Select(
				downloadJobQuery.ID,
				downloadJobQuery.FileObjectID,
				placementQuery.StorageKey,
			).
			Clauses(clause.Locking{
				Strength: "UPDATE",
				Table:    clause.Table{Name: "download_jobs"},
				Options:  "SKIP LOCKED",
			})

		err := candidateQuery.Scan(&candidate)
		if err != nil {
			return err
		}
		if candidate.ID == "" {
			return gorm.ErrRecordNotFound
		}
		lease, leaseErr := randomSecret()
		if leaseErr != nil {
			return leaseErr
		}

		result, err := downloadJobQuery.
			WithContext(c.Request.Context()).
			Where(downloadJobQuery.ID.Eq(candidate.ID)).
			Updates(map[string]any{
				"status":           "running",
				"selected_node_id": node.ID,
				"lease_token":      lease[:32],
				"lease_until":      gorm.Expr("now() + interval '2 minutes'"),
				"attempt_count":    gorm.Expr("attempt_count + 1"),
				"updated_at":       gorm.Expr("now()"),
			})
		if err != nil {
			return err
		}

		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		fetchedJob, err := downloadJobQuery.
			WithContext(c.Request.Context()).
			Where(downloadJobQuery.ID.Eq(candidate.ID)).
			First()
		if err != nil {
			return err
		}

		job = *fetchedJob

		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not claim delivery job"})
		return
	}
	storageKey, _ := query.Placement.WithContext(c.Request.Context()).Select(query.Placement.StorageKey).Where(query.Placement.FileObjectID.Eq(job.FileObjectID), query.Placement.NodeID.Eq(node.ID)).First()
	f, _ := query.FileObject.WithContext(c.Request.Context()).Where(query.FileObject.ID.Eq(job.FileObjectID)).First()
	c.JSON(http.StatusOK, gin.H{"job_id": job.ID, "lease_token": job.LeaseToken, "file_object_id": f.ID, "storage_key": storageKey, "size_bytes": f.SizeBytes, "sha256": f.Sha256, "original_filename": f.OriginalFilename})
}

func (s *Server) receiveDeliveryContent(c *gin.Context) {
	started := time.Now()
	node, ok := currentNode(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	jobID := c.Param("jobID")
	lease := c.Query("lease_token")
	var job model.DownloadJob
	if err := s.db.WithContext(c.Request.Context()).First(&job, "id = ? AND selected_node_id = ? AND status = 'running' AND lease_token = ? AND lease_until > now()", jobID, node.ID, lease).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "invalid or expired delivery lease"})
		return
	}
	var f model.FileObject
	if err := s.db.WithContext(c.Request.Context()).First(&f, "id = ?", job.FileObjectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	if _, _, err := s.fileDownloadIsAvailable(c, f.ID); err != nil {
		s.failDelivery(job, "health_changed")
		c.JSON(http.StatusForbidden, gin.H{"error": "file is no longer available"})
		return
	}
	if err := os.MkdirAll(s.cfg.DeliveryDir, 0o750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delivery storage unavailable"})
		return
	}
	tmp, err := os.CreateTemp(s.cfg.DeliveryDir, ".download-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delivery storage unavailable"})
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(c.Request.Body, f.SizeBytes+1))
	closeErr := tmp.Close()
	gotHash := hex.EncodeToString(h.Sum(nil))
	if copyErr != nil || closeErr != nil || n != f.SizeBytes || !strings.EqualFold(gotHash, strings.TrimSpace(*f.Sha256)) {
		s.failDelivery(job, "size_or_sha256_mismatch")
		s.writeDownloadAudit(job.ID, f.ID, &node.ID, "prepared", "failed", "size_or_sha256_mismatch", n, time.Since(started))
		c.JSON(http.StatusBadRequest, gin.H{"error": "content validation failed"})
		return
	}
	if err := os.Rename(tmpName, filepath.Join(s.cfg.DeliveryDir, job.ID)); err != nil {
		s.failDelivery(job, "finalize_failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not finalize content"})
		return
	}
	if err := s.db.WithContext(c.Request.Context()).Exec(`UPDATE download_jobs SET status='ready',size_bytes=?,sha256=?,lease_token=NULL,lease_until=NULL,updated_at=now() WHERE id=? AND status='running' AND lease_token=?::uuid`, n, gotHash, job.ID, lease).Error; err != nil {
		_ = os.Remove(filepath.Join(s.cfg.DeliveryDir, job.ID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not finalize job"})
		return
	}
	s.writeDownloadAudit(job.ID, f.ID, &node.ID, "prepared", "success", "", n, time.Since(started))
	c.Status(http.StatusCreated)
}

func (s *Server) failDelivery(job model.DownloadJob, reason string) {
	if job.LeaseToken == nil || *job.LeaseToken == "" {
		return
	}

	// contextの取得
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	q := query.DownloadJob
	info, err := q.WithContext(ctx).
		Where(
			q.ID.Eq(job.ID),
			q.Status.Eq("running"),
			q.LeaseToken.Eq(*job.LeaseToken),
		).
		Updates(map[string]any{
			"status": gorm.Expr(
				"CASE WHEN attempt_count < 3 AND expires_at > now() " +
					"THEN 'queued' ELSE 'failed' END",
			),
			"last_error":  reason,
			"lease_token": nil,
			"lease_until": nil,
			"updated_at":  gorm.Expr("now()"),
		})
	if err != nil {
		s.logger.Error("failDelivery_query_failed", "job_id", job.ID, "error", err)
		return
	}

	if info.RowsAffected == 0 {
		// 別のリースへ移行済み、または実行中ではないため更新しない。
		return
	}
}

func (s *Server) runDownloadCleanup(c *gin.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		_ = s.db.Exec("UPDATE download_jobs SET status='failed',last_error=COALESCE(last_error, 'attempts_exhausted'),lease_token=NULL,lease_until=NULL,updated_at=now() WHERE status='running' AND lease_until < now() AND attempt_count >= 3").Error
		var expired []downloadJob
		if err := s.db.Where("expires_at <= now() AND status <> ?", "expired").Find(&expired).Error; err != nil {
			s.logger.Error("download_cleanup_query_failed", "error", err)
			continue
		}
		for _, job := range expired {
			if err := os.Remove(filepath.Join(s.cfg.DeliveryDir, job.ID)); err != nil && !os.IsNotExist(err) {
				s.logger.Warn("download_cleanup_file_failed", "job_id", job.ID, "error", err)
			}
			_ = s.db.Exec("UPDATE download_jobs SET status='expired',lease_token=NULL,lease_until=NULL,updated_at=now() WHERE id=? AND expires_at <= now()", job.ID).Error
			s.writeDownloadAudit(job.ID, job.FileObjectID, job.SelectedNodeID, "expired", "success", "job_expired", 0, 0)
		}
	}
}
