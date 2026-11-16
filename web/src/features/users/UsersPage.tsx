import { LockKeyhole, Pencil, Plus, Trash2 } from "lucide-react";
import type React from "react";
import { useEffect, useRef, useState } from "react";
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
	const [artifactsRefresh, setArtifactsRefresh] = useState(0);
	const artifactsData = useData(
		api,
		"/catalog/artifacts?all=true",
		refresh + localRefresh + artifactsRefresh,
	);

	const [modal, setModal] = useState(false);
	const [editModal, setEditModal] = useState(false);
	const [editingUser, setEditingUser] = useState<Obj | null>(null);
	const [authorArtifactIDs, setAuthorArtifactIDs] = useState<string[]>([]);
	const [initialAuthorArtifactIDs, setInitialAuthorArtifactIDs] = useState<
		string[]
	>([]);
	const [assignmentsLoading, setAssignmentsLoading] = useState(false);
	const [assignmentsError, setAssignmentsError] = useState("");

	const editSessionId = useRef(0);

	const [form, setForm] = useState({
		email: "",
		display_name: "",
		password: "",
		role: "reviewer",
	});

	const [editForm, setEditForm] = useState({
		display_name: "",
		role: "reviewer",
		disabled: false,
		reason: "",
	});

	const [passwordForm, setPasswordForm] = useState({
		new_password: "",
		confirm_password: "",
		reason: "",
	});

	const [saving, setSaving] = useState(false);
	const [deleting, setDeleting] = useState(false);
	const [resettingPassword, setResettingPassword] = useState(false);

	function reload() {
		setLocalRefresh((x) => x + 1);
	}

	function closeEdit() {
		editSessionId.current++;
		setEditModal(false);
		setEditingUser(null);
		setAuthorArtifactIDs([]);
		setInitialAuthorArtifactIDs([]);
		setAssignmentsError("");
		setAssignmentsLoading(false);
		setEditForm({
			display_name: "",
			role: "reviewer",
			disabled: false,
			reason: "",
		});
		setPasswordForm({
			new_password: "",
			confirm_password: "",
			reason: "",
		});
	}

	async function create(e: React.FormEvent) {
		e.preventDefault();

		try {
			await api("/admin/users", {
				method: "POST",
				body: JSON.stringify(form),
			});

			notify("ユーザーを作成しました");
			setForm({
				email: "",
				display_name: "",
				password: "",
				role: "reviewer",
			});
			reload();
			setModal(false);
		} catch (e) {
			notify((e as Error).message);
		}
	}

	async function loadAuthorAssignments(
		userID: string,
		sessionId = editSessionId.current,
	) {
		setAssignmentsLoading(true);
		setAssignmentsError("");

		try {
			const result: Obj = await api(
				`/admin/author-assignments/${encodeURIComponent(userID)}`,
			);
			if (editSessionId.current === sessionId) {
				setAuthorArtifactIDs(result.artifact_ids || []);
				setInitialAuthorArtifactIDs(result.artifact_ids || []);
			}
		} catch (error) {
			if (editSessionId.current === sessionId) {
				setAssignmentsError((error as Error).message);
			}
		} finally {
			if (editSessionId.current === sessionId) {
				setAssignmentsLoading(false);
			}
		}
	}

	function openEdit(user: Obj) {
		editSessionId.current++;
		setEditingUser(user);
		setAuthorArtifactIDs([]);
		setInitialAuthorArtifactIDs([]);
		setAssignmentsError("");
		const sessionId = editSessionId.current;
		void loadAuthorAssignments(user.id, sessionId);

		setEditForm({
			display_name: String(user.display_name || ""),
			role: String(user.role || "reviewer"),
			disabled: Boolean(user.disabled_at),
			reason: "",
		});

		setPasswordForm({
			new_password: "",
			confirm_password: "",
			reason: "",
		});

		setEditModal(true);
	}

	// ユーザー情報更新
	async function update(e: React.FormEvent) {
		e.preventDefault();

		if (!editingUser || saving) return;

		const savesAssignments =
			editingUser.role !== "author" ||
			authorArtifactIDs.length !== initialAuthorArtifactIDs.length ||
			authorArtifactIDs.some((id) => !initialAuthorArtifactIDs.includes(id));
		if (
			editForm.role === "author" &&
			(assignmentsLoading ||
				artifactsData.loading ||
				assignmentsError ||
				artifactsData.error)
		) {
			return;
		}

		const sessionId = editSessionId.current;

		setSaving(true);

		try {
			const body: Record<string, unknown> = {
				display_name: editForm.display_name,
				role: editForm.role,
				disabled: editForm.disabled,
			};

			if (editForm.reason.trim()) {
				body.reason = editForm.reason.trim();
			}

			if (editForm.role === "author" && savesAssignments) {
				body.artifact_ids = authorArtifactIDs;
			}

			await api(`/admin/users/${editingUser.id}`, {
				method: "PUT",
				body: JSON.stringify(body),
			});

			notify("ユーザー情報を更新しました");

			if (editSessionId.current === sessionId) {
				closeEdit();
			}

			reload();
		} catch (e) {
			notify((e as Error).message);
		} finally {
			setSaving(false);
		}
	}

	// パスワードリセット
	async function resetPassword(e: React.FormEvent) {
		e.preventDefault();

		if (!editingUser || resettingPassword) return;

		if (passwordForm.new_password !== passwordForm.confirm_password) {
			notify("パスワードが一致しません");
			return;
		}

		if (!passwordForm.reason.trim() || passwordForm.reason.trim().length < 3) {
			notify("パスワード変更理由を3文字以上で入力してください");
			return;
		}

		const sessionId = editSessionId.current;

		setResettingPassword(true);

		try {
			await api(`/admin/users/${editingUser.id}/password`, {
				method: "PUT",
				body: JSON.stringify({
					new_password: passwordForm.new_password,
					confirm_password: passwordForm.confirm_password,
					reason: passwordForm.reason.trim(),
				}),
			});

			notify("パスワードをリセットしました");

			setPasswordForm({
				new_password: "",
				confirm_password: "",
				reason: "",
			});

			if (editSessionId.current === sessionId) {
				setPasswordForm({
					new_password: "",
					confirm_password: "",
					reason: "",
				});
			}

			reload();
		} catch (e) {
			notify((e as Error).message);
		} finally {
			setResettingPassword(false);
		}
	}

	// ユーザー削除
	async function deleteUser(user: Obj) {
		if (deleting) return;

		const confirmed = window.confirm(
			`「${user.display_name || user.email}」を削除しますか？\nこの操作は取り消せません。`,
		);

		if (!confirmed) return;

		setDeleting(true);

		try {
			await api(`/admin/users/${user.id}`, {
				method: "DELETE",
			});

			notify("ユーザーを削除しました");

			if (editingUser?.id === user.id) {
				closeEdit();
			}

			reload();
		} catch (e) {
			notify((e as Error).message);
		} finally {
			setDeleting(false);
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
							<Plus size={16} />
							ユーザーを追加
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
							"操作",
						]}
					>
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
									<td>
										<div className="file-cell">
											<button
												type="button"
												className="secondary"
												onClick={() => openEdit(x)}
											>
												<Pencil size={14} />
												編集
											</button>
											<button
												type="button"
												className="secondary"
												onClick={() => deleteUser(x)}
												disabled={deleting}
												title="ユーザーを削除"
											>
												<Trash2 size={14} />
											</button>
										</div>
									</td>
								</tr>
							))
						)}
					</Table>
				</section>
			)}

			{/* ユーザー作成 */}
			{modal && (
				<Modal title="ユーザーを作成" close={() => setModal(false)}>
					<form className="form-stack" onSubmit={create}>
						<label>
							表示名
							<input
								required
								value={form.display_name}
								onChange={(e) =>
									setForm({
										...form,
										display_name: e.target.value,
									})
								}
							/>
						</label>

						<label>
							メールアドレス
							<input
								required
								type="email"
								value={form.email}
								onChange={(e) =>
									setForm({
										...form,
										email: e.target.value,
									})
								}
							/>
						</label>

						<label>
							初期パスワード（12文字以上）
							<input
								required
								minLength={12}
								type="password"
								value={form.password}
								onChange={(e) =>
									setForm({
										...form,
										password: e.target.value,
									})
								}
							/>
						</label>

						<label>
							ロール
							<select
								value={form.role}
								onChange={(e) =>
									setForm({
										...form,
										role: e.target.value,
									})
								}
							>
								<option value="reviewer">確認担当者</option>
								<option value="viewer">閲覧者</option>
								<option value="author">Author</option>
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

			{/* ユーザー編集 */}
			{editModal && editingUser && (
				<Modal title="ユーザー情報を編集" close={closeEdit}>
					<div className="form-stack">
						<div className="info-card">
							<div>
								<b>{editingUser.email}</b>
								<p>ユーザーID: {editingUser.id}</p>
							</div>
						</div>

						<form className="form-stack" onSubmit={update}>
							<label>
								表示名
								<input
									required
									maxLength={120}
									value={editForm.display_name}
									onChange={(e) =>
										setEditForm({
											...editForm,
											display_name: e.target.value,
										})
									}
								/>
							</label>

							<label>
								ロール
								<select
									value={editForm.role}
									onChange={(e) =>
										setEditForm({
											...editForm,
											role: e.target.value,
										})
									}
								>
									<option value="reviewer">確認担当者</option>
									<option value="viewer">閲覧者</option>
									<option value="author">Author</option>
									<option value="admin">管理者</option>
								</select>
							</label>

							{editForm.role === "author" && (
								<section className="form-stack">
									<b>操作可能な作品</b>
									<p className="muted">
										選択した作品に属するファイルのみ閲覧・削除できます。
									</p>
									{assignmentsLoading ? (
										<p>割り当てを読み込み中...</p>
									) : assignmentsError ? (
										<div className="policy-warning">
											<p>{assignmentsError}</p>
											<button
												type="button"
												className="secondary"
												onClick={() =>
													editingUser &&
													void loadAuthorAssignments(editingUser.id)
												}
											>
												再試行
											</button>
										</div>
									) : artifactsData.loading ? (
										<p>作品一覧を読み込み中...</p>
									) : artifactsData.error ? (
										<div className="policy-warning">
											<p>
												作品一覧を取得できませんでした: {artifactsData.error}
											</p>
											<button
												type="button"
												className="secondary"
												onClick={() =>
													setArtifactsRefresh((current) => current + 1)
												}
											>
												作品一覧を再取得
											</button>
										</div>
									) : !(artifactsData.data.items || []).length ? (
										<p>作品がありません。</p>
									) : (
										(artifactsData.data.items || []).map((artifact: Obj) => (
											<label key={artifact.id} className="check-label">
												<input
													type="checkbox"
													checked={authorArtifactIDs.includes(artifact.id)}
													onChange={(event) =>
														setAuthorArtifactIDs((current) =>
															event.target.checked
																? [...current, artifact.id]
																: current.filter((id) => id !== artifact.id),
														)
													}
												/>
												{artifact.title}
											</label>
										))
									)}
								</section>
							)}

							<label>
								アカウント状態
								<select
									value={editForm.disabled ? "disabled" : "active"}
									onChange={(e) =>
										setEditForm({
											...editForm,
											disabled: e.target.value === "disabled",
										})
									}
								>
									<option value="active">有効</option>
									<option value="disabled">無効</option>
								</select>
							</label>

							<label>
								変更理由（任意）
								<textarea
									maxLength={2000}
									value={editForm.reason}
									onChange={(e) =>
										setEditForm({
											...editForm,
											reason: e.target.value,
										})
									}
									placeholder="変更理由を入力"
								/>
							</label>

							<div className="modal-actions">
								<button type="button" className="secondary" onClick={closeEdit}>
									キャンセル
								</button>
								<button
									className="primary"
									disabled={
										saving ||
										(editForm.role === "author" &&
											(assignmentsLoading ||
												artifactsData.loading ||
												!!assignmentsError ||
												!!artifactsData.error))
									}
								>
									{saving ? "保存中…" : "変更を保存"}
								</button>
							</div>
						</form>

						<hr />

						<h3>パスワードリセット</h3>
						<p>このユーザーのパスワードを新しいものに変更します。</p>

						<form className="form-stack" onSubmit={resetPassword}>
							<label>
								新しいパスワード（12文字以上）
								<input
									required
									type="password"
									minLength={12}
									maxLength={72}
									value={passwordForm.new_password}
									onChange={(e) =>
										setPasswordForm({
											...passwordForm,
											new_password: e.target.value,
										})
									}
								/>
							</label>

							<label>
								パスワード確認
								<input
									required
									type="password"
									minLength={12}
									maxLength={72}
									value={passwordForm.confirm_password}
									onChange={(e) =>
										setPasswordForm({
											...passwordForm,
											confirm_password: e.target.value,
										})
									}
								/>
							</label>

							<label>
								リセット理由（必須）
								<textarea
									required
									minLength={3}
									maxLength={2000}
									value={passwordForm.reason}
									onChange={(e) =>
										setPasswordForm({
											...passwordForm,
											reason: e.target.value,
										})
									}
									placeholder="例: ユーザーからのパスワード再設定依頼"
								/>
							</label>

							<button className="secondary" disabled={resettingPassword}>
								{resettingPassword ? "リセット中…" : "パスワードをリセット"}
							</button>
						</form>
					</div>
				</Modal>
			)}
		</>
	);
}
