package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var requestTypes = map[string]bool{"takedown": true, "deletion": true, "external_publication": true, "preservation": true, "redistribution_assistance": true, "publisher_assistance": true}
var requestStatuses = map[string]bool{"pending": true, "in_review": true, "accepted": true, "rejected": true, "closed": true}

func (s *Server) submitPublicRequest(c *gin.Context) {
	var req struct {
		RequestType    string  `json:"request_type" binding:"required"`
		TurnstileToken string  `json:"turnstile_token"`
		ArtifactID     *string `json:"artifact_id" binding:"omitempty,uuid"`
		FileObjectID   *string `json:"file_object_id" binding:"omitempty,uuid"`
		Title          *string `json:"title" binding:"omitempty,max=500"`
		CreatorName    *string `json:"creator_name" binding:"omitempty,max=255"`
		SourceURL      *string `json:"source_url" binding:"omitempty,max=4096"`
		SourcePassword string  `json:"source_password" binding:"omitempty,max=1024"`
		SubmitterName  *string `json:"submitter_name" binding:"omitempty,max=255"`
		Contact        *string `json:"contact" binding:"omitempty,max=500"`
		Details        string  `json:"details" binding:"required,max=10000"`
		ExternalURL    *string `json:"external_url" binding:"omitempty,max=4096"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !requestTypes[req.RequestType] || req.RequestType == "redistribution_assistance" || req.RequestType == "publisher_assistance" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "申請種類と申請内容を正しく入力してください。"})
		return
	}
	if !s.verifyTurnstile(c, req.TurnstileToken) {
		return
	}
	if req.RequestType == "preservation" {
		if req.SourceURL == nil || strings.TrimSpace(*req.SourceURL) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "保存申請ではアップロード元URLが必要です。"})
			return
		}
		validated, err := normalizeUploaderURL(*req.SourceURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "アップロード元URLは、uploader.jpまたはgetuploader.comのHTTPS URLを指定してください。"})
			return
		}
		req.SourceURL = &validated
		if req.Title == nil || strings.TrimSpace(*req.Title) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "保存申請ではタイトルを入力してください。"})
			return
		}
	}
	var encryptedSourcePassword *string
	if req.SourcePassword != "" {
		if req.RequestType != "preservation" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ソースパスワードは保存申請でのみ指定できます。"})
			return
		}
		var err error
		encryptedSourcePassword, err = s.encryptSourcePassword(req.SourcePassword)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
	}
	if req.RequestType == "external_publication" {
		if req.ExternalURL == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "外部公開申請では公開先URLを入力してください。"})
			return
		}
		validated, err := validateHTTPURL(*req.ExternalURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "公開先URLは有効なhttpまたはhttpsのURLを指定してください。"})
			return
		}
		req.ExternalURL = &validated
	}
	if req.ArtifactID != nil {
		var count int64
		if err := s.db.WithContext(c.Request.Context()).Model(&model.Artifact{}).Where("id = ?", *req.ArtifactID).Count(&count).Error; err != nil || count == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "指定された成果物が存在しません。"})
			return
		}
	}
	if req.FileObjectID != nil {
		var count int64
		if err := s.db.WithContext(c.Request.Context()).Model(&model.FileObject{}).Where("id = ?", *req.FileObjectID).Count(&count).Error; err != nil || count == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "指定されたファイルが存在しません。"})
			return
		}
	}
	request := model.PublicRequest{
		RequestType: req.RequestType, Status: "pending", ArtifactID: req.ArtifactID, FileObjectID: req.FileObjectID,
		Title: trimOptional(req.Title), CreatorName: trimOptional(req.CreatorName), SourceURL: trimOptional(req.SourceURL),
		SubmitterName: trimOptional(req.SubmitterName), Contact: trimOptional(req.Contact), Details: strings.TrimSpace(req.Details), ExternalURL: trimOptional(req.ExternalURL), SourcePasswordEncrypted: encryptedSourcePassword,
	}
	if req.RequestType == "preservation" && req.SourceURL != nil {
		var existingFile model.FileObject
		if err := s.db.WithContext(c.Request.Context()).Where("split_part(trim(source_url), '#', 1) = ? AND deleted_at IS NULL", *req.SourceURL).First(&existingFile).Error; err == nil {
			c.JSON(http.StatusOK, gin.H{"duplicate": true, "file_object_id": existingFile.ID, "message": "このURLはすでにカタログに登録されているため、重複申請を破棄しました。"})
			return
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "アップロード元URLの確認に失敗しました。"})
			return
		}
		var existingRequest model.PublicRequest
		if err := s.db.WithContext(c.Request.Context()).Where("request_type = 'preservation' AND split_part(trim(source_url), '#', 1) = ?", *req.SourceURL).Order("created_at ASC").First(&existingRequest).Error; err == nil {
			c.JSON(http.StatusOK, gin.H{"duplicate": true, "request_id": existingRequest.ID, "message": "同じURLの保全リクエストがすでに存在するため、重複リクエストを破棄しました。"})
			return
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "既存の保存申請の確認に失敗しました。"})
			return
		}
	}

	if err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		if request.RequestType == "preservation" && request.SourceURL != nil {
			result := tx.Exec(`INSERT INTO source_url_registry (normalized_url, source_url, origin, public_request_id, created_at, updated_at)
				VALUES (?, ?, 'public_request', ?, now(), now()) ON CONFLICT (normalized_url) DO NOTHING`, *request.SourceURL, *request.SourceURL, request.ID)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return errDuplicateSourceURL
			}
		}
		return nil
	}); err != nil {
		if errors.Is(err, errDuplicateSourceURL) {
			var existing struct {
				PublicRequestID *string `gorm:"column:public_request_id"`
			}
			_ = s.db.WithContext(c.Request.Context()).Table("source_url_registry").Select("public_request_id").Where("normalized_url = ?", *request.SourceURL).Scan(&existing).Error
			c.JSON(http.StatusOK, gin.H{"duplicate": true, "request_id": existing.PublicRequestID, "message": "同じURLの申請・収集リクエストがすでに存在するため、重複リクエストを破棄しました。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "申請を送信できませんでした。しばらく待ってから再度お試しください。"})
		return
	}
	s.audit(c, nil, "public_request", request.ID, "public_request.submit", "success", nil)
	// DuckDuckGo discoveries are inserted by the collector directly and do not pass
	// through this public submission handler, so only human-submitted requests reach here.
	if request.RequestType == "preservation" || request.RequestType == "deletion" || request.RequestType == "external_publication" || request.RequestType == "redistribution_assistance" || request.RequestType == "publisher_assistance" {
		typeLabel := map[string]string{"preservation": "保全リクエスト", "deletion": "削除リクエスト", "external_publication": "移設先情報", "redistribution_assistance": "手元のファイルの再配布相談", "publisher_assistance": "配布元からの配布代行相談"}[request.RequestType]
		subjectTitle := typeLabel
		if request.Title != nil && strings.TrimSpace(*request.Title) != "" {
			subjectTitle += ": " + strings.TrimSpace(*request.Title)
		}
		contactName := "申請者"
		if request.SubmitterName != nil {
			contactName = *request.SubmitterName
		}
		content := fmt.Sprintf("申請種別: %s\n申請ID: %s\n連絡先: %s\n対象URL: %s\n移設先URL: %s\n権利者への連絡先: %s\n添付ファイル: %s\n\n申請内容:\n%s", typeLabel, contactName, valueOrEmpty(request.Contact), valueOrEmpty(request.SourceURL), valueOrEmpty(request.ExternalURL), valueOrEmpty(request.RightsHolderContact), valueOrEmpty(request.IntakeFileName), request.Details)
		conversation_id := s.createLibreDeskConversation("public_request", request.ID, subjectTitle, contactName, valueOrEmpty(request.Contact), content)
		if conversation_id != "" {
			url := "https://uploaderjp.udonya.moe/admin/requests/" + request.ID
			way := "臨機応変に"
			switch req.RequestType {
			case "preservation":
				way = "1. 上記URLから内容を確認\n2. ファイルの保全処理を開始\n3. 実際のファイルを確認するなどして権利者と規約を確認\n4. 再配布不可の場合は権利者のSNSを確認\n5. 権利者のSNS上に告知がないようであれば権利者に連絡(新規会話をlibredeskで作成)\n6. 申請者に結果報告"
			case "deletion":
				way = "1. 非公開処理を進める\n2. お詫び文をctrl+m→削除申請対応から挿入後、必要事項を穴埋めして送信する"
			case "external_publication":
				way = "1. リダイレクトリンクを掲載する\n2. ありがとう文をctrl+m→移設申請対応から挿入後、必要事項を穴埋めして送信する"
			case "publisher_assistance":
				way = "1. ctrl+m→配布代行相談を送付する\n2. アサインを谷豊に設定する"
			}
			s.addLibreDeskPrivateNote(conversation_id, "申請URL: ["+url+"]("+url+")\n\n**対応手順**\n"+way+"\n\n**メール送信時の注意**\n1. 必ずctrl+m→挨拶を挿入すること\n2. 相手によってはテンプレートを改変して適切な文章にすること\n3. テンプレートの未入力部分がないかを必ず確認すること")
		}
	}
	c.JSON(http.StatusCreated, gin.H{"id": request.ID, "request_type": request.RequestType, "status": request.Status, "created_at": request.CreatedAt})
}

type publicRequestListItem struct {
	model.PublicRequest
	HasSourcePassword   bool   `json:"has_source_password"`
	LinkedFileName      string `json:"linked_file_name,omitempty"`
	LinkedArtifactTitle string `json:"linked_artifact_title,omitempty"`
}

func (s *Server) listPublicRequests(c *gin.Context) {
	page, limit := pagination(c)
	query := s.db.WithContext(c.Request.Context()).Model(&model.PublicRequest{})
	if status := c.Query("status"); status != "" {
		if !requestStatuses[status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not count requests"})
		return
	}
	var requests []model.PublicRequest
	if err := query.Order("CASE WHEN status = 'pending' THEN 0 WHEN status = 'in_review' THEN 1 ELSE 2 END, created_at ASC").Offset((page - 1) * limit).Limit(limit).Find(&requests).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list requests"})
		return
	}
	items := make([]publicRequestListItem, 0, len(requests))
	for _, request := range requests {
		item := publicRequestListItem{PublicRequest: request, HasSourcePassword: request.SourcePasswordEncrypted != nil && *request.SourcePasswordEncrypted != ""}
		artifactID := request.ArtifactID
		if request.FileObjectID != nil {
			var file model.FileObject
			if err := s.db.WithContext(c.Request.Context()).First(&file, "id = ?", *request.FileObjectID).Error; err == nil {
				item.LinkedFileName = file.OriginalFilename
				if artifactID == nil {
					artifactID = &file.ArtifactID
					item.ArtifactID = artifactID
				}
			}
		}
		if artifactID != nil {
			var artifact model.Artifact
			if err := s.db.WithContext(c.Request.Context()).First(&artifact, "id = ?", *artifactID).Error; err == nil {
				item.LinkedArtifactTitle = artifact.Title
			}
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "page": page, "limit": limit, "total": total})
}

func (s *Server) getPublicRequest(c *gin.Context) {
	var request model.PublicRequest

	db := s.db.WithContext(c.Request.Context())
	if err := db.First(&request, "id = ?", c.Param("id")).Error; err != nil {
		notFoundOrError(c, err, "public request")
		return
	}

	item := publicRequestListItem{
		PublicRequest: request,
		HasSourcePassword: request.SourcePasswordEncrypted != nil &&
			*request.SourcePasswordEncrypted != "",
	}

	artifactID := request.ArtifactID

	if request.FileObjectID != nil {
		var file model.FileObject
		if err := db.First(&file, "id = ?", *request.FileObjectID).Error; err == nil {
			item.LinkedFileName = file.OriginalFilename
			if artifactID == nil {
				artifactID = &file.ArtifactID
				item.ArtifactID = artifactID
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not load linked file",
			})
			return
		}
	}

	if artifactID != nil {
		var artifact model.Artifact
		if err := db.First(&artifact, "id = ?", *artifactID).Error; err == nil {
			item.LinkedArtifactTitle = artifact.Title
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not load linked artifact",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"item": item})
}

func (s *Server) patchPublicRequest(c *gin.Context) {
	var req struct {
		Status       *string `json:"status" binding:"omitempty"`
		Title        *string `json:"title" binding:"omitempty,max=500"`
		CreatorName  *string `json:"creator_name" binding:"omitempty,max=255"`
		SourceURL    *string `json:"source_url"`
		Contact      *string `json:"contact" binding:"omitempty,max=500"`
		Details      *string `json:"details" binding:"omitempty,min=1"`
		ExternalURL  *string `json:"external_url"`
		Reason       string  `json:"reason" binding:"required,min=3,max=2000"`
		ArtifactID   *string `json:"artifact_id" binding:"omitempty,uuid"`
		FileObjectID *string `json:"file_object_id" binding:"omitempty,uuid"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "data is invalid",
		})
		return
	}

	if req.Status != nil &&
		(!requestStatuses[*req.Status] || *req.Status == "pending") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "status is invalid",
		})
		return
	}

	actor, _ := currentUser(c)

	// These errors distinguish validation failures from database failures.
	var (
		errInvalidArtifact = errors.New("artifact_id does not exist")
		errInvalidFile     = errors.New(
			"file_object_id does not exist or does not belong to artifact_id",
		)
		errAcceptedTargetsImmutable = errors.New(
			"cannot change targets of an accepted request",
		)
		errDeletionTargetRequired = errors.New(
			"a deletion request must be linked to a file or artifact before acceptance",
		)
		errAdminRequired = errors.New(
			"administrator role is required to approve deletion requests",
		)
		errRequestConflict = errors.New(
			"public request was concurrently modified",
		)
		errSourceUrlConflict = errors.New(
			"source url conflict",
		)
		errUrlValidationFailed = errors.New(
			"provided url is invalid",
		)
	)

	db := s.db.WithContext(c.Request.Context())

	var publicRequest model.PublicRequest

	now := time.Now().UTC()
	var intakeFilePathToRemove string

	err := db.Transaction(func(tx *gorm.DB) error {
		// Lock the request row before reading its state.
		// Concurrent PATCH requests for the same request must wait
		// until this transaction commits or rolls back.
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&publicRequest, "id = ?", c.Param("id")).Error; err != nil {
			return err
		}

		// Remember the status observed under the row lock.
		originalStatus := publicRequest.Status

		// An accepted request cannot have its targets changed.
		if originalStatus == "accepted" &&
			(req.ArtifactID != nil || req.FileObjectID != nil) {
			return errAcceptedTargetsImmutable
		}

		// Resolve the effective targets from the locked database state
		// and the requested changes.
		targetArtifactID := publicRequest.ArtifactID
		if req.ArtifactID != nil {
			targetArtifactID = req.ArtifactID
		}

		targetFileID := publicRequest.FileObjectID
		if req.FileObjectID != nil {
			targetFileID = req.FileObjectID
		}

		// Validate a newly supplied artifact ID.
		if req.ArtifactID != nil {
			var count int64

			if err := tx.Model(&model.Artifact{}).
				Where("id = ?", *req.ArtifactID).
				Count(&count).Error; err != nil {
				return err
			}

			if count == 0 {
				return errInvalidArtifact
			}
		}

		// Validate the effective file target whenever either target
		// is being changed. This prevents retaining a file that belongs
		// to a different artifact after changing artifact_id.
		if (req.FileObjectID != nil || req.ArtifactID != nil) &&
			targetFileID != nil {
			query := tx.Model(&model.FileObject{}).
				Where("id = ?", *targetFileID)

			if targetArtifactID != nil {
				query = query.Where(
					"artifact_id = ?", *targetArtifactID,
				)
			}

			var count int64

			if err := query.Count(&count).Error; err != nil {
				return err
			}

			if count == 0 {
				return errInvalidFile
			}
		}

		// An accepted deletion request must have a target.
		if req.Status != nil &&
			*req.Status == "accepted" &&
			publicRequest.RequestType == "deletion" &&
			targetFileID == nil &&
			targetArtifactID == nil {
			return errDeletionTargetRequired
		}

		// Only administrators may approve deletion requests.
		if req.Status != nil &&
			*req.Status == "accepted" &&
			publicRequest.RequestType == "deletion" &&
			actor.Role != "admin" {
			return errAdminRequired
		}

		updates := map[string]any{
			"updated_at": now,
		}

		sourceURLChanged := false

		if req.Status != nil {
			updates["status"] = *req.Status
			updates["reviewed_by"] = actor.ID
			updates["reviewed_at"] = now
		}

		if req.Title != nil {
			updates["title"] = *req.Title
		}

		if req.CreatorName != nil {
			updates["creator_name"] = *req.CreatorName
		}

		if req.SourceURL != nil {
			if publicRequest.RequestType == "preservation" {
				url, err := normalizeUploaderURL(*req.SourceURL)
				if err != nil {
					return errUrlValidationFailed
				}
				req.SourceURL = &url

				// Compare normalized URLs, not the raw request value.
				sourceURLChanged = publicRequest.SourceURL == nil ||
					*req.SourceURL != *publicRequest.SourceURL

				// Check for duplicates only when the URL actually changes.
				// Otherwise, the existing request could conflict with itself.
				if sourceURLChanged {
					var existingFile model.FileObject
					if err := tx.
						Where(
							"split_part(trim(source_url), '#', 1) = ? AND deleted_at IS NULL",
							*req.SourceURL,
						).
						First(&existingFile).Error; err == nil {
						return errSourceUrlConflict
					} else if !errors.Is(err, gorm.ErrRecordNotFound) {
						return fmt.Errorf(
							"could not check existing source URL: %w",
							err,
						)
					}

					var existingRequest model.PublicRequest
					if err := tx.
						Where(
							"request_type = 'preservation' AND id <> ? AND split_part(trim(source_url), '#', 1) = ?",
							publicRequest.ID,
							*req.SourceURL,
						).
						Order("created_at ASC").
						First(&existingRequest).Error; err == nil {
						return errSourceUrlConflict
					} else if !errors.Is(err, gorm.ErrRecordNotFound) {
						return fmt.Errorf(
							"could not check existing preservation requests: %w",
							err,
						)
					}
				}
			} else {
				normalized, err := validateHTTPURL(*req.SourceURL)
				if err != nil {
					return errUrlValidationFailed
				}
				req.SourceURL = &normalized
			}

			updates["source_url"] = *req.SourceURL
		}

		if req.Contact != nil {
			updates["contact"] = *req.Contact
		}

		if req.Details != nil {
			updates["details"] = *req.Details
		}

		if req.ExternalURL != nil {
			validated, err := validateHTTPURL(*req.ExternalURL)
			if err != nil {
				return errUrlValidationFailed
			}
			updates["external_url"] = validated
		}

		if req.ArtifactID != nil {
			updates["artifact_id"] = *req.ArtifactID
		}

		if req.FileObjectID != nil {
			updates["file_object_id"] = *req.FileObjectID
		}

		// Clear sensitive data when a request is rejected or closed.
		if req.Status != nil &&
			(*req.Status == "rejected" || *req.Status == "closed") {
			updates["source_password_encrypted"] = nil

			if publicRequest.RequestType == "redistribution_assistance" ||
				publicRequest.RequestType == "publisher_assistance" {
				updates["intake_file_path"] = ""
			}

			intakeFilePathToRemove = publicRequest.IntakeFilePath
		}

		// Include the observed status in the UPDATE predicate.
		// The row lock protects the read-modify-write sequence, while
		// this predicate provides an additional optimistic concurrency
		// check.
		result := tx.Model(&model.PublicRequest{}).
			Where(
				"id = ? AND status = ?",
				publicRequest.ID,
				originalStatus,
			).
			Updates(updates)

		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			return errRequestConflict
		}

		if publicRequest.RequestType == "preservation" &&
			sourceURLChanged {
			// Register the new normalized URL. A conflict with the same
			// request is allowed; a conflict with another request is not.
			result := tx.Exec(`
		INSERT INTO source_url_registry (
			normalized_url,
			source_url,
			origin,
			public_request_id,
			created_at,
			updated_at
		)
		VALUES (?, ?, 'public_request', ?, now(), now())
		ON CONFLICT (normalized_url) DO UPDATE
		SET
			source_url = EXCLUDED.source_url,
			updated_at = now()
		WHERE source_url_registry.public_request_id =
			EXCLUDED.public_request_id
	`, *req.SourceURL, *req.SourceURL, publicRequest.ID)

			if result.Error != nil {
				return result.Error
			}

			// Zero affected rows means the conflicting registry entry
			// belongs to a different request.
			if result.RowsAffected == 0 {
				return errDuplicateSourceURL
			}

			// Remove obsolete registry entries owned by this request.
			// Keep the newly registered URL.
			result = tx.Exec(`
		DELETE FROM source_url_registry
		WHERE origin = 'public_request'
			AND public_request_id = ?
			AND normalized_url <> ?
	`, publicRequest.ID, *req.SourceURL)

			if result.Error != nil {
				return result.Error
			}
		}

		// Takedown requests immediately disable public downloads.
		// Accepted deletion requests enqueue physical deletion tasks.
		// These operations remain in the same transaction as the
		// request update.
		if req.Status != nil &&
			*req.Status == "accepted" &&
			(publicRequest.RequestType == "takedown" ||
				publicRequest.RequestType == "deletion") {

			if targetFileID != nil {
				if err := tx.Model(&model.FileObject{}).
					Where("id = ?", *targetFileID).
					Update("public_download_enabled", false).Error; err != nil {
					return err
				}
			} else if publicRequest.RequestType == "takedown" &&
				targetArtifactID != nil {
				if err := tx.Model(&model.FileObject{}).
					Where(
						"artifact_id = ? AND deleted_at IS NULL",
						*targetArtifactID,
					).
					Update("public_download_enabled", false).Error; err != nil {
					return err
				}
			}

			if publicRequest.RequestType == "deletion" {
				nextRequest := publicRequest
				nextRequest.Status = *req.Status
				nextRequest.ArtifactID = targetArtifactID
				nextRequest.FileObjectID = targetFileID

				var files []model.FileObject

				fileQuery := tx.Where("deleted_at IS NULL")

				if targetFileID != nil {
					fileQuery = fileQuery.Where(
						"id = ?", *targetFileID,
					)
				} else if targetArtifactID != nil {
					fileQuery = fileQuery.Where(
						"artifact_id = ?", *targetArtifactID,
					)
				} else {
					return errDeletionTargetRequired
				}

				if err := fileQuery.Find(&files).Error; err != nil {
					return err
				}

				for _, file := range files {
					if err := queueFileDeletionTx(
						tx,
						file,
						nextRequest,
						actor.ID,
						req.Reason,
					); err != nil {
						return err
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		// Preserve the existing not-found handling.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			notFoundOrError(c, err, "public request")
			return
		}

		switch {
		case errors.Is(err, errInvalidArtifact):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return

		case errors.Is(err, errInvalidFile):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return

		case errors.Is(err, errAcceptedTargetsImmutable):
			c.JSON(http.StatusConflict, gin.H{
				"error": err.Error(),
			})
			return

		case errors.Is(err, errDeletionTargetRequired):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return

		case errors.Is(err, errAdminRequired):
			c.JSON(http.StatusForbidden, gin.H{
				"error": err.Error(),
			})
			return

		case errors.Is(err, errRequestConflict):
			c.JSON(http.StatusConflict, gin.H{
				"error": err.Error(),
			})
			return
		case errors.Is(err, errUrlValidationFailed):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		case errors.Is(err, errSourceUrlConflict):
			c.JSON(http.StatusConflict, gin.H{
				"error": err.Error(),
			})
			return
		}

		// Preserve the existing conflict handling for deletion
		// operations that cannot proceed.
		statusCode := http.StatusInternalServerError

		if strings.Contains(err.Error(), "running preservation task") ||
			strings.Contains(err.Error(), "already being deleted") {
			statusCode = http.StatusConflict
		}

		c.JSON(statusCode, gin.H{
			"error": err.Error(),
		})
		return
	}

	// The transaction has committed successfully. Reflect the changes
	// in the in-memory model used for the response and audit log.
	if req.Status != nil {
		publicRequest.Status = *req.Status
		publicRequest.ReviewedBy = &actor.ID
		publicRequest.ReviewedAt = &now
	}

	if req.Title != nil {
		publicRequest.Title = req.Title
	}

	if req.CreatorName != nil {
		publicRequest.CreatorName = req.CreatorName
	}

	if req.SourceURL != nil {
		publicRequest.SourceURL = req.SourceURL
	}

	if req.Contact != nil {
		publicRequest.Contact = req.Contact
	}

	if req.Details != nil {
		publicRequest.Details = *req.Details
	}

	if req.ExternalURL != nil {
		publicRequest.ExternalURL = req.ExternalURL
	}

	if req.ArtifactID != nil {
		publicRequest.ArtifactID = req.ArtifactID
	}

	if req.FileObjectID != nil {
		publicRequest.FileObjectID = req.FileObjectID
	}

	if req.Status != nil &&
		(*req.Status == "rejected" || *req.Status == "closed") {
		publicRequest.IntakeFilePath = ""
	}

	publicRequest.UpdatedAt = now

	// Remove the intake file only after the database transaction
	// has committed successfully.
	if intakeFilePathToRemove != "" {
		path := filepath.Clean(intakeFilePathToRemove)

		if !filepath.IsAbs(path) &&
			strings.HasPrefix(
				path,
				"public-intake"+string(os.PathSeparator),
			) &&
			!strings.Contains(path, "..") {

			if removeErr := os.Remove(
				filepath.Join(s.cfg.StagingDir, path),
			); removeErr != nil && !os.IsNotExist(removeErr) {
				s.logger.Warn(
					"could not remove closed public intake file",
					"request_id", publicRequest.ID,
					"error", removeErr,
				)
			}
		}
	}

	s.audit(
		c,
		&actor,
		"public_request",
		publicRequest.ID,
		"public_request.review",
		"success",
		&req.Reason,
	)

	if req.Status != nil &&
		*req.Status == "accepted" &&
		publicRequest.RequestType == "deletion" {
		s.audit(
			c,
			&actor,
			"public_request",
			publicRequest.ID,
			"public_request.deletion.queued",
			"success",
			&req.Reason,
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"request": publicRequest,
		"note": "Accepted takedown requests disable public downloads. " +
			"Accepted deletion requests enqueue node-agent deletion tasks; " +
			"the catalog entry is hidden after all placements report deletion success.",
	})
}

func (s *Server) listAuditLogs(c *gin.Context) {
	page, limit := pagination(c)
	var total int64
	if err := s.db.WithContext(c.Request.Context()).Model(&model.AuditLog{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not count audit logs"})
		return
	}
	type auditLogView struct {
		model.AuditLog   `gorm:"embedded"`
		ActorDisplayName *string `json:"actor_display_name,omitempty"`
	}
	var logs []auditLogView
	if err := s.db.WithContext(c.Request.Context()).
		Model(&model.AuditLog{}).
		Select("audit_logs.*, users.display_name AS actor_display_name").
		Joins("LEFT JOIN users ON users.id = audit_logs.actor_id").
		Order("audit_logs.created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Scan(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list audit logs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": logs, "page": page, "limit": limit, "total": total})
}
