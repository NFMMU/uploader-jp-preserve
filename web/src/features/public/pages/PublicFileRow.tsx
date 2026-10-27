import { useState } from "react";
import { Link } from "react-router-dom";
import { publicAPI } from "../../../api/public";
import { formatBytes } from "../../../lib/format";
import type { PublicFile } from "../../../types/public";

export function PublicFileRow({
	file,
	artifactId,
}: {
	file: PublicFile;
	artifactId: string;
}) {
	const [busy, setBusy] = useState(false);
	const [message, setMessage] = useState("");
	async function download() {
		setBusy(true);
		setMessage("準備中…");
		try {
			const created = await publicAPI<{
				job: { id: string };
				access_token: string;
			}>(`/public/files/${file.id}/downloads`, { method: "POST" });
			let ready = false;
			for (let attempt = 0; attempt < 90; attempt += 1) {
				await new Promise((resolve) => window.setTimeout(resolve, 2000));
				const status = await publicAPI<{ status: string; last_error?: string }>(
					`/public/downloads/${created.job.id}`,
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
				`/api/v1/public/downloads/${created.job.id}/content`,
				{
					headers: {
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
			anchor.download = file.original_filename;
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
		<div className="public-file-row">
			<div className="public-file-main">
				<b>{file.original_filename}</b>
				<span>{formatBytes(file.size_bytes)}</span>
				{message && (
					<small className="public-download-message">{message}</small>
				)}
			</div>
			<div className="public-file-actions">
				<button type="button" disabled={busy} onClick={download}>
					{busy ? "準備中…" : "ダウンロード"}
				</button>
				<Link
					to={`/requests?artifact_id=${encodeURIComponent(artifactId)}&file_id=${encodeURIComponent(file.id)}`}
				>
					公開停止等
				</Link>
				<Link
					to={`/relocation-info?artifact_id=${encodeURIComponent(artifactId)}&file_id=${encodeURIComponent(file.id)}`}
				>
					移設先情報
				</Link>
			</div>
		</div>
	);
}
