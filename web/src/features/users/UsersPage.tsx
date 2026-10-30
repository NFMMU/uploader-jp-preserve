import { LockKeyhole, Plus } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { Heading } from "../../components/common/Heading";
import { Modal } from "../../components/common/Modal";
import { Status } from "../../components/common/Status";
import { Table } from "../../components/common/Table";
import { useData } from "../../hooks/useData";
import { date } from "../../lib/format";
import type { Obj } from "../../types/common";

export function UsersPage({ api, refresh, notify, role }: any) {
	const [localRefresh, setLocalRefresh] = useState(0);
	const d = useData(api, "/admin/users", refresh + localRefresh);
	const [modal, setModal] = useState(false);
	const [form, setForm] = useState({
		email: "",
		display_name: "",
		custom_id: "",
		password: "",
		role: "reviewer",
	});
	async function create(e: React.FormEvent) {
		e.preventDefault();
		try {
			await api("/admin/users", { method: "POST", body: JSON.stringify(form) });
			notify("ユーザーを作成しました");
			setForm({ email: "", display_name: "",custom_id:"", password: "",  role: "reviewer" });
			setLocalRefresh((x) => x + 1);
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
											<small>{`@${x.custom_id}`}</small>
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
							ユーザーID
							<input type="text" name="custom_id" id="custom_id"
							onChange={(e) => setForm({ ...form, custom_id: e.target.value })}/>
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
								<option value="viewer">閲覧者</option>
								<option value="author">権利者</option>
								<option value="reviewer">確認担当者</option>
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
