import {
	Archive,
	ClipboardList,
	Globe,
	HardDrive,
	LayoutDashboard,
	LockKeyhole,
	LogOut,
	Menu,
	RefreshCw,
	ScrollText,
	ShieldCheck,
	Users,
	Workflow,
	X,
} from "lucide-react";
import type React from "react";
import { useState } from "react";
import {
	NavLink,
	Route,
	Routes,
	useLocation,
	useNavigate,
} from "react-router-dom";
import CopyEmail from "./CopyEmail";
import { Audit } from "./features/audit/Audit";
import { Dashboard } from "./features/dashboard/Dashboard";
import { Nodes } from "./features/nodes/Nodes";
import PublicSite from "./features/public/PublicSite";
import { RequestDetailPage } from "./features/review/RequestDetailPage";
import { ReviewWorkspace } from "./features/review/ReviewWorkspace";
import { Tasks } from "./features/tasks/Tasks";
import { URLCollector } from "./features/url-collector/URLCollector";
import { UsersPage } from "./features/users/UsersPage";
import type { Obj } from "./types/common";

function App() {
	const apiBase = "/api/v1";
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
						<button className="primary wide" disabled={busy} type="submit">
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
						type="button"
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
						<button
							className="icon-btn"
							title="ログアウト"
							onClick={logout}
							type="button"
						>
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
						type="button"
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
							type="button"
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

export default App;
