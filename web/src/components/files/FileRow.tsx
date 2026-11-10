import { Download, Files } from "lucide-react";
import React, { useState } from "react";
import { publicAPI } from "../../api/admin";
import { bytes, date } from "../../lib/format";
import type { Obj } from "../../types/common";
import { Modal } from "../common/Modal";
import { Status } from "../common/Status";

type SaveFilePicker = (options: { suggestedName: string }) => Promise<{
	createWritable: () => Promise<WritableStream<Uint8Array>>;
}>;

export function FileRow({
	x,
	role,
	isSelected,
	token,
	toggleSelected,
	setDetail,
	setPolicy,
	setPub,
	openMetadata,
	onRefresh,
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
	onRefresh?: () => void | Promise<void>;
}) {
	const [busy, setBusy] = React.useState(false);
	const [message, setMessage] = useState("");

	type RetryPasswordMode = "saved" | "change" | "clear";

	const [retryModalOpen, setRetryModalOpen] = React.useState(false);
	const [retrying, setRetrying] = React.useState(false);
	const [passwordMode, setPasswordMode] =
		React.useState<RetryPasswordMode>("saved");
	const [retryPassword, setRetryPassword] = React.useState("");
	const [retryError, setRetryError] = React.useState("");

	async function download() {
		setBusy(true);
		setMessage("準備中…");
		let writable: WritableStream<Uint8Array> | undefined;
		try {
			const pickerWindow = window as Window & {
				showSaveFilePicker?: SaveFilePicker;
			};
			if (!pickerWindow.showSaveFilePicker)
				throw new Error(
					"このブラウザーはストリーム保存に対応していません。HTTPS接続でChromeまたはEdgeをご利用ください。",
				);
			// The picker requires a user gesture, so open it before preparing the job.
			const handle = await pickerWindow.showSaveFilePicker({
				suggestedName: x.original_filename,
			});
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
			writable = await handle.createWritable();
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
			if (!response.body)
				throw new Error("ダウンロードの内容を取得できませんでした。");
			setMessage("保存中…");
			await response.body.pipeTo(writable);
			setMessage("ファイルを保存しました");
		} catch (e) {
			if (writable) await writable.abort().catch(() => {});
			setMessage(
				e instanceof DOMException && e.name === "AbortError"
					? "ダウンロードをキャンセルしました"
					: e instanceof Error
						? e.message
						: "エラーが発生しました",
			);
		} finally {
			setBusy(false);
		}
	}

	async function retryPreservation() {
		if (!x.source_url || retrying) return;

		if (passwordMode === "change" && !retryPassword) {
			setRetryError("新しいパスワードを入力してください。");
			return;
		}

		setRetrying(true);
		setRetryError("");
		setMessage("再試行タスクを登録しています…");

		try {
			const body: {
				source_url: string;
				accept_terms: boolean;
				priority: number;
				password?: string;
				clear_password?: boolean;
			} = {
				source_url: x.source_url,
				accept_terms: true,
				priority: 3,
			};

			switch (passwordMode) {
				case "saved":
					// passwordを送信しないことで保存済みの値を維持する
					break;

				case "change":
					body.password = retryPassword;
					break;

				case "clear":
					body.clear_password = true;
					break;
			}

			const result = await publicAPI<{
				file_object_id: string;
				source_fetch_task?: {
					task_id: string;
					node_name?: string;
					status: string;
				};
				status: string;
				message?: string;
			}>(`/files/${encodeURIComponent(x.id)}/ingest-source`, token, {
				method: "POST",
				body: JSON.stringify(body),
			});

			setRetryModalOpen(false);
			setRetryPassword("");
			setPasswordMode("saved");

			setMessage(
				result.source_fetch_task
					? `再試行タスクを ${result.source_fetch_task.node_name || "取得ノード"} に登録しました。`
					: result.message || "再試行タスクを登録しました。",
			);

			// 一覧の状態を更新する
			try {
				await onRefresh?.();
			} catch {
				setMessage("タスクは登録されましたが、一覧の更新に失敗しました。");
			}
		} catch (e) {
			setRetryError(
				e instanceof Error
					? `再試行に失敗しました：${e.message}`
					: "再試行に失敗しました。",
			);
		} finally {
			setRetrying(false);
		}
	}

	return (
		<>
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
					{x.preservation_status === "error" && (
						<button
							type="button"
							disabled={
								retrying ||
								role === "viewer" ||
								!!x.deletion_requested_at ||
								!!x.deleted_at ||
								!x.source_url
							}
							onClick={() => {
								setPasswordMode("saved");
								setRetryError("")
								setRetryPassword("");
								setRetryModalOpen(true);
							}}
						>
							再試行
						</button>
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
							type="button"
						>
							公開設定
						</button>
						<button
							className="small-btn"
							disabled={
								role === "viewer" || !!x.deletion_requested_at || !!x.deleted_at
							}
							onClick={() => openMetadata(x)}
							type="button"
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
			{retryModalOpen && (
				<Modal
					title="保全処理を再試行"
					close={() => {
						if (!retrying) {
							setRetryModalOpen(false);
							setRetryPassword("");
						}
					}}
				>
					<div className="modal-body">
						<p>
							<strong>{x.original_filename}</strong>
							の保全処理を再試行します。
						</p>

						<fieldset disabled={retrying}>
							<legend>ダウンロードパスワード</legend>

							<label>
								<input
									type="radio"
									name={`retry-password-${x.id}`}
									value="saved"
									checked={passwordMode === "saved"}
									onChange={() => setPasswordMode("saved")}
								/>
								保存済みのパスワードを使用
							</label>

							<label>
								<input
									type="radio"
									name={`retry-password-${x.id}`}
									value="change"
									checked={passwordMode === "change"}
									onChange={() => setPasswordMode("change")}
								/>
								新しいパスワードを指定
							</label>

							{passwordMode === "change" && (
								<input
									type="password"
									value={retryPassword}
									onChange={(e) => setRetryPassword(e.target.value)}
									placeholder="新しいパスワード"
									autoComplete="new-password"
									maxLength={1024}
								/>
							)}

							<label>
								<input
									type="radio"
									name={`retry-password-${x.id}`}
									value="clear"
									checked={passwordMode === "clear"}
									onChange={() => setPasswordMode("clear")}
								/>
								保存済みのパスワードを削除する
							</label>
						</fieldset>

						{passwordMode === "clear" && (
							<p className="cell-sub">
								パスワードを削除して、パスワードなしで取得を試みます。
							</p>
						)}

						{retryError && (
							<p role="alert" className="cell-sub">
								{retryError}
							</p>
						)}

						<div className="modal-actions">
							<button
								type="button"
								className="secondary"
								disabled={retrying}
								onClick={() => {
									setRetryModalOpen(false);
									setRetryPassword("");
								}}
							>
								キャンセル
							</button>

							<button
								type="button"
								disabled={
									retrying || (passwordMode === "change" && !retryPassword)
								}
								onClick={retryPreservation}
							>
								{retrying ? "登録中…" : "再試行"}
							</button>
						</div>
					</div>
				</Modal>
			)}
		</>
	);
}
