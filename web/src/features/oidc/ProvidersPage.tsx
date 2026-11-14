import CopyEmail from "components/common/CopyEmail";
import { LockKeyhole, Plus } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { Heading } from "@/components/common/Heading";
import { Modal } from "@/components/common/Modal";
import { Status } from "@/components/common/Status";
import { Table } from "@/components/common/Table";
import { useData } from "@/hooks/useData";
import { date } from "@/lib/format";
import type { Obj } from "@/types/common";

export function ProvidersPage({
	api,
	refresh,
	notify,
	role,
}: {
	api: (path: string, opts?: RequestInit) => Promise<Obj>;
	refresh: number;
	notify: (s: string) => void;
	role: any;
}) {
	const [localRefresh, setLocalRefresh] = useState(0);
	const d = useData(api, "/admin/oidc_providers", refresh + localRefresh);
	const [modal, setModal] = useState(false);
	const [form, setForm] = useState({
		display_name: "",
		service_icon: "",
		discovery_endpoint: "",
		client_id: "",
		client_secret: "",
	});
	async function create(e: React.FormEvent) {
		e.preventDefault();
		try {
			await api("/admin/oidc_providers", {
				method: "POST",
				body: JSON.stringify(form),
			});
			notify("OIDCプロバイダーのセットアップを完了しました");
			setForm({
				display_name: "",
				service_icon: "",
				discovery_endpoint: "",
				client_id: "",
				client_secret: "",
			});
			setLocalRefresh((x) => x + 1);
			setModal(false);
		} catch (e) {
			notify((e as Error).message);
		}
	}

	const hostname = window.location.hostname;
	return (
		<>
			<Heading
				eyebrow=""
				title="OIDCプロバイダー"
				desc="OIDCプロバイダー構成の管理"
				actions={
					role === "admin" && (
						<button className="primary" onClick={() => setModal(true)}>
							<Plus size={16} /> OIDCプロバイダーをセットアップ
						</button>
					)
				}
			/>
			{role !== "admin" ? (
				<div className="info-card">
					<LockKeyhole />
					<div>
						<b>管理者のみ</b>
						<p>OIDCプロバイダー管理は管理者のみ利用できます。</p>
					</div>
				</div>
			) : (
				<section className="panel table-panel">
					<Table headers={["名前", "コールバックURL", "状態", "作成日時"]}>
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
												{x.service_icon ? (
													<img alt="サービスアイコン" src={x.service_icon} />
												) : (
													x.display_name.slice(0, 1).toUpperCase()
												)}
											</div>
											<b>{x.display_name}</b>
										</div>
									</td>
									<td>
										<span>
											https://{hostname}/auth/oidc/callback/{x.id}
										</span>
										<CopyEmail
											label="URLを"
											email={`https://${hostname}/auth/oidc/callback/${x.id}`}
										/>
									</td>
									<td>
										{x.is_enabled ? (
											<Status value="enabled" />
										) : (
											<Status value="disabled" />
										)}
									</td>
									<td>{date(x.created_at)}</td>
								</tr>
							))
						)}
					</Table>
				</section>
			)}
			{modal && (
				<Modal
					title="OIDCプロバイダーをセットアップ"
					close={() => setModal(false)}
				>
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
							アイコンURL
							<input
								required
								type="url"
								value={form.service_icon}
								onChange={(e) =>
									setForm({ ...form, service_icon: e.target.value })
								}
							/>
						</label>
						<label>
							OIDC Discovery Endpoint
							<input
								required
								type={"url"}
								value={form.discovery_endpoint}
								onChange={(e) =>
									setForm({ ...form, discovery_endpoint: e.target.value })
								}
							/>
						</label>
						<label>
							Client ID
							<input
								required
								value={form.client_id}
								onChange={(e) =>
									setForm({ ...form, client_id: e.target.value })
								}
							/>
						</label>
						<label>
							Client Secret
							<input
								required
								type="password"
								value={form.client_secret}
								onChange={(e) =>
									setForm({ ...form, client_secret: e.target.value })
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
							<button className="primary">作成</button>
						</div>
					</form>
				</Modal>
			)}
		</>
	);
}
