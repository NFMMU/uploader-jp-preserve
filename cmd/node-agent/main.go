package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const agentVersion = "0.5.1"

type config struct {
	APIURL              string
	Enrollment          string
	Name                string
	NodeType            string
	Requested           int64
	StoragePath         string
	StatePath           string
	UploaderMinInterval time.Duration
}

type credentials struct {
	NodeID string `json:"node_id"`
	Token  string `json:"token"`
}

type task struct {
	TaskID           string  `json:"task_id"`
	TaskType         string  `json:"task_type"`
	SourceNodeID     *string `json:"source_node_id"`
	TargetNodeID     *string `json:"target_node_id"`
	LeaseToken       string  `json:"lease_token"`
	FileObjectID     string  `json:"file_object_id"`
	OriginalFilename string  `json:"original_filename"`
	SizeBytes        int64   `json:"size_bytes"`
	SHA256           *string `json:"sha256"`
	StorageKey       string  `json:"storage_key"`
	AttemptCount     int32   `json:"attempt_count"`
	SourceURL        string  `json:"source_url"`
	SourcePassword   string  `json:"source_password"`
}

type agent struct {
	cfg               config
	creds             credentials
	client            *http.Client
	log               *slog.Logger
	lastUploaderFetch time.Time
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := loadConfig()
	if err != nil {
		logger.Error("configuration_error", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.StoragePath, 0o750); err != nil {
		logger.Error("storage_path_error", "error", err)
		os.Exit(1)
	}
	creds, err := loadOrRegister(cfg, logger)
	if err != nil {
		logger.Error("node_enrollment_error", "error", err)
		os.Exit(1)
	}
	a := &agent{cfg: cfg, creds: creds, client: &http.Client{}, log: logger}
	a.loadUploaderFetchTimestamp()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := a.heartbeat(ctx); err != nil {
		logger.Warn("initial_heartbeat_failed", "error", err)
	}
	go a.heartbeatLoop(ctx)
	logger.Info("node_agent_started", "node_id", creds.NodeID, "node_name", cfg.Name, "storage_path", cfg.StoragePath, "uploader_fetcher_version", uploaderFetcherVersion)
	poll := time.NewTicker(5 * time.Second)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("node_agent_stopping")
			return
		case <-poll.C:
			if err := a.claimAndProcess(ctx); err != nil {
				logger.Warn("task_poll_failed", "error", err)
			}
			if err := a.claimAndProcessDelivery(ctx); err != nil {
				logger.Warn("delivery_poll_failed", "error", err)
			}
		}
	}
}

func loadConfig() (config, error) {
	cfg := config{
		APIURL:              strings.TrimRight(strings.TrimSpace(os.Getenv("CENTRAL_API_URL")), "/"),
		Enrollment:          strings.TrimSpace(os.Getenv("NODE_ENROLLMENT_TOKEN")),
		Name:                strings.TrimSpace(os.Getenv("NODE_NAME")),
		NodeType:            strings.TrimSpace(os.Getenv("NODE_TYPE")),
		StoragePath:         strings.TrimSpace(os.Getenv("NODE_STORAGE_PATH")),
		StatePath:           strings.TrimSpace(os.Getenv("NODE_STATE_PATH")),
		UploaderMinInterval: 30 * time.Second,
	}
	if cfg.APIURL == "" || cfg.Name == "" {
		return config{}, errors.New("CENTRAL_API_URL and NODE_NAME are required")
	}
	parsed, err := url.Parse(cfg.APIURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return config{}, errors.New("CENTRAL_API_URL must be an absolute http(s) URL")
	}
	if cfg.NodeType == "" {
		cfg.NodeType = "A"
	}
	if cfg.NodeType != "A" && cfg.NodeType != "B" {
		return config{}, errors.New("NODE_TYPE must be A or B")
	}
	if cfg.StoragePath == "" {
		cfg.StoragePath = "/data/objects"
	}
	if cfg.StatePath == "" {
		cfg.StatePath = "/data/node-credentials.json"
	}
	if intervalText := strings.TrimSpace(os.Getenv("NODE_UPLOADER_MIN_INTERVAL")); intervalText != "" {
		interval, parseErr := time.ParseDuration(intervalText)
		if parseErr != nil || interval < 0 {
			return config{}, errors.New("NODE_UPLOADER_MIN_INTERVAL must be a non-negative duration such as 30s or 1m")
		}
		cfg.UploaderMinInterval = interval
	}
	quotaText := strings.TrimSpace(os.Getenv("NODE_CAPACITY_BYTES"))
	if quotaText != "" {
		quota, err := strconv.ParseInt(quotaText, 10, 64)
		if err != nil || quota < 0 {
			return config{}, errors.New("NODE_CAPACITY_BYTES must be a non-negative integer; 0 means no participant-defined quota")
		}
		cfg.Requested = quota
	}
	return cfg, nil
}

func loadOrRegister(cfg config, logger *slog.Logger) (credentials, error) {
	if b, err := os.ReadFile(cfg.StatePath); err == nil {
		var creds credentials
		if json.Unmarshal(b, &creds) != nil || creds.NodeID == "" || creds.Token == "" {
			return credentials{}, errors.New("stored node credential file is invalid; do not delete it unless you intend to re-enroll this node")
		}
		return creds, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return credentials{}, err
	}
	if cfg.Enrollment == "" {
		return credentials{}, errors.New("NODE_ENROLLMENT_TOKEN is required for initial enrollment; request a fresh token if this node's credential file was lost")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.StatePath), 0o700); err != nil {
		return credentials{}, err
	}
	body, _ := json.Marshal(map[string]any{
		"name": cfg.Name, "node_type": cfg.NodeType,
		"requested_capacity_bytes": cfg.Requested, "agent_version": agentVersion,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.APIURL+"/api/v1/nodes/register", bytes.NewReader(body))
	if err != nil {
		return credentials{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Enrollment-Token", cfg.Enrollment)
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return credentials{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return credentials{}, fmt.Errorf("node registration returned %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	var response struct {
		Node struct {
			ID string `json:"id"`
		} `json:"node"`
		Token string `json:"node_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return credentials{}, err
	}
	if response.Node.ID == "" || response.Token == "" {
		return credentials{}, errors.New("registration response did not include node credentials")
	}
	creds := credentials{NodeID: response.Node.ID, Token: response.Token}
	encoded, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return credentials{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(cfg.StatePath), ".node-credentials-*")
	if err != nil {
		return credentials{}, err
	}
	tmpPath := tmp.Name()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return credentials{}, err
	}
	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return credentials{}, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return credentials{}, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return credentials{}, err
	}
	if err := os.Rename(tmpPath, cfg.StatePath); err != nil {
		_ = os.Remove(tmpPath)
		return credentials{}, err
	}
	logger.Info("node_registered", "node_id", creds.NodeID, "credentials_path", cfg.StatePath)
	return creds, nil
}

func (a *agent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.heartbeat(ctx); err != nil {
				a.log.Warn("heartbeat_failed", "error", err)
			}
		}
	}
}

func (a *agent) heartbeat(ctx context.Context) error {
	var fs unix.Statfs_t
	if err := unix.Statfs(a.cfg.StoragePath, &fs); err != nil {
		return err
	}
	capacity := int64(fs.Blocks) * int64(fs.Bsize)
	available := int64(fs.Bavail) * int64(fs.Bsize)
	var used int64
	err := filepath.WalkDir(a.cfg.StoragePath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			used += info.Size()
		}
		return nil
	})
	if err != nil {
		return err
	}
	payload := map[string]any{
		"capacity_bytes": capacity, "used_bytes": used, "available_bytes": available,
		"requested_capacity_bytes": a.cfg.Requested, "agent_version": agentVersion,
	}
	return a.postJSON(ctx, "/api/v1/nodes/"+a.creds.NodeID+"/heartbeat", payload, nil)
}

func (a *agent) claimAndProcess(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.APIURL+"/api/v1/nodes/"+a.creds.NodeID+"/tasks/claim", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.creds.Token)
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("claim task returned %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	var t task
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return err
	}
	if t.TaskID == "" || t.LeaseToken == "" || t.FileObjectID == "" {
		return errors.New("central API returned an incomplete task")
	}
	a.log.Info("task_claimed", "task_id", t.TaskID, "task_type", t.TaskType, "file_object_id", t.FileObjectID, "attempt", t.AttemptCount)
	if t.TaskType == "source_fetch" {
		if t.SourceURL == "" {
			return a.reportResult(ctx, t, map[string]any{"lease_token": t.LeaseToken, "status": "failure", "error": "source-fetch task has no source URL"})
		}
		if err := a.fetchAndUploadSource(ctx, t); err != nil {
			a.log.Warn("source_fetch_failed", "task_id", t.TaskID, "uploader_fetcher_version", uploaderFetcherVersion, "error", err)
			return a.reportResult(ctx, t, map[string]any{"lease_token": t.LeaseToken, "status": "failure", "error": truncate(err.Error(), 1900)})
		}
		a.log.Info("source_fetch_uploaded", "task_id", t.TaskID, "file_object_id", t.FileObjectID)
		return nil
	}
	if t.StorageKey == "" || t.SHA256 == nil {
		return errors.New("central API returned an incomplete storage task")
	}
	if t.TaskType == "replicate" {
		if err := a.uploadReplica(ctx, t); err != nil {
			a.log.Warn("replica_upload_failed", "task_id", t.TaskID, "error", err)
			return a.reportResult(ctx, t, map[string]any{"lease_token": t.LeaseToken, "status": "failure", "error": truncate(err.Error(), 1900)})
		}
		a.log.Info("replica_source_uploaded", "task_id", t.TaskID, "target_node_id", t.TargetNodeID)
		return nil
	}
	if t.TaskType == "delete" {
		if err := a.deleteStoredObject(t.StorageKey); err != nil {
			a.log.Warn("object_delete_failed", "task_id", t.TaskID, "error", err)
			return a.reportResult(ctx, t, map[string]any{"lease_token": t.LeaseToken, "status": "failure", "error": truncate(err.Error(), 1900)})
		}
		if err := a.reportResult(ctx, t, map[string]any{"lease_token": t.LeaseToken, "status": "success"}); err != nil {
			return err
		}
		a.log.Info("object_deleted", "task_id", t.TaskID, "file_object_id", t.FileObjectID, "storage_key", t.StorageKey)
		if err := a.heartbeat(ctx); err != nil {
			a.log.Warn("post_task_heartbeat_failed", "error", err)
		}
		return nil
	}
	if t.TaskType != "store" {
		return fmt.Errorf("unsupported task type %q", t.TaskType)
	}
	sha, size, err := a.downloadAndStore(ctx, t)
	if err != nil {
		a.log.Warn("task_storage_failed", "task_id", t.TaskID, "error", err)
		return a.reportResult(ctx, t, map[string]any{"lease_token": t.LeaseToken, "status": "failure", "error": truncate(err.Error(), 1900)})
	}
	if err := a.reportResult(ctx, t, map[string]any{"lease_token": t.LeaseToken, "status": "success", "sha256": sha, "size_bytes": size}); err != nil {
		return err
	}
	a.log.Info("task_verified", "task_id", t.TaskID, "sha256", sha, "size_bytes", size)
	if err := a.heartbeat(ctx); err != nil {
		a.log.Warn("post_task_heartbeat_failed", "error", err)
	}
	return nil
}

func (a *agent) fetchAndUploadSource(ctx context.Context, t task) error {
	validatedURL, err := validateAgentUploaderURL(t.SourceURL)
	if err != nil {
		return err
	}
	fetched, err := fetchUploaderFileWithLimiter(ctx, validatedURL, true, a.waitForUploaderInterval, t.SourcePassword)
	if err != nil {
		return err
	}
	defer fetched.Body.Close()
	if fetched.Size > maxAgentSourceBytes {
		return fmt.Errorf("source file exceeds the 5 GiB limit")
	}
	if strings.Contains(strings.ToLower(fetched.MediaType), "text/html") {
		return fmt.Errorf("uploader.jp returned an HTML page instead of file content")
	}
	endpoint := a.cfg.APIURL + "/api/v1/nodes/" + a.creds.NodeID + "/tasks/" + t.TaskID + "/source-content?lease_token=" + url.QueryEscape(t.LeaseToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, fetched.Body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.creds.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Source-Filename", safeSourceFilename(fetched.Filename))
	req.Header.Set("X-Source-Media-Type", fetched.MediaType)
	if fetched.Size >= 0 {
		req.ContentLength = fetched.Size
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("upload fetched content to central API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("source content upload returned %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	return nil
}

func (a *agent) loadUploaderFetchTimestamp() {
	path := a.cfg.StatePath + ".uploader-last-fetch"
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(b)))
	if err == nil {
		a.lastUploaderFetch = parsed
	}
}

func (a *agent) persistUploaderFetchTimestamp() {
	path := a.cfg.StatePath + ".uploader-last-fetch"
	if err := os.WriteFile(path, []byte(a.lastUploaderFetch.UTC().Format(time.RFC3339Nano)), 0o600); err != nil {
		a.log.Warn("uploader_rate_limit_state_write_failed", "error", err)
	}
}

func (a *agent) waitForUploaderInterval(ctx context.Context) error {
	interval := a.cfg.UploaderMinInterval
	if !a.lastUploaderFetch.IsZero() {
		wait := interval - time.Since(a.lastUploaderFetch)
		if wait > 0 {
			a.log.Info("uploader_rate_limit_wait", "wait", wait.String(), "minimum_interval", interval.String())
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	a.lastUploaderFetch = time.Now()
	a.persistUploaderFetchTimestamp()
	return nil
}

func (a *agent) uploadReplica(ctx context.Context, t task) error {
	if t.TargetNodeID == nil || *t.TargetNodeID == "" {
		return errors.New("replication task has no target node")
	}
	sourcePath, err := safeStoragePath(a.cfg.StoragePath, t.StorageKey)
	if err != nil {
		return err
	}
	sha, size, err := hashFile(sourcePath)
	if err != nil {
		return fmt.Errorf("could not read source placement: %w", err)
	}
	if size != t.SizeBytes || t.SHA256 == nil || !strings.EqualFold(sha, *t.SHA256) {
		return fmt.Errorf("source placement failed verification: expected size=%d sha256=%v, received size=%d sha256=%s", t.SizeBytes, t.SHA256, size, sha)
	}
	f, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer f.Close()
	uploadURL := a.cfg.APIURL + "/api/v1/nodes/" + a.creds.NodeID + "/tasks/" + t.TaskID + "/replica-content?lease_token=" + url.QueryEscape(t.LeaseToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, f)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.creds.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = size
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("replica content upload returned %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	return nil
}

func (a *agent) downloadAndStore(ctx context.Context, t task) (string, int64, error) {
	destination, err := safeStoragePath(a.cfg.StoragePath, t.StorageKey)
	if err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return "", 0, err
	}
	// A queued retry is not a verified placement. Remove any final object left by
	// an earlier failed attempt, as well as abandoned partial downloads, before
	// receiving the payload again.
	if err := cleanupUnverifiedObject(destination); err != nil {
		return "", 0, err
	}
	contentURL := a.cfg.APIURL + "/api/v1/nodes/" + a.creds.NodeID + "/tasks/" + t.TaskID + "/content?lease_token=" + url.QueryEscape(t.LeaseToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, contentURL, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+a.creds.Token)
	resp, err := a.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", 0, fmt.Errorf("download content returned %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".partial-*")
	if err != nil {
		return "", 0, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hasher), resp.Body)
	if err != nil {
		_ = tmp.Close()
		return "", 0, err
	}
	if size != t.SizeBytes {
		_ = tmp.Close()
		return "", size, fmt.Errorf("size mismatch: expected %d, received %d", t.SizeBytes, size)
	}
	sha := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(sha, *t.SHA256) {
		_ = tmp.Close()
		return sha, size, fmt.Errorf("SHA-256 mismatch: expected %s, received %s", *t.SHA256, sha)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", size, err
	}
	if err := tmp.Close(); err != nil {
		return "", size, err
	}
	if err := os.Rename(tmpPath, destination); err != nil {
		return "", size, err
	}
	// Re-open the final path and hash it again to verify the bytes actually persisted at the target path.
	finalSHA, finalSize, err := hashFile(destination)
	if err != nil {
		_ = os.Remove(destination)
		return "", size, err
	}
	if finalSize != t.SizeBytes || !strings.EqualFold(finalSHA, *t.SHA256) {
		_ = os.Remove(destination)
		return finalSHA, finalSize, errors.New("post-write verification failed")
	}
	return finalSHA, finalSize, nil
}

func cleanupUnverifiedObject(destination string) error {
	if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("could not remove stale unverified object: %w", err)
	}
	partials, err := filepath.Glob(filepath.Join(filepath.Dir(destination), ".partial-*"))
	if err != nil {
		return err
	}
	for _, partial := range partials {
		if err := os.Remove(partial); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("could not remove stale partial object: %w", err)
		}
	}
	return nil
}

func (a *agent) deleteStoredObject(storageKey string) error {
	path, err := safeStoragePath(a.cfg.StoragePath, storageKey)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("could not remove stored object: %w", err)
	}
	return nil
}

func safeStoragePath(root, key string) (string, error) {
	if filepath.IsAbs(key) || strings.Contains(key, "\\") {
		return "", errors.New("invalid storage key")
	}
	clean := filepath.Clean(key)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid storage key")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(filepath.Join(rootAbs, clean))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("storage key escapes the managed storage root")
	}
	return pathAbs, nil
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", size, err
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

func (a *agent) reportResult(ctx context.Context, t task, payload map[string]any) error {
	return a.postJSON(ctx, "/api/v1/nodes/"+a.creds.NodeID+"/tasks/"+t.TaskID+"/result", payload, nil)
}

func (a *agent) postJSON(ctx context.Context, path string, payload any, output any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, a.cfg.APIURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.creds.Token)
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("POST %s returned %s: %s", path, resp.Status, strings.TrimSpace(string(payload)))
	}
	if output != nil {
		return json.NewDecoder(resp.Body).Decode(output)
	}
	return nil
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

type deliveryJob struct {
	JobID      string  `json:"job_id"`
	LeaseToken *string `json:"lease_token"`
	StorageKey string  `json:"storage_key"`
	SizeBytes  int64   `json:"size_bytes"`
	SHA256     *string `json:"sha256"`
}

func (a *agent) claimAndProcessDelivery(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.APIURL+"/api/v1/nodes/"+a.creds.NodeID+"/delivery-jobs/claim", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.creds.Token)
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("claim delivery job: status %d", resp.StatusCode)
	}
	var job deliveryJob
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return err
	}
	if job.JobID == "" || job.LeaseToken == nil || *job.LeaseToken == "" {
		return errors.New("delivery job response missing lease")
	}
	path, err := safeStoragePath(a.cfg.StoragePath, job.StorageKey)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() != job.SizeBytes {
		return fmt.Errorf("delivery source size mismatch")
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if job.SHA256 == nil || hex.EncodeToString(h.Sum(nil)) != strings.ToLower(*job.SHA256) {
		return fmt.Errorf("delivery source sha256 mismatch")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	u := a.cfg.APIURL + "/api/v1/nodes/" + a.creds.NodeID + "/delivery-jobs/" + job.JobID + "/content?lease_token=" + url.QueryEscape(*job.LeaseToken)
	upload, err := http.NewRequestWithContext(ctx, http.MethodPost, u, f)
	if err != nil {
		return err
	}
	upload.Header.Set("Authorization", "Bearer "+a.creds.Token)
	upload.Header.Set("Content-Type", "application/octet-stream")
	upload.ContentLength = job.SizeBytes
	upResp, err := a.client.Do(upload)
	if err != nil {
		return err
	}
	defer upResp.Body.Close()
	_, _ = io.Copy(io.Discard, upResp.Body)
	if upResp.StatusCode != http.StatusCreated {
		return fmt.Errorf("upload delivery content: status %d", upResp.StatusCode)
	}
	return nil
}
