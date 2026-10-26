import React, { useEffect, useRef, useState } from "react";
import {
  NavLink,
  Routes,
  Route,
  useNavigate,
  useLocation,
  useParams,
} from "react-router-dom";
import PublicSite from "./PublicSite";
import CopyEmail from "./CopyEmail";
import {
  Archive,
  LayoutDashboard,
  Files,
  HardDrive,
  Workflow,
  ClipboardList,
  Users,
  ScrollText,
  LogOut,
  Search,
  Plus,
  RefreshCw,
  ShieldCheck,
  AlertTriangle,
  CloudUpload,
  ExternalLink,
  Menu,
  X,
  Download,
  CheckCircle2,
  Clock3,
  LockKeyhole,
  Globe,
} from "lucide-react";
type Obj = Record<string, any>;

const apiBase = "/api/v1";
type APIError = { error?: string; message?: string };
async function publicAPI<T>(
  path: string,
  token: string,
  options: RequestInit = {},
): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    ...options,
    headers: {
      Authorization: "Bearer " + token,
      ...(options.body ? { "Content-Type": "application/json" } : {}),
      ...options.headers,
    },
  });
  const text = await response.text();
  let data: any = {};
  try {
    data = text ? JSON.parse(text) : {};
  } catch {
    data = { message: text };
  }
  if (!response.ok)
    throw new Error(
      (data as APIError).error ||
        (data as APIError).message ||
        `HTTP ${response.status}`,
    );
  return data as T;
}

function App() {
  const [token, setToken] = useState(
    localStorage.getItem("archive_token") || "",
  );
  const [user, setUser] = useState<Obj | null>(
    JSON.parse(localStorage.getItem("archive_user") || "null"),
  );
  const [toast, setToast] = useState("");
  const [busy, setBusy] = useState(false);
  const [mobile, setMobile] = useState(false);
  const nav = useNavigate();
  const loc = useLocation();
  const [refresh, setRefresh] = useState(0);
  async function api(path: string, opts: RequestInit = {}) {
    const headers = new Headers(opts.headers);
    if (token) headers.set("Authorization", "Bearer " + token);
    if (opts.body && !(opts.body instanceof FormData))
      headers.set("Content-Type", "application/json");
    const r = await fetch(apiBase + path, { ...opts, headers });
    const text = await r.text();
    let data: Obj = {};
    try {
      data = text ? JSON.parse(text) : {};
    } catch {
      data = { message: text };
    }
    if (!r.ok)
      throw new Error(data.error || data.message || `HTTP ${r.status}`);
    return data;
  }
  function notify(s: string) {
    setToast(s);
    setTimeout(() => setToast(""), 3500);
  }
  async function login(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    const f = new FormData(e.currentTarget);
    try {
      const d = await api("/auth/login", {
        method: "POST",
        body: JSON.stringify({
          email: f.get("email"),
          password: f.get("password"),
        }),
      });
      localStorage.setItem("archive_token", d.access_token);
      localStorage.setItem("archive_user", JSON.stringify(d.user));
      setToken(d.access_token);
      setUser(d.user);
      notify("ログインしました");
      nav("/admin");
    } catch (e) {
      notify((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  function logout() {
    localStorage.removeItem("archive_token");
    localStorage.removeItem("archive_user");
    setToken("");
    setUser(null);
    nav("/admin/login");
  }
  const links = [
    ["/admin", "概要", LayoutDashboard],
    ["/admin/review", "保全レビュー", ClipboardList],
    ["/admin/url-collector", "URL自動収集", Globe],
    ["/admin/nodes", "保存ノード", HardDrive],
    ["/admin/tasks", "タスク", Workflow],
    ["/admin/users", "ユーザー管理", Users],
    ["/admin/audit", "操作履歴", ScrollText],
  ] as const;
  const visibleLinks =
    user?.role === "admin"
      ? links
      : links.filter(([to]) => to !== "/admin/url-collector");
  if (!loc.pathname.startsWith("/admin")) return <PublicSite />;
  if (!token || !user)
    return (
      <div className="login-shell">
        <div className="login-art">
          <div className="brand">
            <div className="brand-icon">
              <Archive />
            </div>
            <div>
              <b>uploader.jp保全プロジェクト</b>
              <small>アーカイブ管理</small>
            </div>
          </div>
          <div className="hero">
            <span className="eyebrow"></span>
            <h1>uploader.jp保全プロジェクト</h1>
            <p>作品情報とファイルを保全します。</p>
            <div className="hero-stats">
              <span>
                <ShieldCheck /> ファイル検証
              </span>
              <span>
                <HardDrive /> 分散保存
              </span>
              <span>
                <LockKeyhole /> 権限管理
              </span>
            </div>
          </div>
          <div className="login-foot"></div>
        </div>
        <div className="login-form-wrap">
          <form className="login-form" onSubmit={login}>
            <span className="eyebrow"></span>
            <h2>ログイン</h2>
            <p>アカウント情報を入力してください。</p>
            <label>
              メールアドレス
              <input
                type="email"
                name="email"
                placeholder="admin@example.jp"
                required
                autoComplete="username"
              />
            </label>
            <label>
              パスワード
              <input
                type="password"
                name="password"
                placeholder="••••••••••••"
                required
                autoComplete="current-password"
              />
            </label>
            <button className="primary wide" disabled={busy}>
              {busy ? "認証中…" : "ログインする"} <span>→</span>
            </button>
            <div className="login-note">
              <LockKeyhole size={15} /> 通信は暗号化されます
            </div>
          </form>
        </div>
        {toast && <div className="toast error">{toast}</div>}
      </div>
    );
  return (
    <div className="app-shell">
      <aside className={"sidebar " + (mobile ? "open" : "")}>
        <div className="brand">
          <div className="brand-icon">
            <Archive />
          </div>
          <div>
            <b>uploader.jp保全プロジェクト</b>
            <small>管理画面</small>
          </div>
          <button
            className="icon-btn mobile-close"
            onClick={() => setMobile(false)}
          >
            <X />
          </button>
        </div>
        <div className="side-caption">メニュー</div>
        <nav>
          {visibleLinks.map(([to, label, Icon]) => (
            <NavLink
              key={to}
              to={to}
              end={to === "/admin"}
              onClick={() => setMobile(false)}
            >
              <Icon size={18} />
              <span>{label}</span>
              {to === "/admin/review" && <i className="nav-dot" />}
            </NavLink>
          ))}
        </nav>
        <div className="side-bottom">
          <div className="system-status">
            <span className="pulse" />
            <div>
              <b>API 接続中</b>
              <small>中央サーバー</small>
            </div>
            <span className="status-label">LIVE</span>
          </div>
          <div className="user-card">
            <div className="avatar">
              {(user.display_name || user.email || "A")
                .slice(0, 1)
                .toUpperCase()}
            </div>
            <div className="user-meta">
              <b>{user.display_name || user.email}</b>
              <small>
                {user.role === "admin"
                  ? "管理者"
                  : user.role === "reviewer"
                    ? "確認担当者"
                    : "閲覧者"}
              </small>
            </div>
            <button className="icon-btn" title="ログアウト" onClick={logout}>
              <LogOut size={16} />
            </button>
          </div>
        </div>
      </aside>
      <main className="main">
        <header className="topbar">
          <button
            className="icon-btn mobile-menu"
            onClick={() => setMobile(true)}
          >
            <Menu />
          </button>
          <div className="breadcrumbs">
            管理 <span>/</span>{" "}
            <b>{links.find((l) => l[0] === loc.pathname)?.[1] || "詳細"}</b>
          </div>
          <div className="top-actions">
            <span className="secure-tag">
              <span className="pulse" />{" "}
            </span>
            <button
              className="icon-btn"
              title="再読み込み"
              onClick={() => setRefresh((x) => x + 1)}
            >
              <RefreshCw size={17} />
            </button>
            <div className="top-avatar">
              {(user.display_name || user.email || "A")
                .slice(0, 1)
                .toUpperCase()}
            </div>
          </div>
        </header>
        <div className="page">
          <Routes>
            <Route
              path="/admin"
              element={
                <Dashboard
                  api={api}
                  refresh={refresh}
                  notify={notify}
                  token={token}
                />
              }
            />
            <Route
              path="/admin/review"
              element={
                <ReviewWorkspace
                  api={api}
                  refresh={refresh}
                  notify={notify}
                  role={user.role}
                  token={token}
                  bumpRefresh={() => setRefresh((x) => x + 1)}
                  initialTab="files"
                />
              }
            />
            <Route
              path="/admin/url-collector"
              element={
                <URLCollector
                  api={api}
                  refresh={refresh}
                  notify={notify}
                  role={user.role}
                  bumpRefresh={() => setRefresh((x) => x + 1)}
                />
              }
            />
            <Route
              path="/admin/artifacts"
              element={
                <ReviewWorkspace
                  api={api}
                  refresh={refresh}
                  notify={notify}
                  role={user.role}
                  token={token}
                  bumpRefresh={() => setRefresh((x) => x + 1)}
                  initialTab="files"
                />
              }
            />
            <Route
              path="/admin/nodes"
              element={
                <Nodes
                  api={api}
                  refresh={refresh}
                  notify={notify}
                  role={user.role}
                />
              }
            />
            <Route
              path="/admin/tasks"
              element={
                <Tasks
                  api={api}
                  refresh={refresh}
                  notify={notify}
                  role={user.role}
                />
              }
            />
            <Route
              path="/admin/requests"
              element={
                <ReviewWorkspace
                  api={api}
                  refresh={refresh}
                  notify={notify}
                  role={user.role}
                  token={token}
                  bumpRefresh={() => setRefresh((x) => x + 1)}
                  initialTab="requests"
                />
              }
            />
            <Route
              path="/admin/requests/:id"
              element={
                <RequestDetailPage
                  api={api}
                  refresh={refresh}
                  notify={notify}
                  role={user.role}
                  token={token}
                  bumpRefresh={() => setRefresh((x) => x + 1)}
                />
              }
            />
            <Route
              path="/admin/users"
              element={
                <UsersPage
                  api={api}
                  refresh={refresh}
                  notify={notify}
                  role={user.role}
                />
              }
            />
            <Route
              path="/admin/audit"
              element={<Audit api={api} refresh={refresh} />}
            />
            <Route
              path="*"
              element={
                <Dashboard
                  api={api}
                  refresh={refresh}
                  notify={notify}
                  token={token}
                />
              }
            />
          </Routes>
        </div>
        <footer>
          <b>uploader.jp保全プロジェクト</b> <span>·</span>{" "}
          運営：全国もちもちうどん連盟 <span>·</span> © 2026
          全国もちもちうどん連盟{" "}
          <span className="footer-right">
            <a href="mailto:uploaderjp-archive@udonya.moe">
              uploaderjp-archive@udonya.moe
            </a>
            <CopyEmail email="uploaderjp-archive@udonya.moe" />
          </span>
        </footer>
      </main>
      {toast && <div className="toast">{toast}</div>}
    </div>
  );
}
function useData(
  api: (p: string, o?: RequestInit) => Promise<Obj>,
  path: string,
  refresh: number,
) {
  const [data, setData] = useState<Obj>({ items: [] });
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    let live = true;
    setLoading(true);
    api(path)
      .then((d) => {
        if (live) {
          setData(d);
          setError("");
        }
      })
      .catch((e) => {
        if (live) setError(e.message);
      })
      .finally(() => live && setLoading(false));
    return () => {
      live = false;
    };
  }, [path, refresh]);
  return { data, error, loading };
}
function Heading({
  eyebrow,
  title,
  desc,
  actions,
}: {
  eyebrow: string;
  title: string;
  desc: string;
  actions?: React.ReactNode;
}) {
  return (
    <div className="page-heading">
      <div>
        <div className="eyebrow">{eyebrow}</div>
        <h1>{title}</h1>
        <p>{desc}</p>
      </div>
      {actions && <div className="heading-actions">{actions}</div>}
    </div>
  );
}
function Stat({
  label,
  value,
  sub,
  icon: Icon,
  tone = "blue",
}: {
  label: string;
  value: React.ReactNode;
  sub: string;
  icon: any;
  tone?: string;
}) {
  return (
    <div className="stat-card">
      <div className={"stat-icon " + tone}>
        <Icon size={19} />
      </div>
      <div className="stat-label">{label}</div>
      <div className="stat-value">{value}</div>
      <div className="stat-sub">{sub}</div>
    </div>
  );
}
function Dashboard({ api, refresh, notify, token }: any) {
  const a = useData(api, "/artifacts", refresh),
    f = useData(api, "/files", refresh),
    n = useData(api, "/nodes", refresh),
    t = useData(api, "/tasks", refresh),
    r = useData(api, "/public-requests?status=pending", refresh);
  const files = f.data.items || [],
    nodes = n.data.items || [],
    tasks = t.data.items || [];
  const degraded = files.filter((x: Obj) =>
    ["degraded", "missing", "error"].includes(x.preservation_status),
  ).length;
  const failed = tasks.filter((x: Obj) => x.status === "failed").length;
  return (
    <>
      <Heading
        eyebrow=""
        title="保全状況"
        desc="保存状況と要対応項目"
        actions={
          <>
            <button
              className="secondary"
              onClick={() => downloadExport("json", token, notify)}
            >
              <Download size={16} /> 一覧を出力
            </button>
            <button
              className="primary"
              onClick={() =>
                api("/admin/placements/reconcile", { method: "POST" })
                  .then(() => notify("配置の再調整を実行しました"))
                  .catch((e: Error) => notify(e.message))
              }
            >
              <RefreshCw size={16} /> 保存先を調整
            </button>
          </>
        }
      />
      <div className="notice">
        <div className="notice-icon">
          <ShieldCheck size={19} />
        </div>
        <div>
          <b>保存と公開は別です</b>
          <p>権利未確認のファイルは、保存しても公開しません。</p>
        </div>
        <span className="notice-tag"></span>
      </div>
      <div className="stat-grid">
        <Stat
          label="作品数"
          value={a.data.total ?? (a.data.items || []).length}
          sub="登録済み"
          icon={Files}
        />
        <Stat
          label="保存ファイル"
          value={files.length}
          sub="登録ファイル数"
          icon={Archive}
          tone="violet"
        />
        <Stat
          label="稼働ノード"
          value={`${nodes.filter((x: Obj) => x.status === "online").length} / ${nodes.length}`}
          sub="稼働状況"
          icon={HardDrive}
          tone="green"
        />
        <Stat
          label="要対応"
          value={failed}
          sub="失敗したタスク"
          icon={Workflow}
          tone={failed ? "red" : "amber"}
        />
      </div>
      <div className="content-grid">
        <section className="panel">
          <div className="panel-head">
            <div>
              <h2>保存状態</h2>
              <p>ファイルの状態</p>
            </div>
            <span className="live-chip">
              <span className="pulse" />{" "}
            </span>
          </div>
          <div className="health-list">
            {[
              ["verified", "検証済み", "green"],
              ["queued", "保存待ち", "blue"],
              ["degraded", "劣化・保存先不足", "amber"],
              ["missing", "実体不明", "red"],
              ["error", "エラー", "red"],
            ].map(([key, label, tone]) => {
              const count = files.filter(
                (x: Obj) => x.preservation_status === key,
              ).length;
              return (
                <div className="health-row" key={key}>
                  <span className={"health-dot " + tone} />
                  <span>{label}</span>
                  <b>{count}</b>
                  <div className="mini-bar">
                    <i
                      className={tone}
                      style={{
                        width: `${files.length ? (count / files.length) * 100 : 0}%`,
                      }}
                    />
                  </div>
                </div>
              );
            })}
          </div>
          <div className="panel-foot">
            <span>
              要対応: <b>{degraded}</b>
            </span>
            <NavLink to="/admin/review?tab=files">一覧を見る →</NavLink>
          </div>
        </section>
        <section className="panel">
          <div className="panel-head">
            <div>
              <h2>操作</h2>
              <p>よく使う機能</p>
            </div>
          </div>
          <div className="quick-actions">
            <NavLink to="/admin/review?tab=files" className="quick-card">
              <div className="quick-icon blue">
                <Plus />
              </div>
              <div>
                <b>作品を登録</b>
                <small>情報を登録して保存</small>
              </div>
              <span>→</span>
            </NavLink>
            <NavLink to="/admin/nodes" className="quick-card">
              <div className="quick-icon green">
                <HardDrive />
              </div>
              <div>
                <b>ノードを確認</b>
                <small>容量と稼働状況</small>
              </div>
              <span>→</span>
            </NavLink>
            <NavLink to="/admin/review?tab=requests" className="quick-card">
              <div className="quick-icon amber">
                <ClipboardList />
              </div>
              <div>
                <b>申請を確認</b>
                <small>
                  未処理 {r.data.total ?? (r.data.items || []).length} 件
                </small>
              </div>
              <span>→</span>
            </NavLink>
            <NavLink to="/admin/tasks" className="quick-card">
              <div className="quick-icon violet">
                <Workflow />
              </div>
              <div>
                <b>タスク</b>
                <small>保存・検証</small>
              </div>
              <span>→</span>
            </NavLink>
          </div>
        </section>
      </div>
      <section className="panel table-panel">
        <div className="panel-head">
          <div>
            <h2>最近のタスク</h2>
            <p>処理状況</p>
          </div>
          <NavLink to="/admin/tasks" className="text-link">
            一覧 →
          </NavLink>
        </div>
        <TaskTable
          items={tasks.slice(0, 6)}
          loading={t.loading}
          error={t.error}
        />
      </section>
    </>
  );
}
function Status({ value }: { value: string }) {
  const map: Record<string, [string, string]> = {
    verified: ["検証済み", "green"],
    stored: ["保存済み", "green"],
    queued: ["待機中", "blue"],
    running: ["実行中", "blue"],
    succeeded: ["成功", "green"],
    completed: ["完了", "green"],
    failed: ["失敗", "red"],
    degraded: ["劣化", "amber"],
    missing: ["実体不明", "red"],
    deleted: ["削除済み", "gray"],
    error: ["エラー", "red"],
    online: ["オンライン", "green"],
    offline: ["オフライン", "red"],
    pending: ["未確認", "amber"],
    in_review: ["確認中", "blue"],
    allowed: ["再配布可", "green"],
    restricted: ["制限あり", "red"],
    unknown: ["不明", "amber"],
    in_progress: ["確認中", "blue"],
    uncollected: ["未取得", "gray"],
    accepted: ["承認", "green"],
    approved: ["承認済み", "green"],
    enrolled: ["登録済み", "green"],
    rejected: ["却下", "red"],
    closed: ["終了", "gray"],
  };
  const [label, tone] = map[value] || [value || "—", "gray"];
  return (
    <span className={"badge " + tone}>
      <i />
      {label}
    </span>
  );
}
function Table({
  headers,
  children,
  empty = "データがありません",
}: {
  headers: string[];
  children: React.ReactNode;
  empty?: string;
}) {
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            {headers.map((h) => (
              <th key={h}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {children || (
            <tr>
              <td colSpan={headers.length} className="empty">
                {empty}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
function TaskTable({
  items,
  loading,
  error,
}: {
  items: Obj[];
  loading: boolean;
  error: string;
}) {
  return (
    <Table headers={["タスクID", "種別", "状態", "優先度", "更新日時"]}>
      {loading ? (
        <tr>
          <td colSpan={5} className="empty">
            読み込み中…
          </td>
        </tr>
      ) : error ? (
        <tr>
          <td colSpan={5} className="empty">
            {error}
          </td>
        </tr>
      ) : items.length ? (
        items.map((x, i) => (
          <tr key={x.id || i}>
            <td>
              <code>{String(x.id || "").slice(0, 8) || "—"}</code>
            </td>
            <td>{x.task_type || "—"}</td>
            <td>
              <Status value={x.status} />
            </td>
            <td>
              <span className="priority">{x.priority ?? "—"}</span>
            </td>
            <td>{date(x.updated_at || x.created_at)}</td>
          </tr>
        ))
      ) : (
        <tr>
          <td colSpan={5} className="empty">
            タスクがありません
          </td>
        </tr>
      )}
    </Table>
  );
}
async function downloadExport(
  format: string,
  token: string,
  notify: (s: string) => void,
) {
  try {
    const r = await fetch(`/api/v1/export?format=${format}`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!r.ok) throw new Error("エクスポートに失敗しました");
    const blob = await r.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `archive-catalog.${format}`;
    a.click();
    URL.revokeObjectURL(url);
    notify("カタログを出力しました");
  } catch (e) {
    notify((e as Error).message);
  }
}
function date(v: string) {
  return v
    ? new Date(v).toLocaleString("ja-JP", {
        dateStyle: "medium",
        timeStyle: "short",
      })
    : "—";
}
function bytes(v: number) {
  if (v == null) return "—";
  const u = ["B", "KB", "MB", "GB", "TB", "PB"];
  let n = v,
    i = 0;
  while (n >= 1024 && i < u.length - 1) {
    n /= 1024;
    i++;
  }
  return `${n.toFixed(i ? 1 : 0)} ${u[i]}`;
}
async function downloadRequestIntakeFile(
  id: string,
  token: string,
  notify: (s: string) => void,
  filename: string,
) {
  try {
    const response = await fetch(
      `/api/v1/public-requests/${encodeURIComponent(id)}/intake-file`,
      { headers: { Authorization: `Bearer ${token}` } },
    );
    if (!response.ok) {
      let detail = "";
      try {
        detail = (await response.json()).error || "";
      } catch {}
      throw new Error(detail || "添付ファイルを取得できませんでした");
    }
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = filename || "intake-file";
    anchor.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    notify("添付ファイルを取得しました");
  } catch (e) {
    notify((e as Error).message);
  }
}
function URLCollector({ api, refresh, notify, role, bumpRefresh }: any) {
  const [searchTerms, setSearchTerms] = useState("");
  const [acceptTerms, setAcceptTerms] = useState(false);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<Obj | null>(null);
  const nav = useNavigate();
  async function scan(e: React.FormEvent) {
    e.preventDefault();
    if (!acceptTerms) return;
    const terms = Array.from(
      new Set(
        searchTerms
          .split(/[\n,\r]+/)
          .map((x: string) => x.trim())
          .filter(Boolean),
      ),
    );
    if (terms.length > 5) {
      notify("検索語は最大5件までです");
      return;
    }
    setBusy(true);
    setResult(null);
    try {
      const searchData = await api("/admin/url-collection/search", {
        method: "POST",
        body: JSON.stringify({
          search_terms: terms,
          accept_terms: acceptTerms,
        }),
      });
      const seeds = (searchData.seed_urls || []) as string[];
      if (!seeds.length) {
        setResult({
          seed_count: 0,
          pages_scanned: 0,
          candidate_count: 0,
          requests_created: 0,
          duplicates_discarded: 0,
          failed: 0,
          items: [],
          search_queries: searchData.queries || [],
          search_results: searchData.results || [],
          search_errors: searchData.errors || [],
          errors: [],
        });
        notify("検索結果から巡回可能なHTTPSページが見つかりませんでした");
        return;
      }
      const data = await api("/admin/url-collection/scan", {
        method: "POST",
        body: JSON.stringify({ seed_urls: seeds, accept_terms: acceptTerms }),
      });
      setResult({
        ...data,
        search_queries: searchData.queries || [],
        search_results: searchData.results || [],
        search_errors: searchData.errors || [],
        search_seed_count: seeds.length,
      });
      bumpRefresh();
      notify(
        `自動収集完了：新規申請 ${data.requests_created} 件、重複破棄 ${data.duplicates_discarded} 件`,
      );
    } catch (e) {
      notify((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  if (role !== "admin")
    return (
      <section className="panel">
        <h2>URL自動収集</h2>
        <p>この機能は管理者のみ利用できます。</p>
      </section>
    );
  const allErrors = [
    ...(result?.search_errors || []),
    ...(result?.errors || []),
  ];
  return (
    <>
      <Heading
        eyebrow="URL DISCOVERY"
        title="URL自動収集"
        desc="DuckDuckGoで公開Webを検索し、検索結果のページを自動巡回して uploader.jp / getuploader.com のリンクを探します。開始URLを手入力する必要はありません。"
        actions={
          <button
            className="secondary"
            onClick={() => nav("/admin/review?tab=requests")}
          >
            <ClipboardList size={16} /> 申請・保全レビューへ
          </button>
        }
      />
      <section className="panel form-panel">
        <div className="panel-head">
          <div>
            <h2>Web検索から自動収集</h2>
            <p>
              検索語を空欄にすると、MMDモデル・モーション・エフェクト・ステージ・配布の5種類を自動検索します。
            </p>
          </div>
          <Search size={20} />
        </div>
        <form className="form-stack" onSubmit={scan}>
          <label>
            検索語（任意、1行に1件・最大5件）
            <textarea
              rows={4}
              value={searchTerms}
              onChange={(e) => setSearchTerms(e.target.value)}
              placeholder={"例：\nMMD モデル\nMMD モーション\n東方 MMD 配布"}
            />
          </label>
          <p className="muted">
            DuckDuckGoの公開HTML検索を利用します（検索APIキーは不要です）。各検索語から最大2ページ、合計最大10ページを巡回元として選びます。巡回は同一サイト内を浅く行い、合計20ページ・候補100件までです。検索結果やサイト側の制限によって見つかる件数は変わります。公開HTTPSページのみを対象とし、ログイン・CAPTCHAを回避せず、プライベートIPや内部ネットワークへの接続も拒否します。
          </p>
          <label className="check-label">
            <input
              type="checkbox"
              checked={acceptTerms}
              onChange={(e) => setAcceptTerms(e.target.checked)}
            />{" "}
            検索結果の各サイトの利用規約・robots.txt等を尊重し、公開情報のみを節度ある頻度で調査します。
          </label>
          <div className="modal-actions">
            <button className="primary" disabled={busy || !acceptTerms}>
              {busy ? "検索・巡回中…" : "DuckDuckGoで検索して自動収集"}
            </button>
          </div>
        </form>
      </section>
      <div className="notice">
        <div className="notice-icon">
          <ShieldCheck size={19} />
        </div>
        <div>
          <b>自動収集は候補登録まで</b>
          <p>
            検出URLは「申請・保全レビュー」に未確認の保全リクエストとして登録されます。検索・巡回だけでファイル本体のダウンロードや一般公開は開始しません。重複URLは既存の収集レジストリ・申請・カタログと照合して破棄します。権利条件を確認し、レビュー画面から取得を開始してください。
          </p>
        </div>
      </div>
      {result && (
        <>
          <div className="stat-grid">
            <Stat
              label="調査ページ"
              value={result.pages_scanned ?? 0}
              sub={`検索結果ページ ${result.search_seed_count ?? 0} 件`}
              icon={Globe}
            />
            <Stat
              label="検出候補"
              value={result.candidate_count ?? 0}
              sub={result.truncated ? "上限に到達" : "候補URL"}
              icon={Search}
              tone="violet"
            />
            <Stat
              label="新規申請"
              value={result.requests_created ?? 0}
              sub="レビュー待ち"
              icon={ClipboardList}
              tone="green"
            />
            <Stat
              label="重複破棄"
              value={result.duplicates_discarded ?? 0}
              sub={`失敗 ${result.failed ?? 0} 件`}
              icon={CheckCircle2}
              tone="amber"
            />
          </div>
          {Array.isArray(result.search_queries) &&
            result.search_queries.length > 0 && (
              <section className="panel">
                <h2>実行した検索</h2>
                <ul>
                  {result.search_queries.map((q: string, i: number) => (
                    <li key={i}>{q}</li>
                  ))}
                </ul>
                {Array.isArray(result.search_results) &&
                  result.search_results.length > 0 && (
                    <>
                      <h3>巡回対象の検索結果</h3>
                      <ul>
                        {result.search_results.map((x: Obj, i: number) => (
                          <li key={x.url || i}>
                            <a href={x.url} target="_blank" rel="noreferrer">
                              {x.title || x.url}
                            </a>
                          </li>
                        ))}
                      </ul>
                    </>
                  )}
              </section>
            )}
          {allErrors.length > 0 && (
            <section className="panel">
              <h2>一部の検索・ページを処理できませんでした</h2>
              <ul>
                {allErrors.map((e: string, i: number) => (
                  <li key={i}>{e}</li>
                ))}
              </ul>
            </section>
          )}
          <section className="panel table-panel">
            <div className="panel-head">
              <div>
                <h2>収集結果</h2>
                <p>重複URLは新しい申請を作らず破棄しました。</p>
              </div>
              <button
                className="secondary"
                onClick={() => nav("/admin/review?tab=requests")}
              >
                申請を確認 →
              </button>
            </div>
            <Table headers={["候補", "URL", "検索結果ページ", "結果"]}>
              {(result.items || []).map((x: Obj, i: number) => (
                <tr key={x.url || i}>
                  <td>{x.label || "候補URL"}</td>
                  <td>
                    <a href={x.url} target="_blank" rel="noreferrer">
                      {x.url}
                    </a>
                  </td>
                  <td>
                    <a href={x.seed_url} target="_blank" rel="noreferrer">
                      巡回元を開く
                    </a>
                  </td>
                  <td>
                    {x.status === "request_created" ? (
                      <span className="badge green">
                        <i />
                        申請登録
                      </span>
                    ) : x.status === "duplicate" ? (
                      <span className="badge amber">
                        <i />
                        重複破棄
                      </span>
                    ) : (
                      <span className="badge red">
                        <i />
                        {x.status === "failed" ? "失敗" : "スキップ"}
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </Table>
          </section>
        </>
      )}
    </>
  );
}

function ReviewWorkspace({
  api,
  refresh,
  notify,
  role,
  token,
  bumpRefresh,
  initialTab = "files",
}: any) {
  const loc = useLocation();
  const nav = useNavigate();
  const queryTab = new URLSearchParams(loc.search).get("tab");
  const tab =
    queryTab === "requests" || (!queryTab && initialTab === "requests")
      ? "requests"
      : "files";
  function choose(next: "files" | "requests") {
    nav(`/admin/review?tab=${next}`);
  }
  return (
    <>
      <div
        className="review-workspace-tabs"
        role="tablist"
        aria-label="保全レビュー"
      >
        <button
          type="button"
          role="tab"
          aria-selected={tab === "files"}
          className={tab === "files" ? "active" : ""}
          onClick={() => choose("files")}
        >
          <Files size={16} /> 作品・ファイル
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "requests"}
          className={tab === "requests" ? "active" : ""}
          onClick={() => choose("requests")}
        >
          <ClipboardList size={16} /> 申請・保全レビュー
        </button>
      </div>
      {tab === "files" ? (
        <Catalog
          api={api}
          refresh={refresh}
          notify={notify}
          role={role}
          token={token}
        />
      ) : (
        <Requests
          api={api}
          refresh={refresh}
          notify={notify}
          role={role}
          bumpRefresh={bumpRefresh}
        />
      )}
    </>
  );
}

function Catalog({ api, refresh, notify, role, token }: any) {
  const [unused, setRefreshDummy] = useState(0);
  const loc = useLocation();
  const nav = useNavigate();
  const params = new URLSearchParams(loc.search);
  const linkedFileID = params.get("file_id") || "";
  const linkedArtifactID = params.get("artifact_id") || "";
  const [query, setQuery] = useState("");
  const [preservationFilter, setPreservationFilter] = useState("");
  const [policyFilter, setPolicyFilter] = useState("");
  const [publicationFilter, setPublicationFilter] = useState("");
  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
  const [bulkPolicy, setBulkPolicy] = useState("restricted");
  const [relatedRequestID, setRelatedRequestID] = useState("");
  const [modal, setModal] = useState(false);
  const [detail, setDetail] = useState<Obj | null>(null);
  const [metadata, setMetadata] = useState<Obj | null>(null);
  const [metaForm, setMetaForm] = useState({
    artifact_title: "",
    creator_name: "",
    artifact_source_url: "",
    artifact_description: "",
    original_filename: "",
    media_type: "",
    source_url: "",
  });
  const [artifact, setArtifact] = useState<Obj | null>(null);
  const [pendingFileId, setPendingFileId] = useState("");
  const [busy, setBusy] = useState(false);
  const [upload, setUpload] = useState<File | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [sourceURL, setSourceURL] = useState("");
  const [acceptSourceTerms, setAcceptSourceTerms] = useState(false);
  const [listingURL, setListingURL] = useState("");
  const [acceptListingTerms, setAcceptListingTerms] = useState(false);
  const [discovering, setDiscovering] = useState(false);
  const [candidates, setCandidates] = useState<Obj[]>([]);
  const [selectedCandidates, setSelectedCandidates] = useState<string[]>([]);
  const [batchBusy, setBatchBusy] = useState(false);
  const [batchStatus, setBatchStatus] = useState("");
  const [priority, setPriority] = useState("3");
  const [policy, setPolicy] = useState("pending");
  const [pub, setPub] = useState(false);
  const [form, setForm] = useState({
    title: "",
    creator_name: "",
    source_url: "",
    description: "",
  });
  let filePath = "/files?limit=100";
  if (query.trim()) filePath += "&q=" + encodeURIComponent(query.trim());
  if (preservationFilter)
    filePath +=
      "&preservation_status=" + encodeURIComponent(preservationFilter);
  if (policyFilter)
    filePath += "&redistribution_policy=" + encodeURIComponent(policyFilter);
  if (publicationFilter)
    filePath += "&publication=" + encodeURIComponent(publicationFilter);
  if (linkedArtifactID)
    filePath += "&artifact_id=" + encodeURIComponent(linkedArtifactID);
  const f = useData(api, filePath, refresh + unused);
  const requestData = useData(
    api,
    "/public-requests?limit=100",
    refresh + unused,
  );
  const files: Obj[] = f.data.items || [];
  const requests: Obj[] = (requestData.data.items || []).filter(
    (x: Obj) =>
      x.status === "accepted" ||
      x.status === "in_review" ||
      x.status === "pending",
  );

  useEffect(() => {
    if (!linkedFileID || f.loading) return;
    const target = (f.data.items || []).find((x: Obj) => x.id === linkedFileID);
    if (target) {
      setDetail(target);
      setPolicy(target.redistribution_policy || "unknown");
      setPub(!!target.public_download_enabled);
      nav("/admin/review?tab=files", { replace: true });
    }
  }, [linkedFileID, f.loading, f.data.items]);

  function openMetadata(x: Obj) {
    setMetadata(x);
    setMetaForm({
      artifact_title: x.artifact_title || "",
      creator_name: x.creator_name || "",
      artifact_source_url: x.artifact_source_url || "",
      artifact_description: x.artifact_description || "",
      original_filename: x.original_filename || "",
      media_type: x.media_type || "",
      source_url: x.source_url || "",
    });
  }
  async function saveMetadata(e: React.FormEvent) {
    e.preventDefault();
    if (!metadata) return;
    const reason = window.prompt("メタデータの変更理由を入力してください");
    if (!reason || reason.trim().length < 3) return;
    setBusy(true);
    try {
      await api(`/artifacts/${metadata.artifact_id}`, {
        method: "PATCH",
        body: JSON.stringify({
          title: metaForm.artifact_title,
          creator_name: metaForm.creator_name,
          source_url: metaForm.artifact_source_url,
          description: metaForm.artifact_description,
          reason: reason.trim(),
        }),
      });
      await api(`/files/${metadata.id}/metadata`, {
        method: "PATCH",
        body: JSON.stringify({
          original_filename: metaForm.original_filename,
          media_type: metaForm.media_type,
          source_url: metaForm.source_url,
          reason: reason.trim(),
        }),
      });
      notify("作品・ファイルのメタデータを更新しました");
      setMetadata(null);
      setRefreshDummy((x) => x + 1);
    } catch (e) {
      notify((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function runBulk(
    action: "policy" | "publication" | "delete",
    extra: Obj = {},
  ) {
    if (selectedIDs.length === 0) {
      notify("先にファイルを選択してください");
      return;
    }
    if (action === "delete" && !relatedRequestID) {
      notify("削除を承認した申請を選択してください");
      return;
    }
    if (action === "delete") {
      const linked = requests.find((r: Obj) => r.id === relatedRequestID);
      if (
        !linked ||
        linked.request_type !== "deletion" ||
        linked.status !== "accepted"
      ) {
        notify("承認済みの削除申請を選択してください");
        return;
      }
    }
    const reason = window.prompt(
      action === "delete"
        ? "削除処理の理由を入力してください"
        : "一括変更の理由を入力してください",
    );
    if (!reason || reason.trim().length < 3) return;
    try {
      const body: Obj = {
        file_ids: selectedIDs,
        action,
        reason: reason.trim(),
        ...extra,
      };
      if (relatedRequestID) body.request_id = relatedRequestID;
      const result = await api("/files/bulk-action", {
        method: "POST",
        body: JSON.stringify(body),
      });
      notify(`${result.count} 件を一括処理しました`);
      setSelectedIDs([]);
      setRefreshDummy((x) => x + 1);
    } catch (e) {
      notify((e as Error).message);
    }
  }
  async function createDeletionRequests() {
    if (selectedIDs.length === 0) return;
    const reason = window.prompt(
      "削除申請の理由を入力してください（5文字以上）",
    );
    if (!reason || reason.trim().length < 5) return;
    if (
      !window.confirm(
        `選択した ${selectedIDs.length} 件について削除申請を作成します。実際の削除は申請を承認した後に始まります。続行しますか？`,
      )
    )
      return;
    try {
      const result = await api("/admin/deletion-requests", {
        method: "POST",
        body: JSON.stringify({ file_ids: selectedIDs, reason: reason.trim() }),
      });
      notify(
        `${result.count} 件の削除申請を作成しました。申請・保全レビューで承認してください。`,
      );
      setSelectedIDs([]);
      nav("/admin/review?tab=requests");
    } catch (e) {
      notify((e as Error).message);
    }
  }
  function toggleSelected(id: string, checked: boolean) {
    setSelectedIDs((old) =>
      checked
        ? old.includes(id)
          ? old
          : [...old, id]
        : old.filter((x) => x !== id),
    );
  }
  function toggleAll(checked: boolean) {
    const selectable = files
      .filter((x: Obj) => !x.deletion_requested_at && !x.deleted_at)
      .map((x: Obj) => x.id);
    setSelectedIDs((old) =>
      checked
        ? Array.from(new Set([...old, ...selectable]))
        : old.filter((id) => !selectable.includes(id)),
    );
  }

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const a = await api("/artifacts", {
        method: "POST",
        body: JSON.stringify(form),
      });
      notify("作品を登録しました");
      setModal(false);
      setForm({ title: "", creator_name: "", source_url: "", description: "" });
      setArtifact(a);
      setPendingFileId("");
      setRefreshDummy((x) => x + 1);
    } catch (e) {
      notify((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function addFile(e: React.FormEvent) {
    e.preventDefault();
    const url = sourceURL.trim();
    if (
      !artifact ||
      (!upload && !url) ||
      (upload && url) ||
      (url && !acceptSourceTerms) ||
      policy === "pending"
    )
      return;
    setBusy(true);
    try {
      let fileId = pendingFileId;
      if (!fileId) {
        const file = await api(`/artifacts/${artifact.id}/files`, {
          method: "POST",
          body: JSON.stringify({
            original_filename: upload?.name || "取得待ち",
            size_bytes: upload?.size || 0,
            media_type: upload?.type || "application/octet-stream",
            source_url: url || undefined,
            redistribution_policy: policy,
            public_download_enabled: false,
          }),
        });
        fileId = file.id;
        setPendingFileId(fileId);
      }
      let ingestResult: any;
      if (upload) {
        const fd = new FormData();
        fd.append("file", upload);
        fd.append("priority", priority);
        ingestResult = await api(`/files/${fileId}/ingest`, {
          method: "POST",
          body: fd,
        });
      } else {
        ingestResult = await api(`/files/${fileId}/ingest-source`, {
          method: "POST",
          body: JSON.stringify({
            source_url: url,
            accept_terms: acceptSourceTerms,
            priority: Number(priority),
          }),
        });
      }
      const createdTasks = Array.isArray(ingestResult?.tasks)
        ? ingestResult.tasks.length
        : 0;
      notify(
        ingestResult?.source_fetch_task
          ? `取得タスクを ${ingestResult.source_fetch_task.node_name || "agent"} に登録しました。取得成功後に保存タスクが作成されます。`
          : createdTasks
            ? `登録しました。保存タスク ${createdTasks} 件を作成しました。`
            : "登録しました。保全処理を受け付けました。",
      );
      setArtifact(null);
      setPendingFileId("");
      setUpload(null);
      setSourceURL("");
      setAcceptSourceTerms(false);
      setListingURL("");
      setAcceptListingTerms(false);
      setCandidates([]);
      setSelectedCandidates([]);
      setBatchStatus("");
      setRefreshDummy((x) => x + 1);
    } catch (e) {
      notify((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function discoverListing() {
    if (!artifact || !listingURL.trim() || !acceptListingTerms) return;
    setDiscovering(true);
    setCandidates([]);
    setSelectedCandidates([]);
    setBatchStatus("一覧を確認しています…");
    try {
      const result = await api(
        `/artifacts/${artifact.id}/discover-source-files`,
        {
          method: "POST",
          body: JSON.stringify({
            listing_url: listingURL.trim(),
            accept_terms: acceptListingTerms,
          }),
        },
      );
      const found = (result.items || []) as Obj[];
      setCandidates(found);
      setSelectedCandidates(found.map((x: Obj) => x.url));
      setBatchStatus(
        `${result.pages_scanned} ページから ${found.length} 件を検出しました。` +
          (result.truncated
            ? "上限100件に達したため、残りは次回確認してください。"
            : ""),
      );
    } catch (e) {
      setBatchStatus((e as Error).message);
    } finally {
      setDiscovering(false);
    }
  }
  async function importSelected() {
    if (!artifact || selectedCandidates.length === 0 || policy === "pending")
      return;
    const chosen = candidates.filter((x: Obj) =>
      selectedCandidates.includes(x.url),
    );
    setBatchBusy(true);
    let ok = 0;
    let fetchTaskCount = 0;
    let taskCount = 0;
    const failures: string[] = [];
    for (let i = 0; i < chosen.length; i++) {
      const item = chosen[i];
      setBatchStatus(
        `取得タスク登録中 ${i + 1}/${chosen.length}: ${item.label || item.url}`,
      );
      try {
        const file = await api(`/artifacts/${artifact.id}/files`, {
          method: "POST",
          body: JSON.stringify({
            original_filename: (item.label || "取得待ち").slice(0, 500),
            size_bytes: 0,
            media_type: "application/octet-stream",
            source_url: item.url,
            redistribution_policy: policy,
          }),
        });
        const queued = await api(`/files/${file.id}/ingest-source`, {
          method: "POST",
          body: JSON.stringify({
            source_url: item.url,
            accept_terms: true,
            priority: Number(priority),
          }),
        });
        ok++;
        if (queued.source_fetch_task) fetchTaskCount++;
        taskCount += Array.isArray(queued?.tasks) ? queued.tasks.length : 0;
      } catch (e) {
        failures.push(`${item.label || item.url}: ${(e as Error).message}`);
      }
    }
    setBatchBusy(false);
    setSelectedCandidates([]);
    setCandidates([]);
    setBatchStatus(
      `完了: ${ok} 件受付、agent取得タスク ${fetchTaskCount} 件登録、保存タスク ${taskCount} 件作成、${failures.length} 件失敗。` +
        (fetchTaskCount
          ? "保存タスクは各 agent の取得成功後に作成されます。 "
          : "") +
        (failures.length ? `最初の失敗: ${failures[0]}` : ""),
    );
    notify(
      `一括保全: ${ok} 件受付、agent取得タスク ${fetchTaskCount} 件登録、失敗 ${failures.length} 件`,
    );
    setRefreshDummy((x) => x + 1);
  }
  async function savePolicy() {
    if (!detail) return;
    const reason = window.prompt("変更理由を入力してください");
    if (!reason || reason.trim().length < 3) return;
    try {
      await api(`/files/${detail.id}/policy`, {
        method: "PATCH",
        body: JSON.stringify({
          redistribution_policy: policy,
          reason: reason.trim(),
        }),
      });
      await api(`/files/${detail.id}/publication`, {
        method: "PATCH",
        body: JSON.stringify({ enabled: pub, reason: reason.trim() }),
      });
      notify("公開ポリシーを更新しました");
      setDetail(null);
      setRefreshDummy((x) => x + 1);
    } catch (e) {
      notify((e as Error).message);
    }
  }

  return (
    <>
      <Heading
        eyebrow=""
        title="作品・ファイル"
        desc="メタデータ、保全状態、公開条件を検索・一括管理"
        actions={
          <>
            <button
              className="secondary"
              onClick={() => downloadExport("csv", token, notify)}
            >
              <Download size={16} /> CSV出力
            </button>
            {role === "admin" && (
              <button className="primary" onClick={() => setModal(true)}>
                <Plus size={16} /> 作品を登録
              </button>
            )}
          </>
        }
      />
      <div className="filter-bar file-filter-bar">
        <div className="search-box">
          <Search size={17} />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="作品名・ファイル名・作者・IDで検索"
          />
        </div>
        <select
          aria-label="保全状態で絞り込み"
          value={preservationFilter}
          onChange={(e) => setPreservationFilter(e.target.value)}
        >
          <option value="">すべての保全状態</option>
          <option value="uncollected">未取得</option>
          <option value="queued">待機中</option>
          <option value="stored">保存済み</option>
          <option value="verified">検証済み</option>
          <option value="degraded">劣化</option>
          <option value="missing">実体不明</option>
          <option value="error">エラー</option>
        </select>
        <select
          aria-label="公開条件で絞り込み"
          value={policyFilter}
          onChange={(e) => setPolicyFilter(e.target.value)}
        >
          <option value="">すべての公開条件</option>
          <option value="allowed">再配布可</option>
          <option value="restricted">制限付き</option>
          <option value="in_progress">確認中</option>
          <option value="pending">未確認</option>
          <option value="unknown">不明</option>
        </select>
        <select
          aria-label="公開状態で絞り込み"
          value={publicationFilter}
          onChange={(e) => setPublicationFilter(e.target.value)}
        >
          <option value="">公開状態：すべて</option>
          <option value="public">公開中</option>
          <option value="private">非公開</option>
          <option value="restricted">制限付き</option>
          <option value="pending">未確認</option>
          <option value="unknown">条件不明</option>
        </select>
        <span className="result-count">{f.data.total ?? files.length} 件</span>
      </div>
      {selectedIDs.length > 0 && role !== "viewer" && (
        <section className="bulk-toolbar panel">
          <div className="bulk-toolbar-title">
            <b>{selectedIDs.length} 件を選択中</b>
            <button className="small-btn" onClick={() => setSelectedIDs([])}>
              選択解除
            </button>
          </div>
          <div className="bulk-toolbar-controls">
            <label>
              関連申請
              <select
                value={relatedRequestID}
                onChange={(e) => setRelatedRequestID(e.target.value)}
              >
                <option value="">申請に紐づけない</option>
                {requests.map((r: Obj) => (
                  <option key={r.id} value={r.id}>
                    {r.request_type === "deletion"
                      ? "削除"
                      : r.request_type === "takedown"
                        ? "公開停止"
                        : r.request_type === "preservation"
                          ? "保全"
                          : r.request_type === "redistribution_assistance"
                            ? "再配布相談"
                            : r.request_type === "publisher_assistance"
                              ? "配布代行"
                              : "移設先"}{" "}
                    /{" "}
                    {r.linked_file_name ||
                      r.linked_artifact_title ||
                      r.title ||
                      String(r.id).slice(0, 8)}{" "}
                    /{" "}
                    {r.status === "accepted"
                      ? "承認"
                      : r.status === "in_review"
                        ? "確認中"
                        : "未処理"}
                  </option>
                ))}
              </select>
            </label>
            <label>
              公開条件
              <select
                value={bulkPolicy}
                onChange={(e) => setBulkPolicy(e.target.value)}
              >
                <option value="allowed">再配布可</option>
                <option value="restricted">制限付き</option>
                <option value="in_progress">確認中</option>
                <option value="pending">未確認</option>
                <option value="unknown">不明</option>
              </select>
            </label>
            <button
              className="secondary"
              onClick={() =>
                runBulk("policy", { redistribution_policy: bulkPolicy })
              }
            >
              公開条件を一括変更
            </button>
            {role === "admin" && (
              <>
                <button
                  className="primary"
                  onClick={() => runBulk("publication", { enabled: true })}
                >
                  一括公開
                </button>
                <button
                  className="secondary"
                  onClick={() => runBulk("publication", { enabled: false })}
                >
                  一括公開停止
                </button>
                <button className="danger-btn" onClick={createDeletionRequests}>
                  削除申請を作成
                </button>
                <button
                  className="danger-btn"
                  onClick={() => runBulk("delete")}
                >
                  承認済み申請で削除
                </button>
              </>
            )}
          </div>
          <p className="muted">
            「削除申請を作成」は申請を未処理で登録します。申請・保全レビューで管理者が承認するとノード削除タスクが開始します。承認済みの申請を使った削除も可能です。
          </p>
        </section>
      )}
      <section className="panel table-panel">
        <Table
          headers={[
            "",
            "作品 / ファイル",
            "保全状態",
            "公開条件 / 状態",
            "サイズ",
            "登録日時",
            "操作",
          ]}
        >
          {f.loading ? (
            <tr>
              <td colSpan={7} className="empty">
                読み込み中…
              </td>
            </tr>
          ) : f.error ? (
            <tr>
              <td colSpan={7} className="empty">
                {f.error}
              </td>
            </tr>
          ) : files.length ? (
            <>
              <tr className="select-all-row">
                <td>
                  <input
                    type="checkbox"
                    aria-label="表示中のファイルをすべて選択"
                    checked={
                      files.filter(
                        (x: Obj) => !x.deletion_requested_at && !x.deleted_at,
                      ).length > 0 &&
                      files
                        .filter(
                          (x: Obj) => !x.deletion_requested_at && !x.deleted_at,
                        )
                        .every((x: Obj) => selectedIDs.includes(x.id))
                    }
                    disabled={role === "viewer"}
                    onChange={(e) => toggleAll(e.target.checked)}
                  />
                </td>
                <td colSpan={6} className="muted">
                  表示中の {files.length} 件を選択
                </td>
              </tr>
              {files.map((x: Obj) => (
                <FileRow
                  key={x.id}
                  x={x}
                  role={role}
                  isSelected={selectedIDs.includes(x.id)}
                  toggleSelected={toggleSelected}
                  setDetail={setDetail}
                  setPolicy={setPolicy}
                  setPub={setPub}
                  openMetadata={openMetadata}
                  token={token}
                />
              ))}
            </>
          ) : (
            <tr>
              <td colSpan={7} className="empty">
                ファイルがありません
              </td>
            </tr>
          )}
        </Table>
      </section>
      {modal && (
        <Modal title="作品を登録" close={() => setModal(false)}>
          <form onSubmit={create} className="form-stack">
            <label>
              作品名 *
              <input
                required
                value={form.title}
                onChange={(e) => setForm({ ...form, title: e.target.value })}
              />
            </label>
            <label>
              作者・配布者
              <input
                value={form.creator_name}
                onChange={(e) =>
                  setForm({ ...form, creator_name: e.target.value })
                }
              />
            </label>
            <label>
              元ページURL *
              <input
                required
                type="url"
                placeholder="https://www.uploader.jp/..."
                value={form.source_url}
                onChange={(e) =>
                  setForm({ ...form, source_url: e.target.value })
                }
              />
            </label>
            <label>
              説明
              <textarea
                rows={3}
                value={form.description}
                onChange={(e) =>
                  setForm({ ...form, description: e.target.value })
                }
              />
            </label>
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setModal(false)}
              >
                キャンセル
              </button>
              <button className="primary" disabled={busy}>
                {busy ? "登録中…" : "次へ"}
              </button>
            </div>
          </form>
        </Modal>
      )}
      {artifact && (
        <Modal
          title="ファイルを追加・保全"
          close={() => {
            setArtifact(null);
            setPendingFileId("");
            setListingURL("");
            setAcceptListingTerms(false);
            setCandidates([]);
            setSelectedCandidates([]);
            setBatchStatus("");
          }}
        >
          <form className="form-stack" onSubmit={addFile}>
            <p className="muted">
              作品: <b>{artifact.title}</b>
            </p>
            <label>
              手元のファイル
              <input
                ref={fileInputRef}
                type="file"
                onChange={(e) => {
                  setUpload(e.target.files?.[0] || null);
                  if (e.target.files?.[0]) {
                    setSourceURL("");
                    setAcceptSourceTerms(false);
                    setListingURL("");
                    setAcceptListingTerms(false);
                    setCandidates([]);
                    setSelectedCandidates([]);
                    setPendingFileId("");
                  }
                }}
              />
              <small>最大5 GiB。保存先は2ノード必要です。</small>
            </label>
            <label>
              または uploader.jp のダウンロードURL
              <input
                type="url"
                value={sourceURL}
                onChange={(e) => {
                  setSourceURL(e.target.value);
                  if (e.target.value.trim()) {
                    setUpload(null);
                    setListingURL("");
                    setAcceptListingTerms(false);
                    setCandidates([]);
                    setSelectedCandidates([]);
                    if (fileInputRef.current) fileInputRef.current.value = "";
                  } else setPendingFileId("");
                }}
                placeholder="https://uu.getuploader.com/名前/download/123"
              />
              <small>
                個別ファイル用。パスワードや画像認証の回避は行いません。
              </small>
            </label>
            {sourceURL.trim() && (
              <label className="check-label">
                <input
                  type="checkbox"
                  checked={acceptSourceTerms}
                  onChange={(e) => setAcceptSourceTerms(e.target.checked)}
                />{" "}
                uploader.jp の利用規約を確認し、同意しました
              </label>
            )}
            <div className="form-divider" />
            <label>
              またはアップローダーの一覧URL
              <input
                type="url"
                value={listingURL}
                onChange={(e) => {
                  setListingURL(e.target.value);
                  if (e.target.value.trim()) {
                    setSourceURL("");
                    setAcceptSourceTerms(false);
                    setUpload(null);
                    setPendingFileId("");
                    if (fileInputRef.current) fileInputRef.current.value = "";
                  }
                  setCandidates([]);
                  setSelectedCandidates([]);
                  setBatchStatus("");
                }}
                placeholder="https://uu.getuploader.com/名前/"
              />
              <small>
                一覧ページを調べ、掲載ファイルを選んで順番に取得します（最大100件・最大10ページ）。
              </small>
            </label>
            {listingURL.trim() && (
              <label className="check-label">
                <input
                  type="checkbox"
                  checked={acceptListingTerms}
                  onChange={(e) => setAcceptListingTerms(e.target.checked)}
                />{" "}
                uploader.jp の利用規約を確認しました
              </label>
            )}
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                disabled={
                  !listingURL.trim() ||
                  !acceptListingTerms ||
                  discovering ||
                  batchBusy
                }
                onClick={discoverListing}
              >
                {discovering ? "確認中…" : "一覧を調べる"}
              </button>
            </div>
            {batchStatus && <p className="muted">{batchStatus}</p>}
            {candidates.length > 0 && (
              <div className="candidate-list">
                <div className="candidate-list-head">
                  <b>検出したファイル ({candidates.length})</b>
                  <button
                    type="button"
                    className="small-btn"
                    onClick={() =>
                      setSelectedCandidates(
                        selectedCandidates.length === candidates.length
                          ? []
                          : candidates.map((x: Obj) => x.url),
                      )
                    }
                  >
                    {selectedCandidates.length === candidates.length
                      ? "選択解除"
                      : "全件選択"}
                  </button>
                </div>
                {candidates.map((x: Obj) => (
                  <label className="candidate-row" key={x.url}>
                    <input
                      type="checkbox"
                      checked={selectedCandidates.includes(x.url)}
                      onChange={(e) =>
                        setSelectedCandidates(
                          e.target.checked
                            ? [...selectedCandidates, x.url]
                            : selectedCandidates.filter((u) => u !== x.url),
                        )
                      }
                    />
                    <span>
                      {x.label || x.url}
                      <small>{x.url}</small>
                    </span>
                  </label>
                ))}
                <button
                  type="button"
                  className="primary wide"
                  disabled={
                    batchBusy ||
                    selectedCandidates.length === 0 ||
                    policy === "pending"
                  }
                  onClick={importSelected}
                >
                  {batchBusy
                    ? "取得タスク登録中…"
                    : `選択した ${selectedCandidates.length} 件の取得タスクを登録`}
                </button>
              </div>
            )}
            <label>
              公開条件
              <select
                value={policy}
                disabled={busy || batchBusy}
                onChange={(e) => setPolicy(e.target.value)}
              >
                <option value="pending">未確認（保全不可）</option>
                <option value="unknown">不明</option>
                <option value="in_progress">確認中</option>
                <option value="allowed">許可済み</option>
                <option value="restricted">制限あり</option>
              </select>
            </label>
            <p className="muted">
              保全と一般公開は別の判定です。公開条件が未確認のファイルは保全を開始できません。
            </p>
            <label>
              保存優先度（0〜5）
              <select
                value={priority}
                onChange={(e) => setPriority(e.target.value)}
              >
                {[5, 4, 3, 2, 1, 0].map((x) => (
                  <option key={x} value={x}>
                    {x}
                  </option>
                ))}
              </select>
            </label>
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => {
                  setArtifact(null);
                  setPendingFileId("");
                  setListingURL("");
                  setAcceptListingTerms(false);
                  setCandidates([]);
                  setSelectedCandidates([]);
                  setBatchStatus("");
                }}
              >
                キャンセル
              </button>
              <button
                className="primary"
                disabled={
                  busy ||
                  (!upload && !sourceURL.trim()) ||
                  (!!upload && !!sourceURL.trim()) ||
                  (!!sourceURL.trim() && !acceptSourceTerms) ||
                  policy === "pending"
                }
              >
                <CloudUpload size={16} />
                {busy
                  ? "処理中…"
                  : sourceURL.trim()
                    ? "取得して保全開始"
                    : "登録して保全開始"}
              </button>
            </div>
          </form>
        </Modal>
      )}
      {detail && (
        <Modal title="公開設定" close={() => setDetail(null)}>
          <div className="detail-summary">
            <b>{detail.artifact_title || detail.original_filename}</b>
            <small>{detail.original_filename}</small>
            <small>{detail.id}</small>
            {detail.source_url && (
              <small>
                <a href={detail.source_url} target="_blank" rel="noreferrer">
                  取得元URLを開く
                </a>
              </small>
            )}
            {detail.deleted_at && (
              <div className="policy-warning">
                <LockKeyhole size={16} /> 削除済みのファイルです。
              </div>
            )}
            {detail.deletion_requested_at && !detail.deleted_at && (
              <div className="policy-warning">
                <LockKeyhole size={16} /> ノードへの削除タスクを処理中です。
              </div>
            )}
            <div className="policy-warning">
              <LockKeyhole size={16} /> 公開は権利確認後に有効にしてください。
            </div>
          </div>
          <div className="form-stack">
            <label>
              公開条件
              <select
                value={policy}
                onChange={(e) => setPolicy(e.target.value)}
              >
                <option value="unknown">不明</option>
                <option value="pending">未確認（保全不可）</option>
                <option value="in_progress">確認中</option>
                <option value="allowed">許可済み</option>
                <option value="restricted">制限あり</option>
              </select>
            </label>
            <label className="check-label">
              <input
                type="checkbox"
                checked={pub}
                onChange={(e) => setPub(e.target.checked)}
              />{" "}
              一般公開する
            </label>
            <p className="muted">
              公開には再配布可の確認、検証済み状態、オンライン保存先が必要です。
            </p>
            <div className="modal-actions">
              <button className="secondary" onClick={() => setDetail(null)}>
                閉じる
              </button>
              <button
                className="primary"
                onClick={savePolicy}
                disabled={!!detail.deleted_at || !!detail.deletion_requested_at}
              >
                設定を保存
              </button>
            </div>
          </div>
        </Modal>
      )}
      {metadata && (
        <Modal
          title="作品・ファイルのメタデータ"
          close={() => setMetadata(null)}
        >
          <form className="form-stack" onSubmit={saveMetadata}>
            <label>
              作品名 *
              <input
                required
                value={metaForm.artifact_title}
                onChange={(e) =>
                  setMetaForm({ ...metaForm, artifact_title: e.target.value })
                }
              />
            </label>
            <label>
              作者・配布者
              <input
                value={metaForm.creator_name}
                onChange={(e) =>
                  setMetaForm({ ...metaForm, creator_name: e.target.value })
                }
              />
            </label>
            <label>
              作品の元ページURL *
              <input
                required
                type="url"
                value={metaForm.artifact_source_url}
                onChange={(e) =>
                  setMetaForm({
                    ...metaForm,
                    artifact_source_url: e.target.value,
                  })
                }
              />
            </label>
            <label>
              作品の説明
              <textarea
                rows={3}
                value={metaForm.artifact_description}
                onChange={(e) =>
                  setMetaForm({
                    ...metaForm,
                    artifact_description: e.target.value,
                  })
                }
              />
            </label>
            <div className="form-divider" />
            <label>
              ファイル名 *
              <input
                required
                value={metaForm.original_filename}
                onChange={(e) =>
                  setMetaForm({
                    ...metaForm,
                    original_filename: e.target.value,
                  })
                }
              />
            </label>
            <label>
              MIME タイプ
              <input
                value={metaForm.media_type}
                onChange={(e) =>
                  setMetaForm({ ...metaForm, media_type: e.target.value })
                }
                placeholder="application/zip"
              />
            </label>
            <label>
              ファイル取得元URL
              <input
                type="url"
                value={metaForm.source_url}
                onChange={(e) =>
                  setMetaForm({ ...metaForm, source_url: e.target.value })
                }
              />
            </label>
            <p className="muted">
              作者・作品名は作品レコードを更新するため、同じ作品に属するファイルの表示にも反映されます。
            </p>
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setMetadata(null)}
              >
                キャンセル
              </button>
              <button className="primary" disabled={busy}>
                {busy ? "保存中…" : "メタデータを保存"}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </>
  );
}
function Modal({
  title,
  close,
  children,
}: {
  title: string;
  close: () => void;
  children: React.ReactNode;
}) {
  return (
    <div
      className="modal-backdrop"
      onMouseDown={(e) => e.target === e.currentTarget && close()}
    >
      <div className="modal">
        <div className="modal-head">
          <h2>{title}</h2>
          <button className="icon-btn" onClick={close}>
            <X />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}
function Nodes({ api, refresh, notify, role }: any) {
  const [localRefresh, setLocalRefresh] = useState(0);
  const d = useData(api, "/nodes", refresh + localRefresh);
  const [deletingRows, setDeletingRows] = useState<string[]>([]);
  const deleteNode = async (id: string) => {
    setDeletingRows((rows) => [...rows, id]);
    try {
      await api(`/nodes/${encodeURIComponent(id)}`, {
        method: "DELETE",
      });
    } catch (e) {
      notify(e instanceof Error ? e.message : "エラーが発生しました");
    } finally {
      setDeletingRows([...deletingRows.filter((row) => row !== id)]);
      setLocalRefresh((x) => x + 1);
    }
  };
  return (
    <>
      <Heading
        eyebrow=""
        title="保存ノード"
        desc="稼働状況と容量"
        actions={
          <button
            className="secondary"
            onClick={() =>
              api("/admin/placements/reconcile", { method: "POST" })
                .then(() => notify("再調整を実行しました"))
                .catch((e: Error) => notify(e.message))
            }
          >
            <RefreshCw size={16} /> 保存先を調整
          </button>
        }
      />
      <div className="stat-grid">
        <Stat
          label="登録ノード"
          value={(d.data.items || []).length}
          sub="参加ノード数"
          icon={HardDrive}
        />
        <Stat
          label="オンライン"
          value={
            (d.data.items || []).filter((x: Obj) => x.status === "online")
              .length
          }
          sub="稼働状況"
          icon={CheckCircle2}
          tone="green"
        />
        <Stat
          label="申告容量"
          value={bytes(
            (d.data.items || []).reduce(
              (s: number, x: Obj) =>
                s +
                (x.requested_capacity_bytes === 0
                  ? x.capacity_bytes || 0
                  : x.requested_capacity_bytes || 0),
              0,
            ),
          )}
          sub="参加者の設定値の合計"
          icon={Archive}
          tone="violet"
        />
        <Stat
          label="要確認"
          value={
            (d.data.items || []).filter((x: Obj) => x.status !== "online")
              .length
          }
          sub="停止・劣化ノード"
          icon={AlertTriangle}
          tone="amber"
        />
      </div>
      <section className="panel table-panel">
        <div className="panel-head">
          <div>
            <h2>ノード一覧</h2>
            <p>実容量と申告上限を表示</p>
          </div>
        </div>
        <Table
          headers={[
            "ノード",
            "種別",
            "状態",
            "連絡先",
            "実容量 / 空き",
            "申告上限",
            "使用量",
            "最終応答",
            "エージェント",
            "操作",
          ]}
        >
          {d.loading ? (
            <tr>
              <td colSpan={7} className="empty">
                読み込み中…
              </td>
            </tr>
          ) : d.error ? (
            <tr>
              <td colSpan={7} className="empty">
                {d.error}
              </td>
            </tr>
          ) : (
            (d.data.items || []).map((x: Obj) => (
              <tr key={x.id}>
                <td>
                  <div className="file-cell">
                    <div className="file-icon">
                      <HardDrive size={17} />
                    </div>
                    <div>
                      <b>{x.name || x.id}</b>
                      <small>
                        <code>{String(x.id).slice(0, 8)}</code>
                      </small>
                    </div>
                  </div>
                </td>
                <td>
                  <span className="type-pill">{x.node_type || "—"}</span>
                </td>
                <td>
                  <Status value={x.status} />
                </td>
                <td>
                  {x.contact_email ? (
                    <>
                      <a href={`mailto:${x.contact_email}`}>{x.contact_email}</a>{" "}
                      <CopyEmail email={x.contact_email} />
                    </>
                  ) : (
                    "—"
                  )}
                </td>
                <td>
                  <b>
                    {bytes(
                      x.total_bytes ??
                        x.actual_capacity_bytes ??
                        x.capacity_bytes,
                    )}
                  </b>
                  <small className="cell-sub">
                    空き {bytes(x.available_bytes ?? x.free_bytes)}
                  </small>
                </td>
                <td>
                  {x.requested_capacity_bytes === 0
                    ? "無制限"
                    : bytes(x.requested_capacity_bytes ?? x.capacity_bytes)}
                </td>
                <td>
                  {x.used_bytes
                    ? bytes(x.used_bytes) +
                      `(${Math.trunc(
                        x.requested_capacity_bytes === 0
                          ? (x.used_bytes /
                              (x.total_bytes ??
                                x.actual_capacity_bytes ??
                                x.capacity_bytes)) *
                              100
                          : (x.used_bytes /
                              (x.requested_capacity_bytes ??
                                x.capacity_bytes)) *
                              100,
                      )}%)`
                    : "不明"}
                </td>
                <td>{date(x.last_seen_at)}</td>
                <td>{x.agent_version || "—"}</td>
                <td>
                  <div className="row-actions">
                    <button
                      className="small-btn"
                      disabled={
                        role === "viewer" || deletingRows.includes(x.id)
                      }
                      onClick={() => {
                        deleteNode(x.id);
                      }}
                    >
                      {deletingRows.includes(x.id) ? "処理中..." : "削除"}
                    </button>
                  </div>
                </td>
              </tr>
            ))
          )}
        </Table>
      </section>
      {role === "admin" && (
        <NodeEnrollmentRequests
          api={api}
          refresh={refresh + localRefresh}
          notify={notify}
        />
      )}
      <div className="info-card">
        <AlertTriangle size={18} />
        <div>
          <b>運用上の注意</b>
          <p>
            開発用ノードは同じホスト上です。本番では別ホストに配置してください。参加申請を承認すると72時間有効・一回限りの登録トークンが発行されます。
          </p>
        </div>
      </div>
    </>
  );
}
function NodeEnrollmentRequests({ api, refresh, notify }: any) {
  const [localRefresh, setLocalRefresh] = useState(0);
  const d = useData(
    api,
    "/admin/node-enrollment-requests",
    refresh + localRefresh,
  );
  const [busyID, setBusyID] = useState("");
  const [issued, setIssued] = useState<Obj | null>(null);
  const [conversationTarget, setConversationTarget] = useState<Obj | null>(
    null,
  );
  async function review(item: Obj, status: "approved" | "rejected") {
    if (
      status === "rejected" &&
      !window.confirm(`「${item.name}」の参加申請を却下しますか？`)
    )
      return;
    if (
      status === "approved" &&
      !window.confirm(
        `「${item.name}」を承認し、一回限りの登録トークンを発行しますか？トークンはこの画面で一度だけ表示されます。`,
      )
    )
      return;
    setBusyID(item.id);
    try {
      const result = await api(`/admin/node-enrollment-requests/${item.id}`, {
        method: "PATCH",
        body: JSON.stringify({ status }),
      });
      if (status === "approved")
        setIssued({
          name: item.name,
          token: result.enrollment_token,
          expires_at: result.expires_at,
        });
      else notify("参加申請を却下しました");
      setLocalRefresh((x) => x + 1);
    } catch (e) {
      notify((e as Error).message);
    } finally {
      setBusyID("");
    }
  }
  const items = d.data.items || [];
  return (
    <>
      <section className="panel table-panel">
        <div className="panel-head">
          <div>
            <h2>ストレージノード参加申請</h2>
            <p>申請を確認し、承認時に申請ごとの一回限りトークンを発行</p>
          </div>
          <button
            className="secondary"
            onClick={() => setLocalRefresh((x) => x + 1)}
          >
            <RefreshCw size={14} /> 更新
          </button>
        </div>
        <Table
          headers={[
            "申請日時",
            "ノード",
            "種別",
            "申告容量",
            "連絡先",
            "補足",
            "状態",
            "操作",
          ]}
        >
          {d.loading ? (
            <tr>
              <td colSpan={8} className="empty">
                読み込み中…
              </td>
            </tr>
          ) : d.error ? (
            <tr>
              <td colSpan={8} className="empty">
                {d.error}
              </td>
            </tr>
          ) : items.length ? (
            items.map((x: Obj) => (
              <tr key={x.id}>
                <td>{date(x.created_at)}</td>
                <td>
                  <b>{x.name}</b>
                  <small className="cell-sub">
                    <code>{String(x.id).slice(0, 8)}</code>
                  </small>
                </td>
                <td>{x.node_type}</td>
                <td>
                  {x.requested_capacity_bytes === 0
                    ? "上限なし"
                    : bytes(x.requested_capacity_bytes)}
                </td>
                <td>
                  <a href={`mailto:${x.contact_email}`}>{x.contact_email}</a>{" "}
                  <CopyEmail email={String(x.contact_email || "")} />
                </td>
                <td className="error-cell" title={x.message || ""}>
                  {x.message || "—"}
                </td>
                <td>
                  <Status value={x.status} />
                  {x.status === "approved" && x.token_expires_at && (
                    <small className="cell-sub">
                      期限 {date(x.token_expires_at)}
                    </small>
                  )}
                </td>
                <td>
                  <div className="row-actions">
                    {(x.status === "pending" || x.status === "approved") && (
                      <>
                        <button
                          className="small-btn"
                          disabled={!!busyID}
                          onClick={() => review(x, "approved")}
                        >
                          {x.status === "approved" ? "再発行" : "承認・発行"}
                        </button>
                        <button
                          className="small-btn"
                          disabled={!!busyID}
                          onClick={() => review(x, "rejected")}
                        >
                          却下
                        </button>
                      </>
                    )}
                    {x.status === "enrolled" && <small>登録済み</small>}
                    <button
                      className="small-btn"
                      onClick={() => setConversationTarget(x)}
                    >
                      連絡・会話URL
                    </button>
                  </div>
                </td>
              </tr>
            ))
          ) : (
            <tr>
              <td colSpan={8} className="empty">
                参加申請はありません
              </td>
            </tr>
          )}
        </Table>
      </section>
      {conversationTarget && (
        <Modal
          title={`連絡・会話URL：${conversationTarget.name}`}
          close={() => setConversationTarget(null)}
        >
          <ConversationLinks
            api={api}
            refresh={refresh}
            notify={notify}
            targetType="node_enrollment_request"
            targetId={conversationTarget.id}
            role="admin"
          />
        </Modal>
      )}
      {issued && (
        <Modal title="発行した登録トークン" close={() => setIssued(null)}>
          <p>{issued.name} 向け。参加者へ安全な方法で共有してください。</p>
          <div className="issued-token-body">
            <p className="policy-warning">
              <LockKeyhole size={16} />{" "}
              このトークンは再表示できません。画面を閉じる前にコピーしてください。
            </p>
            <code className="issued-token-value">{issued.token}</code>
            <div className="modal-actions">
              <button
                className="secondary"
                onClick={async () => {
                  try {
                    if (!navigator.clipboard)
                      throw new Error("clipboard unavailable");
                    await navigator.clipboard.writeText(String(issued.token));
                    notify("トークンをコピーしました");
                  } catch {
                    notify(
                      "コピーできませんでした。トークンを手動でコピーしてください。",
                    );
                  }
                }}
              >
                トークンをコピー
              </button>
              <button className="primary" onClick={() => setIssued(null)}>
                共有済み・閉じる
              </button>
            </div>
            <p className="muted">
              有効期限：{date(issued.expires_at)}
              <br />
              初回登録後は再利用できません。
            </p>
          </div>
        </Modal>
      )}
    </>
  );
}
function Tasks({ api, refresh, notify, role }: any) {
  const d = useData(api, "/tasks", refresh);
  const [filter, setFilter] = useState("all");
  const items = (d.data.items || []).filter(
    (x: Obj) => filter === "all" || x.status === filter,
  );
  async function retry(id: string) {
    try {
      await api(`/tasks/${id}/retry`, { method: "POST" });
      notify("再試行を開始しました");
    } catch (e) {
      notify((e as Error).message);
    }
  }
  return (
    <>
      <Heading
        eyebrow=""
        title="タスク"
        desc="処理状況と失敗理由"
        actions={
          <button className="secondary" onClick={() => location.reload()}>
            <RefreshCw size={16} /> 更新
          </button>
        }
      />
      <div className="filter-bar">
        <div className="filter-tabs">
          {[
            ["all", "すべて"],
            ["queued", "待機中"],
            ["running", "実行中"],
            ["failed", "失敗"],
            ["completed", "成功"],
          ].map(([v, l]) => (
            <button
              key={v}
              className={filter === v ? "active" : ""}
              onClick={() => setFilter(v)}
            >
              {l}
            </button>
          ))}
        </div>
        <span className="result-count">{items.length} 件</span>
      </div>
      <section className="panel table-panel">
        <Table
          headers={[
            "タスクID",
            "タスク種別",
            "状態",
            "優先度",
            "試行回数",
            "エラー / 結果",
            "ノード",
            "更新日時",
            "操作",
          ]}
        >
          {d.loading ? (
            <tr>
              <td colSpan={8} className="empty">
                読み込み中…
              </td>
            </tr>
          ) : d.error ? (
            <tr>
              <td colSpan={8} className="empty">
                {d.error}
              </td>
            </tr>
          ) : items.length ? (
            items.map((x: Obj) => (
              <tr key={x.id}>
                <td>
                  <code>{String(x.id).slice(0, 8)}</code>
                </td>
                <td>
                  {x.task_type === "source_fetch"
                    ? "元ファイル取得（agent）"
                    : x.task_type === "store"
                      ? "保存"
                      : x.task_type === "replicate"
                        ? "複製"
                        : x.task_type === "delete"
                          ? "削除"
                          : x.task_type}
                </td>
                <td>
                  <Status value={x.status} />
                </td>
                <td>
                  <span className="priority">{x.priority ?? "—"}</span>
                </td>
                <td>{x.attempt_count ?? 0} / 5</td>
                <td className="error-cell">{x.last_error || "—"}</td>
                <td>{x.target_node ? x.source_node : "-"}</td>
                <td>{date(x.updated_at)}</td>
                <td>
                  {role === "admin" && x.status === "failed" && (
                    <button className="small-btn" onClick={() => retry(x.id)}>
                      再試行
                    </button>
                  )}
                </td>
              </tr>
            ))
          ) : (
            <tr>
              <td colSpan={8} className="empty">
                タスクがありません
              </td>
            </tr>
          )}
        </Table>
      </section>
      <div className="info-card">
        <Clock3 size={18} />
        <div>
          <b>再試行</b>
          <p>
            失敗タスクは最大5回まで自動で再試行します。管理者は手動でも再試行できます。
          </p>
        </div>
      </div>
    </>
  );
}
function ConversationLinks({
  api,
  refresh,
  notify,
  targetType,
  targetId,
  role,
}: any) {
  const [localRefresh, setLocalRefresh] = useState(0);
  const [url, setURL] = useState("");
  const [label, setLabel] = useState("");
  const [busy, setBusy] = useState(false);
  const d = useData(
    api,
    `/admin/conversation-links?target_type=${encodeURIComponent(targetType)}&target_id=${encodeURIComponent(targetId)}`,
    refresh + localRefresh,
  );
  async function add(e: React.FormEvent) {
    e.preventDefault();
    if (!url.trim()) return;
    setBusy(true);
    try {
      await api("/admin/conversation-links", {
        method: "POST",
        body: JSON.stringify({
          target_type: targetType,
          target_id: targetId,
          url: url.trim(),
          label: label.trim(),
        }),
      });
      setURL("");
      setLabel("");
      setLocalRefresh((x) => x + 1);
      notify("会話URLを追加しました");
    } catch (e) {
      notify((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function remove(item: Obj) {
    if (!window.confirm(`「${item.label || item.url}」を削除しますか？`))
      return;
    try {
      await api(`/admin/conversation-links/${item.id}`, { method: "DELETE" });
      setLocalRefresh((x) => x + 1);
      notify("会話URLを削除しました");
    } catch (e) {
      notify((e as Error).message);
    }
  }
  const items: Obj[] = d.data.items || [];
  return (
    <section className="conversation-links">
      <div className="conversation-links-heading">
        <div>
          <b>連絡・会話URL</b>
          <p>LibreDesk の会話や関連ページを複数登録できます。</p>
        </div>
        <span className="type-pill">{items.length} 件</span>
      </div>
      {d.loading ? (
        <p className="muted">読み込み中…</p>
      ) : d.error ? (
        <p className="muted">URLを読み込めませんでした：{d.error}</p>
      ) : items.length ? (
        <ul className="conversation-link-list">
          {items.map((item: Obj) => (
            <li key={item.id}>
              <div className="conversation-link-main">
                <a href={item.url} target="_blank" rel="noreferrer">
                  {item.label || item.url}
                  <ExternalLink size={13} />
                </a>
                <small>{item.url}</small>
              </div>
              {role !== "viewer" && (
                <button
                  type="button"
                  className="small-btn"
                  onClick={() => remove(item)}
                >
                  削除
                </button>
              )}
            </li>
          ))}
        </ul>
      ) : (
        <p className="conversation-links-empty">
          まだ会話URLは登録されていません。
        </p>
      )}
      {role !== "viewer" && (
        <form className="conversation-link-form" onSubmit={add}>
          <label>
            表示名
            <input
              value={label}
              maxLength={255}
              onChange={(e) => setLabel(e.target.value)}
              placeholder="例：LibreDesk での連絡"
            />
          </label>
          <label>
            URL
            <input
              type="url"
              value={url}
              maxLength={4096}
              required
              onChange={(e) => setURL(e.target.value)}
              placeholder="https://support.example.com/..."
            />
          </label>
          <button className="secondary" disabled={busy || !url.trim()}>
            {busy ? "追加中…" : "URLを追加"}
          </button>
        </form>
      )}
    </section>
  );
}

function RequestDetailPage({
  api,
  refresh,
  notify,
  role,
  token,
  bumpRefresh,
}: any) {
  const { id = "" } = useParams();
  const nav = useNavigate();
  const [localRefresh, setLocalRefresh] = useState(0);
  const requests = useData(
    api,
    "/public-requests?limit=100",
    refresh + localRefresh,
  );
  const request: Obj | undefined = (requests.data.items || []).find(
    (x: Obj) => String(x.id) === id,
  );
  const artifactID = String(request?.artifact_id || "");
  const filesData = useData(
    api,
    artifactID
      ? `/files?artifact_id=${encodeURIComponent(artifactID)}&limit=100`
      : "/files?limit=100",
    refresh + localRefresh,
  );
  const files: Obj[] = (filesData.data.items || []).filter((x: Obj) =>
    artifactID
      ? String(x.artifact_id || artifactID) === artifactID
      : request?.file_object_id && x.id === request.file_object_id,
  );
  const [status, setStatus] = useState("in_review");
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const [password, setPassword] = useState("");
  const [activeArtifactID, setActiveArtifactID] = useState("");
  const [terms, setTerms] = useState(false);
  const [preserving, setPreserving] = useState(false);
  const [preserveMessage, setPreserveMessage] = useState("");
  const [intakeIngesting, setIntakeIngesting] = useState(false);
  const [intakeIngestMessage, setIntakeIngestMessage] = useState("");
  const [candidates, setCandidates] = useState<Obj[]>([]);
  const [selectedURLs, setSelectedURLs] = useState<string[]>([]);
  useEffect(() => {
    if (request) {
      setStatus(request.status === "pending" ? "in_review" : request.status);
      setReason("");
    }
  }, [request?.id, request?.status]);
  async function ingestAcceptedUpload() {
    if (!request) return;
    if (
      !window.confirm(
        "権利者の再配布許諾と配布条件を確認済みですか？ この操作はファイルを保存ノードへ保全しますが、一般公開は有効化しません。",
      )
    )
      return;
    setIntakeIngesting(true);
    setIntakeIngestMessage("保全タスクを登録しています…");
    try {
      const result = await api(`/public-requests/${request.id}/ingest-upload`, {
        method: "POST",
      });
      setIntakeIngestMessage(
        `保全タスクを登録しました。ファイルID: ${result.file_object_id || "登録済み"}。公開設定は別途確認してください。`,
      );
      notify("カタログ登録と保全タスクの登録が完了しました");
      setLocalRefresh((x) => x + 1);
      bumpRefresh?.();
    } catch (e) {
      setIntakeIngestMessage(`登録できませんでした：${(e as Error).message}`);
      notify((e as Error).message);
    } finally {
      setIntakeIngesting(false);
    }
  }
  async function saveStatus() {
    if (!request || reason.trim().length < 3) {
      notify("対応理由を3文字以上で入力してください");
      return;
    }
    if (
      request.request_type === "deletion" &&
      status === "accepted" &&
      role !== "admin"
    ) {
      notify("削除申請の承認は管理者のみです");
      return;
    }
    if (
      request.request_type === "deletion" &&
      status === "accepted" &&
      !window.confirm(
        "削除申請を承認すると、公開停止に加えて保存ノードへの削除タスクが登録されます。対象を確認しましたか？",
      )
    )
      return;
    setSaving(true);
    try {
      await api(`/public-requests/${request.id}`, {
        method: "PATCH",
        body: JSON.stringify({ status, reason: reason.trim() }),
      });
      notify("申請ステータスを更新しました");
      setReason("");
      setLocalRefresh((x) => x + 1);
      bumpRefresh?.();
    } catch (e) {
      notify((e as Error).message);
    } finally {
      setSaving(false);
    }
  }
  async function ensureArtifact(req: Obj) {
    if (req.artifact_id) {
      setActiveArtifactID(String(req.artifact_id));
      return String(req.artifact_id);
    }
    const artifact: Obj = await api("/artifacts", {
      method: "POST",
      body: JSON.stringify({
        title: req.title || "保全リクエスト",
        creator_name: req.creator_name || undefined,
        source_url: req.source_url,
        description: req.details || undefined,
      }),
    });
    await api(`/public-requests/${req.id}`, {
      method: "PATCH",
      body: JSON.stringify({
        status: req.status === "pending" ? "in_review" : req.status,
        reason: "申請詳細ページから作品情報を登録",
        artifact_id: artifact.id,
      }),
    });
    setActiveArtifactID(String(artifact.id));
    setLocalRefresh((x) => x + 1);
    bumpRefresh?.();
    return String(artifact.id);
  }
  function individualURL(raw: string) {
    try {
      const u = new URL(raw);
      return (
        u.pathname.toLowerCase().includes("/download/") ||
        (u.hostname.toLowerCase().startsWith("dl") &&
          u.hostname.toLowerCase().includes("getuploader.com") &&
          u.pathname.toLowerCase().includes("/g/"))
      );
    } catch {
      return false;
    }
  }
  async function beginPreservation() {
    if (!request || !request.source_url || !terms) return;
    setPreserving(true);
    setPreserveMessage("保全対象を準備しています…");
    setCandidates([]);
    setSelectedURLs([]);
    try {
      const aid = await ensureArtifact(request);
      if (individualURL(request.source_url)) {
        const current = await api(
          `/files?artifact_id=${encodeURIComponent(aid)}&limit=100`,
        );
        let file: Obj | undefined = (current.items || []).find(
          (x: Obj) =>
            x.id === request.file_object_id ||
            x.source_url === request.source_url,
        );
        if (file?.preservation_status === "verified") {
          setPreserveMessage(
            "このファイルはすでに検証済みです。重複取得を行いません。",
          );
          return;
        }
        if (!file) {
          file = await api(`/artifacts/${aid}/files`, {
            method: "POST",
            body: JSON.stringify({
              original_filename: (request.title || "取得待ち").slice(0, 500),
              size_bytes: 0,
              media_type: "application/octet-stream",
              source_url: request.source_url,
              redistribution_policy: "unknown",
            }),
          });
        }
        if (!file) throw new Error("ファイル登録結果を取得できませんでした");
        await api(`/public-requests/${request.id}`, {
          method: "PATCH",
          body: JSON.stringify({
            status: "in_review",
            reason: "保全対象ファイルを登録",
            artifact_id: aid,
            file_object_id: file.id,
          }),
        });
        const result = await api(`/files/${file.id}/ingest-source`, {
          method: "POST",
          body: JSON.stringify({
            source_url: request.source_url,
            accept_terms: true,
            priority: 3,
            password,
            public_request_id: request.id,
          }),
        });
        setPreserveMessage(
          result.source_fetch_task
            ? `取得タスクを ${result.source_fetch_task.node_name || "取得ノード"} に登録しました。取得成功後に保存タスクが作成されます。`
            : result.message || "取得タスクを登録しました。",
        );
        setLocalRefresh((x) => x + 1);
        bumpRefresh?.();
      } else {
        const result = await api(`/artifacts/${aid}/discover-source-files`, {
          method: "POST",
          body: JSON.stringify({
            listing_url: request.source_url,
            accept_terms: true,
            password,
            public_request_id: request.id,
          }),
        });
        const found: Obj[] = result.items || [];
        setCandidates(found);
        setSelectedURLs(found.map((x: Obj) => x.url));
        setPreserveMessage(
          `${result.pages_scanned ?? 0} ページを調査し、${found.length} 件のファイル候補を見つけました。取得するものを選択してください。`,
        );
      }
    } catch (e) {
      setPreserveMessage(`処理に失敗しました：${(e as Error).message}`);
    } finally {
      setPreserving(false);
    }
  }
  async function queueSelected() {
    const aid = activeArtifactID || String(request?.artifact_id || "");
    if (!request || !aid || !selectedURLs.length) return;
    setPreserving(true);
    let queued = 0,
      skipped = 0;
    const errors: string[] = [];
    try {
      const current = await api(
        `/files?artifact_id=${encodeURIComponent(aid)}&limit=100`,
      );
      const existing: Obj[] = [...(current.items || [])];
      let linked = String(request.file_object_id || "");
      for (const item of candidates.filter((x: Obj) =>
        selectedURLs.includes(x.url),
      )) {
        try {
          let file: Obj | undefined = existing.find(
            (x: Obj) => x.source_url === item.url,
          );
          if (file?.preservation_status === "verified") {
            skipped++;
            continue;
          }
          if (!file) {
            const created: Obj = await api(`/artifacts/${aid}/files`, {
              method: "POST",
              body: JSON.stringify({
                original_filename: (item.label || "取得待ち").slice(0, 500),
                size_bytes: 0,
                media_type: "application/octet-stream",
                source_url: item.url,
                redistribution_policy: "unknown",
              }),
            });
            file = created;
            existing.push(created);
          }
          if (!file?.id) throw new Error("ファイル登録結果にIDがありません");
          if (!linked) {
            await api(`/public-requests/${request.id}`, {
              method: "PATCH",
              body: JSON.stringify({
                status: "in_review",
                reason: "保全対象ファイルを登録",
                artifact_id: aid,
                file_object_id: file.id,
              }),
            });
            linked = file.id;
          }
          const result = await api(`/files/${file.id}/ingest-source`, {
            method: "POST",
            body: JSON.stringify({
              source_url: item.url,
              accept_terms: true,
              priority: 3,
              password,
              public_request_id: request.id,
            }),
          });
          if (result.source_fetch_task) queued++;
        } catch (e) {
          errors.push(`${item.label || item.url}: ${(e as Error).message}`);
        }
      }
      setPreserveMessage(
        `取得タスク ${queued} 件登録、検証済みスキップ ${skipped} 件、失敗 ${errors.length} 件${errors.length ? `。最初の失敗：${errors[0]}` : ""}`,
      );
      setCandidates([]);
      setSelectedURLs([]);
      setActiveArtifactID("");
      setLocalRefresh((x) => x + 1);
      bumpRefresh?.();
    } catch (e) {
      setPreserveMessage(`処理に失敗しました：${(e as Error).message}`);
    } finally {
      setPreserving(false);
    }
  }
  if (requests.loading)
    return (
      <>
        <Heading
          eyebrow="申請詳細"
          title="申請を読み込んでいます"
          desc="申請情報を取得しています。"
        />
        <div className="panel empty">読み込み中…</div>
      </>
    );
  if (requests.error || !request)
    return (
      <>
        <Heading
          eyebrow="申請詳細"
          title="申請が見つかりません"
          desc={
            requests.error ||
            "一覧から削除されたか、表示権限がない可能性があります。"
          }
          actions={
            <button
              className="secondary"
              onClick={() => nav("/admin/requests")}
            >
              申請一覧へ戻る
            </button>
          }
        />
      </>
    );
  return (
    <div className="request-page">
      <div className="request-page-top">
        <button
          type="button"
          className="back-link"
          onClick={() => nav("/admin/requests")}
        >
          ← 申請一覧に戻る
        </button>
        <span className="type-pill">
          {request.request_type === "preservation"
            ? "保全"
            : request.request_type === "deletion"
              ? "削除"
              : request.request_type === "takedown"
                ? "公開停止"
                : request.request_type === "redistribution_assistance"
                  ? "再配布相談"
                  : request.request_type === "publisher_assistance"
                    ? "配布代行"
                    : "移設先情報"}
        </span>
        <Status value={request.status} />
      </div>
      <Heading
        eyebrow="申請詳細"
        title={
          request.title ||
          request.linked_file_name ||
          request.linked_artifact_title ||
          "タイトル未入力の申請"
        }
        desc={`受付日時：${date(request.created_at)}　／　申請ID：${request.id}`}
      />
      <div className="request-page-grid">
        <div className="request-page-main">
          <section className="panel request-info-panel">
            <div className="panel-head">
              <div>
                <h2>申請内容</h2>
                <p>申請者から届いた情報</p>
              </div>
            </div>
            <dl className="request-page-dl">
              <dt>申請者</dt>
              <dd>{request.submitter_name || "匿名"}</dd>
              <dt>連絡先</dt>
              <dd>{request.contact || "未記入"}</dd>
              <dt>作者・配布者</dt>
              <dd>{request.creator_name || "未記入"}</dd>
              <dt>権利者への連絡先</dt>
              <dd>{request.rights_holder_contact || "未記入"}</dd>
              <dt>申請内容</dt>
              <dd className="request-page-details">
                {request.details || "補足はありません"}
              </dd>
              <dt>保全元URL</dt>
              <dd>
                {request.source_url ? (
                  <a href={request.source_url} target="_blank" rel="noreferrer">
                    {request.source_url} <ExternalLink size={13} />
                  </a>
                ) : (
                  "—"
                )}
              </dd>
              <dt>移設先URL</dt>
              <dd>
                {request.external_url ? (
                  <a
                    href={request.external_url}
                    target="_blank"
                    rel="noreferrer"
                  >
                    {request.external_url} <ExternalLink size={13} />
                  </a>
                ) : (
                  "—"
                )}
              </dd>
              <dt>パスワード</dt>
              <dd>
                {request.has_source_password
                  ? "申請時に提供あり（暗号化保存）"
                  : "未提供"}
              </dd>
            </dl>
          </section>
          {request.intake_file_name && (
            <section className="panel request-info-panel">
              <div className="panel-head">
                <div>
                  <h2>申請時の添付ファイル</h2>
                  <p>
                    権利確認のために送信されたファイル。公開用の保存ファイルとは別です。
                  </p>
                </div>
              </div>
              <dl className="request-page-dl">
                <dt>ファイル名</dt>
                <dd>{request.intake_file_name}</dd>
                <dt>サイズ</dt>
                <dd>
                  {request.intake_file_size
                    ? bytes(request.intake_file_size)
                    : "不明"}
                </dd>
                <dt>SHA-256</dt>
                <dd>
                  <code>{request.intake_file_sha256 || "—"}</code>
                </dd>
                <dt>送信日時</dt>
                <dd>{date(request.intake_file_uploaded_at)}</dd>
              </dl>
              {["rejected", "closed"].includes(request.status) ? (
                <p className="public-footnote">
                  申請が却下・終了となったため、添付ファイルは削除済みです。
                </p>
              ) : (
                <button
                  type="button"
                  className="secondary"
                  onClick={() =>
                    downloadRequestIntakeFile(
                      request.id,
                      token,
                      notify,
                      request.intake_file_name,
                    )
                  }
                >
                  添付ファイルを取得
                </button>
              )}
            </section>
          )}
          <section className="panel request-files-panel">
            <div className="panel-head">
              <div>
                <div style={{ display: "flex", flexDirection: "row" }}>
                  <h2>関連ファイル</h2>
                  <span className="type-pill">{files.length} 件</span>
                </div>
                <p>この申請に紐付く作品・ファイル</p>
              </div>
              {request.artifact_id && (
                <button
                  className="secondary"
                  onClick={() =>
                    nav(
                      `/admin/review?tab=files&artifact_id=${encodeURIComponent(request.artifact_id)}`,
                    )
                  }
                >
                  作品内の全ファイルを一括操作
                </button>
              )}
            </div>
            {!request.artifact_id && !request.file_object_id ? (
              <div className="empty">
                公開カタログのファイルはまだ紐付いていません。添付ファイルがある場合は、上の「申請時の添付ファイル」から確認できます。
              </div>
            ) : filesData.loading ? (
              <div className="empty">ファイルを読み込んでいます…</div>
            ) : filesData.error ? (
              <div className="empty">
                ファイル一覧を読み込めませんでした：{filesData.error}
              </div>
            ) : files.length ? (
              <div className="request-file-list">
                {files.map((file: Obj) => (
                  <article className="request-file-row" key={file.id}>
                    <div className="request-file-icon">
                      <Files size={18} />
                    </div>
                    <div className="request-file-info">
                      <b>
                        {file.original_filename ||
                          file.artifact_title ||
                          "ファイル名未設定"}
                      </b>
                      <small>{file.source_url || file.id}</small>
                      <div className="request-file-status">
                        <Status
                          value={
                            file.preservation_status || file.status || "queued"
                          }
                        />
                        <span>
                          {file.size_bytes
                            ? bytes(file.size_bytes)
                            : "サイズ未取得"}
                        </span>
                        <span>
                          {file.redistribution_policy === "allowed"
                            ? "公開許可済み"
                            : "一般公開しない"}
                        </span>
                      </div>
                    </div>
                    <button
                      type="button"
                      className="secondary"
                      onClick={() =>
                        nav(
                          `/admin/review?tab=files&file_id=${encodeURIComponent(file.id)}`,
                        )
                      }
                    >
                      ファイル操作
                    </button>
                  </article>
                ))}
              </div>
            ) : (
              <div className="empty">関連ファイルはまだありません。</div>
            )}
          </section>
          {role !== "viewer" && (
            <section className="panel request-conversations-panel">
              <div className="panel-head">
                <div>
                  <h2>連絡・会話</h2>
                  <p>LibreDesk の会話 URL を複数登録できます。</p>
                </div>
              </div>
              <ConversationLinks
                api={api}
                refresh={refresh + localRefresh}
                notify={notify}
                targetType="public_request"
                targetId={request.id}
                role={role}
              />
            </section>
          )}
        </div>
        <aside className="request-page-sidebar">
          {role !== "viewer" && (
            <section className="panel request-action-panel">
              <div className="panel-head">
                <div>
                  <h2>申請を処理</h2>
                  <p>確認後に状態を更新します。</p>
                </div>
              </div>
              <div className="form-stack">
                <label>
                  対応ステータス
                  <select
                    value={status}
                    onChange={(e) => setStatus(e.target.value)}
                  >
                    <option value="in_review">確認中</option>
                    <option
                      value="accepted"
                      disabled={
                        request.request_type === "deletion" && role !== "admin"
                      }
                    >
                      承認
                    </option>
                    <option value="rejected">却下</option>
                    <option value="closed">終了</option>
                  </select>
                </label>
                <label>
                  対応理由 *
                  <textarea
                    rows={4}
                    minLength={3}
                    value={reason}
                    onChange={(e) => setReason(e.target.value)}
                    placeholder="判断理由や申請者への連絡メモを入力"
                  />
                </label>
                <button
                  className="primary wide"
                  disabled={saving || reason.trim().length < 3}
                  onClick={saveStatus}
                >
                  {saving ? "保存中…" : "対応内容を保存"}
                </button>
              </div>
            </section>
          )}
          {role === "admin" &&
            ["redistribution_assistance", "publisher_assistance"].includes(
              request.request_type,
            ) &&
            request.intake_file_name &&
            request.status === "accepted" &&
            !request.artifact_id &&
            !request.file_object_id && (
              <section className="panel request-action-panel">
                <div className="panel-head">
                  <div>
                    <h2>許諾確認後の保全</h2>
                    <p>
                      権利者の再配布許諾と配布条件を確認した後に実行します。添付ファイルは自動ウイルス検査されません。
                    </p>
                  </div>
                </div>
                <div className="form-stack">
                  <p>
                    ファイルを非公開の保存ノードへ保全します。これだけでは一般公開・ダウンロードは有効になりません。
                  </p>
                  <button
                    type="button"
                    className="primary wide"
                    disabled={intakeIngesting}
                    onClick={ingestAcceptedUpload}
                  >
                    {intakeIngesting
                      ? "登録中…"
                      : "許諾確認済み：カタログ登録・保全"}
                  </button>
                  {intakeIngestMessage && (
                    <div className="info-card">
                      <div>
                        <b>処理結果</b>
                        <p>{intakeIngestMessage}</p>
                      </div>
                    </div>
                  )}
                </div>
              </section>
            )}
          {role === "admin" &&
            request.request_type === "preservation" &&
            request.source_url &&
            ["pending", "in_review"].includes(request.status) && (
              <section className="panel request-action-panel">
                <div className="panel-head">
                  <div>
                    <h2>保全を実行</h2>
                    <p>ファイルを取得して保存タスクを登録します。</p>
                  </div>
                </div>
                <div className="form-stack">
                  <label>
                    ダウンロードパスワード（任意）
                    <input
                      type="password"
                      maxLength={1024}
                      autoComplete="off"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      placeholder={
                        request.has_source_password
                          ? "申請時に提供済み。上書きする場合のみ入力"
                          : "必要な場合に入力"
                      }
                    />
                    <small>
                      空欄なら申請時に暗号化保存されたパスワードを使用します。追加認証は回避しません。
                    </small>
                  </label>
                  <label className="check-label">
                    <input
                      type="checkbox"
                      checked={terms}
                      onChange={(e) => setTerms(e.target.checked)}
                    />{" "}
                    利用規約を確認しました
                  </label>
                  <button
                    type="button"
                    className="primary wide"
                    disabled={!terms || preserving}
                    onClick={beginPreservation}
                  >
                    {preserving ? "処理中…" : "URLを調査・取得タスク登録"}
                  </button>
                  {preserveMessage && (
                    <div className="info-card">
                      <div>
                        <b>処理結果</b>
                        <p>{preserveMessage}</p>
                      </div>
                    </div>
                  )}
                  {candidates.length > 0 && (
                    <>
                      <div className="candidate-list">
                        {candidates.map((item: Obj) => (
                          <label className="candidate-row" key={item.url}>
                            <input
                              type="checkbox"
                              checked={selectedURLs.includes(item.url)}
                              onChange={(e) =>
                                setSelectedURLs((old) =>
                                  e.target.checked
                                    ? [...old, item.url]
                                    : old.filter((x) => x !== item.url),
                                )
                              }
                            />
                            <span>
                              <b>{item.label || "ファイル候補"}</b>
                              <small>{item.url}</small>
                            </span>
                          </label>
                        ))}
                      </div>
                      <button
                        type="button"
                        className="secondary"
                        onClick={() =>
                          setSelectedURLs(candidates.map((x: Obj) => x.url))
                        }
                      >
                        すべて選択
                      </button>
                      <button
                        type="button"
                        className="primary wide"
                        disabled={preserving || selectedURLs.length === 0}
                        onClick={queueSelected}
                      >
                        選択した {selectedURLs.length} 件を保全
                      </button>
                    </>
                  )}
                </div>
              </section>
            )}
        </aside>
      </div>
    </div>
  );
}

function Requests({ api, refresh, notify, role, bumpRefresh }: any) {
  const nav = useNavigate();
  const d = useData(api, "/public-requests?limit=100", refresh);
  const [filter, setFilter] = useState("pending");
  const [selectedRequestIDs, setSelectedRequestIDs] = useState<string[]>([]);
  const [bulkPreserveTerms, setBulkPreserveTerms] = useState(false);
  const [bulkPreserveBusy, setBulkPreserveBusy] = useState(false);
  const [bulkPreserveStatus, setBulkPreserveStatus] = useState("");
  const items = (d.data.items || []).filter(
    (x: Obj) => filter === "all" || x.status === filter,
  );
  function toggleRequest(id: string, checked: boolean) {
    setSelectedRequestIDs((old) =>
      checked
        ? old.includes(id)
          ? old
          : [...old, id]
        : old.filter((x) => x !== id),
    );
  }
  async function bulkReview(nextStatus: "accepted" | "rejected") {
    const selected = (d.data.items || []).filter((x: Obj) =>
      selectedRequestIDs.includes(x.id),
    );
    if (!selected.length) {
      notify("申請を選択してください");
      return;
    }
    if (nextStatus === "accepted" && role !== "admin") {
      notify("一括承認は管理者のみ利用できます");
      return;
    }
    if (
      nextStatus === "accepted" &&
      selected.some(
        (x: Obj) => !["deletion", "takedown"].includes(x.request_type),
      )
    ) {
      notify(
        "一括承認は削除・公開停止申請のみ対応しています。保全などの申請は個別に確認してください",
      );
      return;
    }
    if (
      selected.some((x: Obj) => !["pending", "in_review"].includes(x.status))
    ) {
      notify("未処理または確認中の申請を選択してください");
      return;
    }
    const reason = window.prompt(
      nextStatus === "accepted"
        ? "一括承認の理由を入力してください"
        : "一括却下の理由を入力してください",
    );
    if (!reason || reason.trim().length < 3) return;
    if (
      nextStatus === "accepted" &&
      !window.confirm(
        `選択した ${selected.length} 件の申請を承認します。削除申請は保存ノードへの削除タスクを登録します。続行しますか？`,
      )
    )
      return;
    let ok = 0;
    const failures: string[] = [];
    for (const item of selected) {
      try {
        await api(`/public-requests/${item.id}`, {
          method: "PATCH",
          body: JSON.stringify({ status: nextStatus, reason: reason.trim() }),
        });
        ok++;
      } catch (e) {
        failures.push(
          `${String(item.id).slice(0, 8)}: ${(e as Error).message}`,
        );
      }
    }
    setSelectedRequestIDs([]);
    bumpRefresh?.();
    notify(
      `${ok} 件を処理しました${failures.length ? `。失敗 ${failures.length} 件：${failures[0]}` : ""}`,
    );
  }
  async function bulkPreserveSelected() {
    const selected = (d.data.items || []).filter((x: Obj) =>
      selectedRequestIDs.includes(x.id),
    );
    if (role !== "admin") {
      notify("一括保全は管理者のみ利用できます");
      return;
    }
    if (!selected.length) {
      notify("保全リクエストを選択してください");
      return;
    }
    if (
      selected.some(
        (x: Obj) => x.request_type !== "preservation" || !x.source_url,
      )
    ) {
      notify(
        "一括保全では、保全元URLがある保全リクエストだけを選択してください",
      );
      return;
    }
    if (
      selected.some((x: Obj) => !["pending", "in_review"].includes(x.status))
    ) {
      notify("未処理または確認中の保全リクエストだけを選択してください");
      return;
    }
    if (!bulkPreserveTerms) {
      notify("利用規約を確認してチェックしてください");
      return;
    }
    if (
      !window.confirm(
        `選択した ${selected.length} 件の保全リクエストを一括処理します。個別URLは取得タスクを登録し、一覧URLは最大100件の掲載ファイルを調査して取得タスクを登録します。保全したファイルは一般公開されません。続行しますか？`,
      )
    )
      return;
    setBulkPreserveBusy(true);
    let requestProcessed = 0,
      queuedCount = 0,
      skipped = 0,
      noCandidates = 0;
    const failures: string[] = [];
    try {
      for (let i = 0; i < selected.length; i++) {
        const req = selected[i];
        try {
          setBulkPreserveStatus(
            `${i + 1}/${selected.length} 件目を処理中: ${req.title || req.source_url}`,
          );
          let artifactID = String(req.artifact_id || "");
          if (!artifactID) {
            const artifact: Obj = await api("/artifacts", {
              method: "POST",
              body: JSON.stringify({
                title: req.title || "保全リクエスト",
                creator_name: req.creator_name || undefined,
                source_url: req.source_url,
                description: req.details || undefined,
              }),
            });
            artifactID = artifact.id;
          }
          await api(`/public-requests/${req.id}`, {
            method: "PATCH",
            body: JSON.stringify({
              status: "in_review",
              reason: "一括保全の実行により確認中へ変更",
              artifact_id: artifactID,
              ...(req.file_object_id
                ? { file_object_id: req.file_object_id }
                : {}),
            }),
          });
          const existingResult = await api(
            `/files?artifact_id=${encodeURIComponent(artifactID)}&limit=100`,
          );
          const existingFiles: Obj[] = [...(existingResult.items || [])];
          if (isIndividualDownload(req.source_url)) {
            let file: Obj | undefined = req.file_object_id
              ? existingFiles.find((x: Obj) => x.id === req.file_object_id)
              : undefined;
            if (!file)
              file = existingFiles.find(
                (x: Obj) => x.source_url === req.source_url,
              );
            if (file?.preservation_status === "verified") {
              skipped++;
              requestProcessed++;
              continue;
            }
            if (!file) {
              const createdFile: Obj = await api(
                `/artifacts/${artifactID}/files`,
                {
                  method: "POST",
                  body: JSON.stringify({
                    original_filename: (req.title || "取得待ち").slice(0, 500),
                    size_bytes: 0,
                    media_type: "application/octet-stream",
                    source_url: req.source_url,
                    redistribution_policy: "unknown",
                  }),
                },
              );
              file = createdFile;
              existingFiles.push(createdFile);
            }
            if (!file)
              throw new Error("保全対象ファイルを作成できませんでした");
            const targetFile: Obj = file;
            await api(`/public-requests/${req.id}`, {
              method: "PATCH",
              body: JSON.stringify({
                status: "in_review",
                reason: "一括保全の対象ファイルを登録",
                artifact_id: artifactID,
                file_object_id: targetFile.id,
              }),
            });
            const result = await api(`/files/${targetFile.id}/ingest-source`, {
              method: "POST",
              body: JSON.stringify({
                source_url: req.source_url,
                accept_terms: true,
                priority: 3,
                public_request_id: req.id,
              }),
            });
            if (result.source_fetch_task) queuedCount++;
          } else {
            const discovered = await api(
              `/artifacts/${artifactID}/discover-source-files`,
              {
                method: "POST",
                body: JSON.stringify({
                  listing_url: req.source_url,
                  accept_terms: true,
                  public_request_id: req.id,
                }),
              },
            );
            const candidates: Obj[] = (discovered.items || []).slice(0, 100);
            if (!candidates.length) {
              noCandidates++;
              requestProcessed++;
              continue;
            }
            let linkedFileID = String(req.file_object_id || "");
            for (const item of candidates) {
              try {
                let file = existingFiles.find(
                  (x: Obj) => x.source_url === item.url,
                );
                if (file?.preservation_status === "verified") {
                  skipped++;
                  continue;
                }
                if (!file) {
                  const createdFile: Obj = await api(
                    `/artifacts/${artifactID}/files`,
                    {
                      method: "POST",
                      body: JSON.stringify({
                        original_filename: (item.label || "取得待ち").slice(
                          0,
                          500,
                        ),
                        size_bytes: 0,
                        media_type: "application/octet-stream",
                        source_url: item.url,
                        redistribution_policy: "unknown",
                      }),
                    },
                  );
                  file = createdFile;
                  existingFiles.push(createdFile);
                }
                if (!file)
                  throw new Error("保全対象ファイルを作成できませんでした");
                const targetFile: Obj = file;
                if (!linkedFileID) {
                  await api(`/public-requests/${req.id}`, {
                    method: "PATCH",
                    body: JSON.stringify({
                      status: "in_review",
                      reason: "一括保全の対象ファイルを登録",
                      artifact_id: artifactID,
                      file_object_id: targetFile.id,
                    }),
                  });
                  linkedFileID = targetFile.id;
                }
                const result = await api(
                  `/files/${targetFile.id}/ingest-source`,
                  {
                    method: "POST",
                    body: JSON.stringify({
                      source_url: item.url,
                      accept_terms: true,
                      priority: 3,
                      public_request_id: req.id,
                    }),
                  },
                );
                if (result.source_fetch_task) queuedCount++;
              } catch (e) {
                failures.push(
                  `${String(req.id).slice(0, 8)} / ${item.label || item.url}: ${(e as Error).message}`,
                );
              }
            }
          }
          requestProcessed++;
        } catch (e) {
          failures.push(
            `${String(req.id).slice(0, 8)}: ${(e as Error).message}`,
          );
        }
      }
    } finally {
      setBulkPreserveBusy(false);
      setSelectedRequestIDs([]);
      setBulkPreserveTerms(false);
      bumpRefresh?.();
      setBulkPreserveStatus(
        `一括保全終了：リクエスト ${requestProcessed}/${selected.length} 件処理、取得タスク ${queuedCount} 件登録、保全済みスキップ ${skipped} 件、候補なし ${noCandidates} 件、失敗 ${failures.length} 件${failures.length ? `。最初の失敗：${failures[0]}` : ""}`,
      );
      notify(
        `一括保全：取得タスク ${queuedCount} 件登録、保全済み ${skipped} 件スキップ、失敗 ${failures.length} 件`,
      );
    }
  }
  function isIndividualDownload(raw: string) {
    try {
      const u = new URL(raw);
      return (
        u.pathname.toLowerCase().includes("/download/") ||
        (u.hostname.toLowerCase().startsWith("dl") &&
          u.hostname.toLowerCase().includes("getuploader.com") &&
          u.pathname.toLowerCase().includes("/g/"))
      );
    } catch {
      return false;
    }
  }
  return (
    <>
      <Heading eyebrow="" title="申請" desc="保全・削除・公開停止の申請" />
      <div className="notice amber-notice">
        <div className="notice-icon">
          <AlertTriangle size={19} />
        </div>
        <div>
          <b>迷ったときは「確認中」に</b>
          <p>
            内容を確認し、必要なら申請者へ連絡してください。保全・公開・削除は別の処理です。削除の承認は保存先からの削除も実行するため、対象を確認してから承認してください。
          </p>
        </div>
      </div>
      <div className="review-summary-grid">
        <button
          type="button"
          className={
            "review-summary-card " + (filter === "pending" ? "selected" : "")
          }
          onClick={() => {
            setFilter("pending");
            setSelectedRequestIDs([]);
          }}
        >
          <span>未処理</span>
          <b>
            {
              (d.data.items || []).filter((x: Obj) => x.status === "pending")
                .length
            }
          </b>
          <small>まず確認する申請</small>
        </button>
        <button
          type="button"
          className={
            "review-summary-card " + (filter === "in_review" ? "selected" : "")
          }
          onClick={() => {
            setFilter("in_review");
            setSelectedRequestIDs([]);
          }}
        >
          <span>確認中</span>
          <b>
            {
              (d.data.items || []).filter((x: Obj) => x.status === "in_review")
                .length
            }
          </b>
          <small>対応を進めている申請</small>
        </button>
        <button
          type="button"
          className={
            "review-summary-card " + (filter === "all" ? "selected" : "")
          }
          onClick={() => {
            setFilter("all");
            setSelectedRequestIDs([]);
          }}
        >
          <span>すべて</span>
          <b>{(d.data.items || []).length}</b>
          <small>受付済みの申請</small>
        </button>
      </div>
      <div className="filter-bar">
        <div className="filter-tabs">
          {[
            ["pending", "未処理"],
            ["in_review", "確認中"],
            ["all", "すべて"],
            ["accepted", "承認"],
            ["rejected", "却下"],
            ["closed", "終了"],
          ].map(([v, l]) => (
            <button
              key={v}
              className={filter === v ? "active" : ""}
              onClick={() => {
                setFilter(v);
                setSelectedRequestIDs([]);
              }}
            >
              {l}
            </button>
          ))}
        </div>
        {role === "admin" && (
          <button
            className="secondary"
            disabled={bulkPreserveBusy}
            onClick={() => {
              const eligible = items.filter(
                (x: Obj) =>
                  x.request_type === "preservation" &&
                  x.source_url &&
                  ["pending", "in_review"].includes(x.status),
              );
              setSelectedRequestIDs(eligible.map((x: Obj) => x.id));
              setBulkPreserveTerms(false);
              notify(`${eligible.length} 件の保全リクエストを選択しました`);
            }}
          >
            <ClipboardList size={16} /> 保全リクエストを一括選択
          </button>
        )}
      </div>
      {selectedRequestIDs.length > 0 && role !== "viewer" && (
        <section className="bulk-toolbar panel">
          <div className="bulk-toolbar-title">
            <b>{selectedRequestIDs.length} 件の申請を選択中</b>
            <button
              className="small-btn"
              onClick={() => setSelectedRequestIDs([])}
            >
              選択解除
            </button>
          </div>
          <div className="bulk-toolbar-controls">
            {role === "admin" && (
              <>
                <button
                  className="primary"
                  disabled={
                    bulkPreserveBusy ||
                    !bulkPreserveTerms ||
                    (d.data.items || [])
                      .filter((x: Obj) => selectedRequestIDs.includes(x.id))
                      .some(
                        (x: Obj) =>
                          x.request_type !== "preservation" ||
                          !x.source_url ||
                          !["pending", "in_review"].includes(x.status),
                      )
                  }
                  onClick={bulkPreserveSelected}
                >
                  {bulkPreserveBusy ? "一括保全中…" : "一括で保全"}
                </button>
                <label className="check-label">
                  <input
                    type="checkbox"
                    checked={bulkPreserveTerms}
                    disabled={bulkPreserveBusy}
                    onChange={(e) => setBulkPreserveTerms(e.target.checked)}
                  />{" "}
                  利用規約を確認しました
                </label>
                <button
                  className="secondary"
                  disabled={bulkPreserveBusy}
                  onClick={() => bulkReview("accepted")}
                >
                  公開停止・削除を一括承認
                </button>
              </>
            )}
            <button
              className="secondary"
              disabled={bulkPreserveBusy}
              onClick={() => bulkReview("rejected")}
            >
              一括却下
            </button>
          </div>
        </section>
      )}
      {bulkPreserveStatus && (
        <section className="panel">
          <div className="info-card">
            <div>
              <b>一括保全の結果</b>
              <p>{bulkPreserveStatus}</p>
            </div>
          </div>
        </section>
      )}
      <section className="review-request-list" aria-label="申請一覧">
        {d.loading ? (
          <div className="panel empty">申請を読み込んでいます…</div>
        ) : d.error ? (
          <div className="panel empty">
            申請を読み込めませんでした：{d.error}
          </div>
        ) : items.length ? (
          items.map((x: Obj) => (
            <article className="review-request-card" key={x.id}>
              <div className="review-request-select">
                <input
                  type="checkbox"
                  aria-label={`${x.title || x.request_type}を選択`}
                  checked={selectedRequestIDs.includes(x.id)}
                  disabled={role === "viewer"}
                  onChange={(e) => toggleRequest(x.id, e.target.checked)}
                />
              </div>
              <div className="review-request-main">
                <div className="review-request-meta">
                  <span className="type-pill">
                    {x.request_type === "takedown"
                      ? "公開停止"
                      : x.request_type === "deletion"
                        ? "削除"
                        : x.request_type === "preservation"
                          ? "保全"
                          : x.request_type === "redistribution_assistance"
                            ? "再配布相談"
                            : x.request_type === "publisher_assistance"
                              ? "配布代行"
                              : "移設先情報"}
                  </span>
                  <Status value={x.status} />
                  <span className="review-request-date">
                    受付 {date(x.created_at)}
                  </span>
                  {x.intake_file_name && (
                    <span className="review-request-date">
                      添付ファイルあり
                    </span>
                  )}
                </div>
                <h3>
                  {x.title ||
                    x.linked_file_name ||
                    x.linked_artifact_title ||
                    "タイトル未入力の申請"}
                </h3>
                <p className="review-request-submitter">
                  申請者：{x.submitter_name || "匿名"}　·　連絡先：
                  {x.contact || "未記入"}
                </p>
                <p className="review-request-details">
                  {String(x.details || "").length > 180
                    ? String(x.details).slice(0, 180) + "…"
                    : x.details || "補足はありません"}
                </p>
                {x.source_url && (
                  <a
                    className="review-request-url"
                    href={x.source_url}
                    target="_blank"
                    rel="noreferrer"
                  >
                    保全元URLを開く <ExternalLink size={12} />
                  </a>
                )}
                {x.external_url && (
                  <a
                    className="review-request-url"
                    href={x.external_url}
                    target="_blank"
                    rel="noreferrer"
                  >
                    移設先URLを開く <ExternalLink size={12} />
                  </a>
                )}
              </div>
              <div className="review-request-actions">
                <button
                  className="primary"
                  onClick={() =>
                    nav(`/admin/requests/${encodeURIComponent(x.id)}`)
                  }
                >
                  申請ページを開く <span>→</span>
                </button>
                {(x.file_object_id || x.artifact_id) && (
                  <button
                    className="secondary"
                    onClick={() =>
                      nav(
                        `/admin/review?tab=files&${x.file_object_id ? "file_id=" + encodeURIComponent(x.file_object_id) : "artifact_id=" + encodeURIComponent(x.artifact_id)}`,
                      )
                    }
                  >
                    ファイル一覧
                  </button>
                )}
              </div>
            </article>
          ))
        ) : (
          <div className="panel empty">この条件の申請はありません。</div>
        )}
      </section>
    </>
  );
}
function UsersPage({ api, refresh, notify, role }: any) {
  const d = useData(api, "/admin/users", refresh);
  const [modal, setModal] = useState(false);
  const [form, setForm] = useState({
    email: "",
    display_name: "",
    password: "",
    role: "reviewer",
  });
  async function create(e: React.FormEvent) {
    e.preventDefault();
    try {
      await api("/admin/users", { method: "POST", body: JSON.stringify(form) });
      notify("ユーザーを作成しました");
      setModal(false);
    } catch (e) {
      notify((e as Error).message);
    }
  }
  return (
    <>
      <Heading
        eyebrow=""
        title="ユーザー"
        desc="ユーザーと権限"
        actions={
          role === "admin" && (
            <button className="primary" onClick={() => setModal(true)}>
              <Plus size={16} /> ユーザーを追加
            </button>
          )
        }
      />
      {role !== "admin" ? (
        <div className="info-card">
          <LockKeyhole />
          <div>
            <b>管理者のみ</b>
            <p>ユーザー管理は管理者のみ利用できます。</p>
          </div>
        </div>
      ) : (
        <section className="panel table-panel">
          <Table
            headers={[
              "ユーザー",
              "メールアドレス",
              "ロール",
              "状態",
              "作成日時",
            ]}
          >
            {d.loading ? (
              <tr>
                <td colSpan={5} className="empty">
                  読み込み中…
                </td>
              </tr>
            ) : d.error ? (
              <tr>
                <td colSpan={5} className="empty">
                  {d.error}
                </td>
              </tr>
            ) : (
              (d.data.items || []).map((x: Obj) => (
                <tr key={x.id}>
                  <td>
                    <div className="file-cell">
                      <div className="avatar">
                        {(x.display_name || x.email).slice(0, 1).toUpperCase()}
                      </div>
                      <b>{x.display_name}</b>
                    </div>
                  </td>
                  <td>{x.email}</td>
                  <td>
                    <span className="type-pill">{x.role}</span>
                  </td>
                  <td>
                    <Status value={x.disabled_at ? "offline" : "online"} />
                  </td>
                  <td>{date(x.created_at)}</td>
                </tr>
              ))
            )}
          </Table>
        </section>
      )}
      {modal && (
        <Modal title="ユーザーを作成" close={() => setModal(false)}>
          <form className="form-stack" onSubmit={create}>
            <label>
              表示名
              <input
                required
                value={form.display_name}
                onChange={(e) =>
                  setForm({ ...form, display_name: e.target.value })
                }
              />
            </label>
            <label>
              メールアドレス
              <input
                required
                type="email"
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
              />
            </label>
            <label>
              初期パスワード（12文字以上）
              <input
                required
                minLength={12}
                type="password"
                value={form.password}
                onChange={(e) => setForm({ ...form, password: e.target.value })}
              />
            </label>
            <label>
              ロール
              <select
                value={form.role}
                onChange={(e) => setForm({ ...form, role: e.target.value })}
              >
                <option value="reviewer">確認担当者</option>
                <option value="viewer">閲覧者</option>
                <option value="admin">管理者</option>
              </select>
            </label>
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setModal(false)}
              >
                キャンセル
              </button>
              <button className="primary">作成</button>
            </div>
          </form>
        </Modal>
      )}
    </>
  );
}
function Audit({ api, refresh }: any) {
  const d = useData(api, "/admin/audit-logs", refresh);
  return (
    <>
      <Heading eyebrow="" title="操作履歴" desc="管理操作の履歴" />
      <section className="panel table-panel">
        <Table headers={["日時", "実行主体", "対象", "操作", "結果", "理由"]}>
          {d.loading ? (
            <tr>
              <td colSpan={6} className="empty">
                読み込み中…
              </td>
            </tr>
          ) : d.error ? (
            <tr>
              <td colSpan={6} className="empty">
                {d.error}
              </td>
            </tr>
          ) : (
            (d.data.items || []).map((x: Obj) => (
              <tr key={x.id}>
                <td>{date(x.created_at)}</td>
                <td>
                  {x.actor_display_name ||
                    (x.actor_label === "anonymous"
                      ? "system"
                      : x.actor_label) ||
                    x.actor_email ||
                    x.actor_id ||
                    "system"}
                </td>
                <td>
                  {x.target_type}{" "}
                  <code>{String(x.target_id || "").slice(0, 8)}</code>
                </td>
                <td>{x.action}</td>
                <td>
                  <Status value={x.result || x.status || "success"} />
                </td>
                <td>{x.reason || x.details || "—"}</td>
              </tr>
            ))
          )}
        </Table>
      </section>
    </>
  );
}

function FileRow({
  x,
  role,
  isSelected,
  token,
  toggleSelected,
  setDetail,
  setPolicy,
  setPub,
  openMetadata,
}: {
  x: Obj;
  role: string;
  isSelected: boolean;
  token: string;
  toggleSelected: (id: string, checked: boolean) => void;
  setDetail: React.Dispatch<React.SetStateAction<Obj | null>>;
  setPolicy: React.Dispatch<React.SetStateAction<string>>;
  setPub: React.Dispatch<React.SetStateAction<boolean>>;
  openMetadata: React.Dispatch<React.SetStateAction<Obj>>;
}) {
  const [busy, setBusy] = React.useState(false);
  const [message, setMessage] = useState("");
  async function download() {
    setBusy(true);
    setMessage("準備中…");
    try {
      const created = await publicAPI<{
        job: { id: string };
        access_token: string;
      }>(`/files/${x.id}/downloads`, token, { method: "POST" });
      let ready = false;
      for (let attempt = 0; attempt < 90; attempt += 1) {
        await new Promise((resolve) => window.setTimeout(resolve, 2000));
        const status = await publicAPI<{ status: string; last_error?: string }>(
          `/downloads/${created.job.id}`,
          token,
          {
            headers: {
              "Download-Authorization": `Bearer ${created.access_token}`,
            },
          },
        );
        if (status.status === "ready") {
          ready = true;
          break;
        }
        if (status.status === "failed" || status.status === "expired")
          throw new Error(
            "ダウンロードを準備できませんでした。時間をおいて再度お試しください。",
          );
        setMessage("準備中…");
      }
      if (!ready)
        throw new Error(
          "準備に時間がかかっています。時間をおいて再度お試しください。",
        );
      const response = await fetch(
        `/api/v1/downloads/${created.job.id}/content`,
        {
          headers: {
            Authorization: `Bearer ${token}`,
            "Download-Authorization": `Bearer ${created.access_token}`,
          },
        },
      );
      if (!response.ok) {
        let detail = "";
        try {
          detail = (await response.json()).error || "";
        } catch {
          /* no body */
        }
        throw new Error(detail || "ダウンロードに失敗しました。");
      }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = x.original_filename;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      setMessage("ダウンロードを開始しました");
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "エラーが発生しました");
    } finally {
      setBusy(false);
    }
  }

  return (
    <tr className={isSelected ? "selected-row" : ""}>
      <td>
        <input
          type="checkbox"
          aria-label={`${x.original_filename}を選択`}
          checked={isSelected}
          disabled={
            role === "viewer" || !!x.deletion_requested_at || !!x.deleted_at
          }
          onChange={(e) => toggleSelected(x.id, e.target.checked)}
        />
      </td>
      <td>
        <div className="file-cell">
          <div className="file-icon">
            <Files size={17} />
          </div>
          <div>
            <b>{x.artifact_title || "作品名未設定"}</b>
            <small>作者：{x.creator_name || "不明"}</small>
            <span>{x.original_filename || "名称未設定"}</span>
            <small>
              <code>{x.id}</code>
            </small>
          </div>
        </div>
      </td>
      <td>
        <Status value={x.deleted_at ? "deleted" : x.preservation_status} />
        {x.deletion_requested_at && !x.deleted_at && (
          <small className="cell-sub">削除処理中</small>
        )}
      </td>
      <td>
        <Status value={x.redistribution_policy} />
        <small className="cell-sub">
          {x.public_download_enabled ? "公開フラグON" : "非公開"}
        </small>
      </td>
      <td>{bytes(x.size_bytes)}</td>
      <td>{date(x.created_at)}</td>
      <td>
        <div className="row-actions">
          <button
            className="small-btn"
            disabled={role === "viewer"}
            onClick={() => {
              setDetail(x);
              setPolicy(x.redistribution_policy || "unknown");
              setPub(!!x.public_download_enabled);
            }}
          >
            公開設定
          </button>
          <button
            className="small-btn"
            disabled={
              role === "viewer" || !!x.deletion_requested_at || !!x.deleted_at
            }
            onClick={() => openMetadata(x)}
          >
            メタデータ
          </button>
          <button
            className="small-btn"
            type="button"
            disabled={busy}
            onClick={download}
          >
            {busy ? "準備中…" : "ダウンロード"}
          </button>
        </div>
        {message && (
          <small className="public-download-message">{message}</small>
        )}
      </td>
    </tr>
  );
}

export default App;
