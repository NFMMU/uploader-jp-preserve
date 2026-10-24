package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const dockerAPIVersion = "v1.45"
const agentContainerName = "archive-node-agent"

var (
	namePattern   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._ -]{0,99}$`)
	volumePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)
	imagePattern  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]{0,255}$`)
)

type nodeConfig struct {
	CentralAPIURL string `json:"central_api_url"`
	Enrollment    string `json:"enrollment_token"`
	Name          string `json:"name"`
	CapacityBytes int64  `json:"capacity_bytes"`
}

type manager struct {
	listenAddr        string
	configPath        string
	image             string
	volume            string
	allowInsecureHTTP bool
	docker            *dockerClient
	csrf              string
	logger            *slog.Logger
	mu                sync.Mutex
	cfg               *nodeConfig
	lastError         string
}

type pageData struct {
	Configured bool
	NodeName   string
	APIURL     string
	Capacity   int64
	Image      string
	Status     string
	LastError  string
	CSRF       string
}

var pageTemplate = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="ja">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Archive Node Manager</title>
<style>
body{font-family:system-ui,sans-serif;max-width:720px;margin:2rem auto;padding:0 1rem;line-height:1.6;color:#202124}
label{display:block;margin:.9rem 0 .25rem;font-weight:600}input{box-sizing:border-box;width:100%;padding:.7rem;border:1px solid #bbb;border-radius:6px;font:inherit}
button{margin-top:1.2rem;padding:.7rem 1rem;border:0;border-radius:6px;background:#155eef;color:white;font:inherit;cursor:pointer}
.panel{border:1px solid #ddd;border-radius:10px;padding:1rem;margin:1rem 0}.error{color:#b42318;white-space:pre-wrap}.muted{color:#666;font-size:.92rem}
</style>
</head>
<body>
<h1>Archive Node Manager</h1>
<p>この画面は、この PC 上の Node Agent を設定・管理します。管理画面をインターネットや LAN に公開しないでください。</p>
{{if .LastError}}<div class="panel error"><strong>問題が発生しました</strong><br>{{.LastError}}</div>{{end}}
{{if .Configured}}
<div class="panel">
<h2>ノード設定</h2>
<p><strong>ノード名:</strong> {{.NodeName}}</p>
<p><strong>中央 API:</strong> {{.APIURL}}</p>
<p><strong>提供容量:</strong> {{.Capacity}} bytes</p>
<p><strong>Agent イメージ:</strong> {{.Image}}</p>
<p><strong>Agent 状態:</strong> {{.Status}}</p>
<p class="muted">参加トークンは再表示しません。設定の変更機能は今後追加します。</p>
<form method="post" action="/">
<input type="hidden" name="csrf" value="{{.CSRF}}">
<input type="hidden" name="action" value="retry">
<button type="submit">Agent の状態を再確認・起動</button>
</form>
</div>
{{else}}
<div class="panel">
<h2>初期設定</h2>
<form method="post" action="/">
<input type="hidden" name="csrf" value="{{.CSRF}}">
<input type="hidden" name="action" value="configure">
<label for="central_api_url">中央 API URL</label>
<input id="central_api_url" name="central_api_url" type="url" placeholder="https://archive.example.org" required>
<label for="enrollment_token">参加登録トークン</label>
<input id="enrollment_token" name="enrollment_token" type="password" autocomplete="new-password" required>
<label for="name">ノード名</label>
<input id="name" name="name" maxlength="100" placeholder="my-archive-node" required>
<label for="capacity_bytes">提供容量（バイト）</label>
<input id="capacity_bytes" name="capacity_bytes" type="number" min="0" step="1" value="10737418240" required>
<p class="muted">10 GiB は 10737418240 バイトです。0 は参加者側で上限を設定しない指定です。</p>
<button type="submit">設定を保存して Agent を起動</button>
</form>
</div>
{{end}}
<p class="muted">Manager MVP · 自動更新はまだ有効化されていません。</p>
</body>
</html>`))

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	listenAddr := envOr("LISTEN_ADDR", ":8080")
	configDir := envOr("MANAGER_CONFIG_DIR", "/var/lib/archive-node-manager")
	image := envOr("NODE_AGENT_IMAGE", "archive-node-agent:local")
	volume := envOr("NODE_DATA_VOLUME", "archive-node-data")
	socket := envOr("DOCKER_SOCKET", "/var/run/docker.sock")
	if !imagePattern.MatchString(image) {
		logger.Error("invalid_NODE_AGENT_IMAGE")
		os.Exit(1)
	}
	if !volumePattern.MatchString(volume) {
		logger.Error("invalid_NODE_DATA_VOLUME")
		os.Exit(1)
	}
	csrf, err := randomToken(32)
	if err != nil {
		logger.Error("csrf_token_generation_failed", "error", err)
		os.Exit(1)
	}
	m := &manager{
		listenAddr:        listenAddr,
		configPath:        filepath.Join(configDir, "config.json"),
		image:             image,
		volume:            volume,
		allowInsecureHTTP: strings.EqualFold(envOr("ALLOW_INSECURE_HTTP", "false"), "true"),
		docker:            newDockerClient(socket),
		csrf:              csrf,
		logger:            logger,
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		logger.Error("config_directory_failed", "error", err)
		os.Exit(1)
	}
	if err := m.loadConfig(); err != nil {
		logger.Error("config_load_failed", "error", err)
		os.Exit(1)
	}
	if m.cfg != nil {
		if err := m.reconcile(context.Background()); err != nil {
			m.setError(err)
			logger.Error("agent_reconcile_failed", "error", err)
		}
	}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			ctx, cancel := context.WithTimeout(
				context.Background(),
				45*time.Second,
			)

			err := m.reconcile(ctx)
			cancel()

			if err != nil {
				m.setError(err)
				logger.Error("agent_reconcile_failed", "error", err)
			} else {
				m.setError(nil)
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("/", m.handleIndex)
	server := &http.Server{
		Addr:              listenAddr,
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	logger.Info("node_manager_started", "listen_addr", listenAddr, "agent_image", image, "data_volume", volume)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http_server_failed", "error", err)
		os.Exit(1)
	}
}

func (m *manager) handleIndex(w http.ResponseWriter, r *http.Request) {
	if !isLocalHost(r.Host) {
		http.Error(w, "local access only", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodPost {
		m.handlePost(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	m.render(w)
}

func (m *manager) handlePost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	if r.FormValue("csrf") != m.csrf {
		http.Error(w, "invalid form token; reload the page", http.StatusForbidden)
		return
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); !originHeaderAllowed(origin, r.Host) {
		m.logger.Warn("origin_rejected", "origin", origin, "request_host", r.Host)
		http.Error(w, "origin rejected", http.StatusForbidden)
		return
	}
	m.mu.Lock()
	configured := m.cfg != nil
	m.mu.Unlock()

	action := r.FormValue("action")
	var err error
	switch action {
	case "configure":
		if configured {
			http.Error(w, "configuration is already saved", http.StatusConflict)
			return
		}
		cfg, cfgErr := parseConfig(r, m.allowInsecureHTTP)
		if cfgErr != nil {
			m.setError(cfgErr)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if err = m.saveConfig(cfg); err == nil {
			err = m.reconcile(r.Context())
		}
	case "retry":
		if !configured {
			http.Error(w, "complete initial setup first", http.StatusConflict)
			return
		}
		err = m.reconcile(r.Context())
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	m.setError(err)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func parseConfig(r *http.Request, allowInsecureHTTP bool) (*nodeConfig, error) {
	apiURL := strings.TrimRight(strings.TrimSpace(r.FormValue("central_api_url")), "/")
	u, err := url.Parse(apiURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Fragment != "" {
		return nil, errors.New("中央 API URL は http(s) の絶対 URL で指定してください")
	}
	if u.Scheme == "http" && !isLoopbackHost(u.Hostname()) && !allowInsecureHTTP {
		return nil, errors.New("HTTP はローカル開発環境でのみ利用できます。本番環境では HTTPS を使用してください")
	}
	token := strings.TrimSpace(r.FormValue("enrollment_token"))
	if token == "" || len(token) > 4096 {
		return nil, errors.New("参加登録トークンが空、または長すぎます")
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if !namePattern.MatchString(name) {
		return nil, errors.New("ノード名は英数字、空白、ピリオド、ハイフン、アンダースコアを使い、100 文字以内にしてください")
	}
	capacity, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("capacity_bytes")), 10, 64)
	if err != nil || capacity < 0 {
		return nil, errors.New("提供容量は 0 以上の整数（バイト）で指定してください")
	}
	return &nodeConfig{CentralAPIURL: apiURL, Enrollment: token, Name: name, CapacityBytes: capacity}, nil
}

func (m *manager) render(w http.ResponseWriter) {
	m.mu.Lock()
	cfg := m.cfg
	lastErr := m.lastError
	m.mu.Unlock()
	data := pageData{Configured: cfg != nil, Image: m.image, Status: "not created", LastError: lastErr, CSRF: m.csrf}
	if cfg != nil {
		data.NodeName = cfg.Name
		data.APIURL = cfg.CentralAPIURL
		data.Capacity = cfg.CapacityBytes
		status, err := m.agentStatus(context.Background())
		if err != nil {
			data.Status = "確認できません: " + err.Error()
		} else {
			data.Status = status
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := pageTemplate.Execute(w, data); err != nil {
		m.logger.Error("render_failed", "error", err)
	}
}

func (m *manager) loadConfig() error {
	b, err := os.ReadFile(m.configPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var cfg nodeConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return fmt.Errorf("設定ファイルが不正です: %w", err)
	}
	if cfg.CentralAPIURL == "" || cfg.Enrollment == "" || cfg.Name == "" || cfg.CapacityBytes < 0 {
		return errors.New("設定ファイルに必須項目がありません")
	}
	m.cfg = &cfg
	return nil
}

func (m *manager) saveConfig(cfg *nodeConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cfg != nil {
		return errors.New("設定はすでに保存されています")
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(m.configPath), ".config-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, m.configPath); err != nil {
		return err
	}
	m.cfg = cfg
	ok = true
	return nil
}

func (m *manager) reconcile(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cfg == nil {
		return nil
	}
	if err := m.docker.ensureVolume(ctx, m.volume); err != nil {
		return err
	}
	if _, err := m.docker.inspectImage(ctx, m.image); err != nil {
		return fmt.Errorf("Agent イメージ %q を確認できません。先にイメージを用意してください: %w", m.image, err)
	}
	container, status, err := m.docker.inspectContainer(ctx, agentContainerName)
	if err == nil {
		if status == "running" {
			return nil
		}
		return m.docker.startContainer(ctx, container.ID)
	}
	if !errors.Is(err, errNotFound) {
		return err
	}
	env := []string{
		"CENTRAL_API_URL=" + m.cfg.CentralAPIURL,
		"NODE_ENROLLMENT_TOKEN=" + m.cfg.Enrollment,
		"NODE_NAME=" + m.cfg.Name,
		"NODE_TYPE=B",
		"NODE_CAPACITY_BYTES=" + strconv.FormatInt(m.cfg.CapacityBytes, 10),
		"NODE_STORAGE_PATH=/data/objects",
		"NODE_STATE_PATH=/data/node-credentials.json",
	}
	id, err := m.docker.createContainer(ctx, agentContainerName, m.image, env, m.volume)
	if err != nil {
		return err
	}
	if err := m.docker.startContainer(ctx, id); err != nil {
		return fmt.Errorf("Agent コンテナは作成されましたが起動できません: %w", err)
	}
	m.logger.Info("node_agent_created_and_started", "container_id", id, "image", m.image, "volume", m.volume)
	return nil
}

func (m *manager) agentStatus(ctx context.Context) (string, error) {
	_, status, err := m.docker.inspectContainer(ctx, agentContainerName)
	if errors.Is(err, errNotFound) {
		return "not created", nil
	}
	if err != nil {
		return "unknown", err
	}
	return status, nil
}

func (m *manager) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err == nil {
		m.lastError = ""
		return
	}
	m.lastError = err.Error()
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func isLocalHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	host = strings.Trim(host, "[]")
	return strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

// originHeaderAllowed permits a missing or opaque ("null") Origin only after
// handlePost has validated the synchronizer CSRF token. All non-opaque origins
// must match the local request host and port.
func originHeaderAllowed(origin, requestHost string) bool {
	if origin == "" || origin == "null" {
		return true
	}
	return originMatchesLocalHost(origin, requestHost)
}

func originMatchesLocalHost(origin, requestHost string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if !isLoopbackHost(u.Hostname()) {
		return false
	}
	requestName, requestPort := splitHostPort(requestHost)
	if !isLoopbackHost(requestName) {
		return false
	}
	return u.Port() == requestPort
}

func splitHostPort(hostport string) (string, string) {
	host, port, err := net.SplitHostPort(hostport)
	if err == nil {
		return strings.Trim(host, "[]"), port
	}
	return strings.Trim(hostport, "[]"), ""
}

func isLoopbackHost(host string) bool {
	return strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

type dockerClient struct {
	client     *http.Client
	socketPath string
}

type dockerContainerInspect struct {
	ID     string `json:"Id"`
	Config struct {
		Image string   `json:"Image"`
		Env   []string `json:"Env"`
	} `json:"Config"`
	State struct {
		Status  string `json:"Status"`
		Running bool   `json:"Running"`
	} `json:"State"`
}

var errNotFound = errors.New("docker object not found")

func newDockerClient(socketPath string) *dockerClient {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &dockerClient{client: &http.Client{Transport: transport, Timeout: 30 * time.Second}, socketPath: socketPath}
}

func (d *dockerClient) request(ctx context.Context, method, path string, payload any) (int, []byte, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		body = strings.NewReader(string(b))
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://docker/"+dockerAPIVersion+"/"+strings.TrimPrefix(path, "/"), body)
	if err != nil {
		return 0, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("Docker Engine に接続できません (%s): %w", d.socketPath, err)
	}
	defer resp.Body.Close()
	b, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return resp.StatusCode, nil, readErr
	}
	return resp.StatusCode, b, nil
}

func (d *dockerClient) ensureVolume(ctx context.Context, name string) error {
	status, body, err := d.request(ctx, http.MethodGet, "/volumes/"+url.PathEscape(name), nil)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		return nil
	}
	if status != http.StatusNotFound {
		return dockerError("volume inspect", status, body)
	}
	status, body, err = d.request(ctx, http.MethodPost, "/volumes/create", map[string]string{"Name": name, "Driver": "local"})
	if err != nil {
		return err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return dockerError("volume create", status, body)
	}
	return nil
}

func (d *dockerClient) inspectImage(ctx context.Context, image string) (bool, error) {
	status, body, err := d.request(ctx, http.MethodGet, "/images/"+image+"/json", nil)
	if err != nil {
		return false, err
	}
	if status == http.StatusNotFound {
		return false, fmt.Errorf("image not found: %s", image)
	}
	if status != http.StatusOK {
		return false, dockerError("image inspect", status, body)
	}
	return true, nil
}

func (d *dockerClient) inspectContainer(ctx context.Context, name string) (dockerContainerInspect, string, error) {
	status, body, err := d.request(ctx, http.MethodGet, "/containers/"+url.PathEscape(name)+"/json", nil)
	if err != nil {
		return dockerContainerInspect{}, "", err
	}
	if status == http.StatusNotFound {
		return dockerContainerInspect{}, "", errNotFound
	}
	if status != http.StatusOK {
		return dockerContainerInspect{}, "", dockerError("container inspect", status, body)
	}
	var info dockerContainerInspect
	if err := json.Unmarshal(body, &info); err != nil {
		return dockerContainerInspect{}, "", err
	}
	return info, info.State.Status, nil
}

func (d *dockerClient) createContainer(ctx context.Context, name, image string, env []string, volume string) (string, error) {
	payload := map[string]any{
		"Image": image,
		"Env":   env,
		"HostConfig": map[string]any{
			"RestartPolicy": map[string]string{"Name": "unless-stopped"},
			"Mounts":        []map[string]string{{"Type": "volume", "Source": volume, "Target": "/data"}},
			"NetworkMode":   "bridge",
		},
	}
	status, body, err := d.request(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), payload)
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated {
		return "", dockerError("container create", status, body)
	}
	var response struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	if response.ID == "" {
		return "", errors.New("Docker Engine did not return a container ID")
	}
	return response.ID, nil
}

func (d *dockerClient) startContainer(ctx context.Context, id string) error {
	status, body, err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/start", nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent && status != http.StatusNotModified {
		return dockerError("container start", status, body)
	}
	return nil
}

func dockerError(operation string, status int, body []byte) error {
	message := strings.TrimSpace(string(body))
	if len(message) > 600 {
		message = message[:600]
	}
	return fmt.Errorf("Docker %s failed (HTTP %d): %s", operation, status, message)
}
