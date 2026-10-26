package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"gorm.io/gorm"
)

const maxPublicIntakeFileBytes int64 = 100 << 20

func (s *Server) rateLimitPublicIntake() gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now()
		ip := c.ClientIP()
		s.enrollmentLimiterMu.Lock()
		attempt, ok := s.publicIntakeAttempts[ip]
		if !ok || now.Sub(attempt.windowStart) >= time.Hour {
			attempt = enrollmentAttempt{windowStart: now}
		}
		if attempt.count >= 5 {
			s.enrollmentLimiterMu.Unlock()
			c.Header("Retry-After", "3600")
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "申請回数の上限に達しました。時間をおいて再度お試しください。"})
			c.Abort()
			return
		}
		attempt.count++
		s.publicIntakeAttempts[ip] = attempt
		if len(s.publicIntakeAttempts) > 2000 {
			for key, value := range s.publicIntakeAttempts {
				if now.Sub(value.windowStart) >= time.Hour {
					delete(s.publicIntakeAttempts, key)
				}
			}
		}
		s.enrollmentLimiterMu.Unlock()
		c.Next()
	}
}

func sanitizeIntakeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	var b strings.Builder
	for _, r := range name {
		if unicode.IsControl(r) || r == '/' || r == '\\' || r == '"' || r == ';' {
			continue
		}
		b.WriteRune(r)
	}
	name = strings.TrimSpace(b.String())
	if name == "" || name == "." {
		return "uploaded-file"
	}
	runes := []rune(name)
	if len(runes) > 500 {
		name = string(runes[:500])
	}
	return name
}

func (s *Server) submitRedistributionAssistance(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPublicIntakeFileBytes+(2<<20))
	if err := c.Request.ParseMultipartForm(8 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "フォームを読み込めませんでした。ファイルは100 MiB以下にしてください。"})
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}

	requestType := strings.TrimSpace(c.PostForm("request_type"))
	if requestType != "redistribution_assistance" && requestType != "publisher_assistance" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "申請種別が正しくありません。"})
		return
	}
	if !s.verifyTurnstile(c, c.PostForm("turnstile_token")) {
		return
	}
	title := strings.TrimSpace(c.PostForm("title"))
	creator := strings.TrimSpace(c.PostForm("creator_name"))
	sourceURL := strings.TrimSpace(c.PostForm("source_url"))
	rightsContact := strings.TrimSpace(c.PostForm("rights_holder_contact"))
	name := strings.TrimSpace(c.PostForm("submitter_name"))
	contact := strings.TrimSpace(c.PostForm("contact"))
	details := strings.TrimSpace(c.PostForm("details"))
	if title == "" || len([]rune(title)) > 500 || contact == "" || len([]rune(contact)) > 500 || len([]rune(details)) < 5 || len([]rune(details)) > 10000 || len([]rune(creator)) > 255 || len([]rune(rightsContact)) > 1000 || len([]rune(name)) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "アップローダー・ファイル名、連絡先、5文字以上の説明を入力してください。"})
		return
	}
	if c.PostForm("agree_to_terms") != "on" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ファイルの取扱いと権利確認に関する同意が必要です。"})
		return
	}
	var sourceURLPtr *string
	if sourceURL != "" {
		if len(sourceURL) > 4096 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "対象URLは4096文字以内で入力してください。"})
			return
		}
		validated, err := validateHTTPURL(sourceURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "対象URLには有効な http(s) URL を入力してください。"})
			return
		}
		sourceURL = validated
		sourceURLPtr = &sourceURL
	}

	var relativePath string
	var fileNamePtr, shaPtr *string
	var sizePtr *int64
	var uploadedAt *time.Time
	fileHeader, fileErr := c.FormFile("file")
	hasUploadedFile := fileErr == nil && strings.TrimSpace(fileHeader.Filename) != ""
	if hasUploadedFile {
		if fileHeader.Size > maxPublicIntakeFileBytes {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "ファイルは100 MiB以下にしてください。"})
			return
		}
		source, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "アップロードされたファイルを開けませんでした。"})
			return
		}
		defer source.Close()
		dir := filepath.Join(s.cfg.StagingDir, "public-intake")
		if err := os.MkdirAll(dir, 0700); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "アップロード先を準備できませんでした。"})
			return
		}
		fileID := uuid.NewString()
		relativePath = filepath.Join("public-intake", fileID+".upload")
		absolutePath := filepath.Join(s.cfg.StagingDir, relativePath)
		destination, err := os.OpenFile(absolutePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ファイルを一時保管できませんでした。"})
			return
		}
		hasher := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(destination, hasher), io.LimitReader(source, maxPublicIntakeFileBytes+1))
		closeErr := destination.Close()
		if copyErr != nil || closeErr != nil || n <= 0 || n > maxPublicIntakeFileBytes {
			_ = os.Remove(absolutePath)
			if n > maxPublicIntakeFileBytes {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "ファイルは100 MiB以下にしてください。"})
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "ファイルを最後まで受け取れませんでした。"})
			}
			return
		}
		filename := sanitizeIntakeFilename(fileHeader.Filename)
		fileNamePtr = &filename
		sum := hex.EncodeToString(hasher.Sum(nil))
		shaPtr = &sum
		size := n
		sizePtr = &size
		now := time.Now().UTC()
		uploadedAt = &now
	} else if requestType == "redistribution_assistance" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "手元のファイルの再配布相談には、対象ファイルの添付が必要です。"})
		return
	}
	if requestType == "publisher_assistance" && relativePath == "" && sourceURLPtr == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ファイルを添付するか、配布元URLを入力してください。"})
		return
	}

	request := model.PublicRequest{
		RequestType: requestType, Status: "pending", Title: stringPtr(title), CreatorName: trimOptional(&creator), SourceURL: sourceURLPtr,
		SubmitterName: trimOptional(&name), Contact: trimOptional(&contact), Details: details,
		RightsHolderContact: trimOptional(&rightsContact), IntakeFilePath: relativePath, IntakeFileName: fileNamePtr,
		IntakeFileSize: sizePtr, IntakeFileSHA256: shaPtr, IntakeFileUploadedAt: uploadedAt,
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&request).Error; err != nil {
		if relativePath != "" {
			_ = os.Remove(filepath.Join(s.cfg.StagingDir, relativePath))
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "申請を保存できませんでした。時間をおいて再度お試しください。"})
		return
	}
	s.audit(c, nil, "public_request", request.ID, "public_request.submit", "success", nil)
	label := "手元のファイルの再配布相談"
	if requestType == "publisher_assistance" {
		label = "配布元からの配布代行相談"
	}
	content := fmt.Sprintf("申請種別: %s\n申請者: %s\n連絡先: %s\nアップローダー・ファイル名: %s\n作者・配布元: %s\n対象URL: %s\n権利者への連絡先: %s\n添付ファイル: %s\nファイルサイズ: %d bytes\nSHA-256: %s\n\n申請内容:\n%s", label, valueOrEmpty(request.SubmitterName), valueOrEmpty(request.Contact), title, creator, valueOrEmpty(sourceURLPtr), rightsContact, valueOrEmpty(fileNamePtr), valueOrZero(sizePtr), valueOrEmpty(shaPtr), details)
	conversation_id := s.createLibreDeskConversation("public_request", request.ID, "【uploader.jp保全】"+label+": "+title, valueOrEmpty(request.SubmitterName), valueOrEmpty(request.Contact), content)
	if conversation_id != "" {
		url := "https://uploaderjp.udonya.moe/admin/requests/" + request.ID
		way := "臨機応変に"
		switch requestType {
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
	c.JSON(http.StatusCreated, gin.H{"id": request.ID, "request_type": request.RequestType, "status": request.Status, "message": "申請を受け付けました。権利関係を確認し、必要に応じて連絡します。許諾が確認できるまでは公開・再配布しません。"})
}

func valueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func (s *Server) downloadPublicRequestIntakeFile(c *gin.Context) {
	var request model.PublicRequest
	if err := s.db.WithContext(c.Request.Context()).First(&request, "id = ?", c.Param("id")).Error; err != nil {
		notFoundOrError(c, err, "public request")
		return
	}
	if request.IntakeFilePath == "" || request.IntakeFileName == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "添付ファイルがありません。"})
		return
	}
	relative := filepath.Clean(request.IntakeFilePath)
	if filepath.IsAbs(relative) || !strings.HasPrefix(relative, "public-intake"+string(os.PathSeparator)) || strings.Contains(relative, "..") {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添付ファイルの保存先が正しくありません。"})
		return
	}
	path := filepath.Join(s.cfg.StagingDir, relative)
	if _, err := os.Stat(path); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "添付ファイルが見つかりません。"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.FileAttachment(path, sanitizeIntakeFilename(*request.IntakeFileName))
}

// ingestPublicRequestUpload transfers an approved intake upload into the private
// preservation network. It deliberately does not enable public downloads or
// grant redistribution permission.
func (s *Server) ingestPublicRequestUpload(c *gin.Context) {
	var request model.PublicRequest
	if err := s.db.WithContext(c.Request.Context()).First(&request, "id = ?", c.Param("id")).Error; err != nil {
		notFoundOrError(c, err, "public request")
		return
	}
	if request.RequestType != "redistribution_assistance" && request.RequestType != "publisher_assistance" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "この申請種別は添付ファイルの保全に対応していません。"})
		return
	}
	if request.Status != "accepted" {
		c.JSON(http.StatusConflict, gin.H{"error": "権利者の許諾と配布条件を確認し、申請を承認した後に実行してください。"})
		return
	}
	if request.IntakeFilePath == "" || request.IntakeFileName == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "保全対象の添付ファイルがありません。"})
		return
	}
	if request.ArtifactID != nil || request.FileObjectID != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "この申請には既にカタログの作品またはファイルが紐付いています。"})
		return
	}
	relative := filepath.Clean(request.IntakeFilePath)
	if filepath.IsAbs(relative) || !strings.HasPrefix(relative, "public-intake"+string(os.PathSeparator)) || strings.Contains(relative, "..") {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添付ファイルの保存先が正しくありません。"})
		return
	}
	if err := os.MkdirAll(s.cfg.StagingDir, 0750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "一時保管領域を準備できませんでした。"})
		return
	}
	source, err := os.Open(filepath.Join(s.cfg.StagingDir, relative))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "添付ファイルが見つかりません。"})
		return
	}
	defer source.Close()
	tmp, err := os.CreateTemp(s.cfg.StagingDir, ".request-ingest-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保全用の一時ファイルを作成できませんでした。"})
		return
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(source, maxPublicIntakeFileBytes+1))
	if copyErr != nil || written <= 0 || written > maxPublicIntakeFileBytes {
		_ = tmp.Close()
		cleanup()
		c.JSON(http.StatusBadRequest, gin.H{"error": "添付ファイルを読み込めませんでした。"})
		return
	}
	if request.IntakeFileSize != nil && written != *request.IntakeFileSize {
		_ = tmp.Close()
		cleanup()
		c.JSON(http.StatusConflict, gin.H{"error": "添付ファイルのサイズが申請時の記録と一致しません。"})
		return
	}
	sha := hex.EncodeToString(hasher.Sum(nil))
	if request.IntakeFileSHA256 != nil && !strings.EqualFold(sha, *request.IntakeFileSHA256) {
		_ = tmp.Close()
		cleanup()
		c.JSON(http.StatusConflict, gin.H{"error": "添付ファイルのハッシュが申請時の記録と一致しません。"})
		return
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保全用ファイルを同期できませんでした。"})
		return
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保全用ファイルを閉じられませんでした。"})
		return
	}

	artifactSourceURL := "https://uploader.jp/"
	var fileSourceURL *string
	if request.SourceURL != nil {
		if validated, validationErr := validateUploaderURL(*request.SourceURL); validationErr == nil {
			artifactSourceURL = validated
			fileSourceURL = &validated
		}
	}
	description := "申請フォームから受け付けたファイルです。権利許諾確認後に公開可否を別途判断します。"
	if fileSourceURL == nil {
		description += "\nカタログの参照先欄には uploader.jp トップページを仮設定しており、正確な元URLではありません。"
		if request.SourceURL != nil {
			description += "\n申請時に提供された元URL: " + *request.SourceURL
		} else {
			description += "\n正確な元URLは未提供です。"
		}
	}
	artifact := model.Artifact{Title: strings.TrimSpace(valueOrEmpty(request.Title)), CreatorName: trimOptional(request.CreatorName), SourceURL: artifactSourceURL, Description: &description}
	file := model.FileObject{OriginalFilename: *request.IntakeFileName, SizeBytes: written, SHA256: &sha, MediaType: stringPtr("application/octet-stream"), PreservationStatus: "uncollected", RedistributionPolicy: "unknown", PublicDownloadEnabled: false, SourceURL: fileSourceURL}
	db := s.db.WithContext(c.Request.Context())
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&artifact).Error; err != nil {
			return err
		}
		file.ArtifactID = artifact.ID
		if err := tx.Create(&file).Error; err != nil {
			return err
		}
		result := tx.Model(&model.PublicRequest{}).Where("id = ? AND status = 'accepted' AND artifact_id IS NULL AND file_object_id IS NULL", request.ID).Updates(map[string]any{"artifact_id": artifact.ID, "file_object_id": file.ID, "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("申請の状態が変わったため、もう一度確認してください")
		}
		return nil
	})
	if err != nil {
		cleanup()
		c.JSON(http.StatusConflict, gin.H{"error": "カタログへの登録に失敗しました。申請を再読み込みして確認してください。"})
		return
	}

	if !s.queueStagedIngest(c, file, tmpName, written, sha, "application/octet-stream", *request.IntakeFileName, 3, fileSourceURL, "", "", "") {
		_ = db.Model(&model.PublicRequest{}).Where("id = ?", request.ID).Updates(map[string]any{"artifact_id": nil, "file_object_id": nil, "updated_at": time.Now().UTC()}).Error
		if err := db.Delete(&model.Artifact{}, "id = ?", artifact.ID).Error; err != nil {
			s.logger.Warn("could not roll back failed intake ingestion", "request_id", request.ID, "artifact_id", artifact.ID, "error", err)
		}
		return
	}
	actor, _ := currentUser(c)
	s.audit(c, &actor, "public_request", request.ID, "public_request.upload_ingest.queued", "success", stringPtr("Permission checked by operator before ingest"))
}
