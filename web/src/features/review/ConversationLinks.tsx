import { ExternalLink } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { useData } from "../../hooks/useData";
import type { Obj } from "../../types/common";

export function ConversationLinks({
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
