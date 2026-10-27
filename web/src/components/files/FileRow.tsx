import { Download, Files } from "lucide-react";
import React, { useState } from "react";
import { publicAPI } from "../../api/admin";
import { bytes, date } from "../../lib/format";
import type { Obj } from "../../types/common";
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
