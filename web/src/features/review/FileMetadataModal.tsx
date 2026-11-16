import { useState } from "react";
import type React from "react";
import { Modal } from "@/components/common/Modal";
import type { Obj } from "@/types/common";

type Props = {
	file: Obj;
	api: (path: string, options?: Obj) => Promise<any>;
	notify: (message: string) => void;
	busy: boolean;
	setBusy: (busy: boolean) => void;
	close: () => void;
	onSaved: () => void;
};

export function FileMetadataModal({ file, api, notify, busy, setBusy, close, onSaved }: Props) {
	const [form, setForm] = useState({
		original_filename: file.original_filename || "",
		media_type: file.media_type || "",
		source_url: file.source_url || "",
		description: file.description || "",
	});

	async function save(event: React.FormEvent) {
		event.preventDefault();
		const reason = window.prompt("ファイルメタデータの変更理由を入力してください");
		if (!reason || reason.trim().length < 3) return;
		if (Array.from(form.description).length > 20_000) {
			notify("ファイルの説明は20,000文字以内で入力してください");
			return;
		}
		setBusy(true);
		try {
			await api(`/catalog/files/${file.id}/metadata`, {
				method: "PATCH",
				body: JSON.stringify({
					original_filename: form.original_filename,
					media_type: form.media_type,
					source_url: form.source_url,
					description: form.description,
					reason: reason.trim(),
				}),
			});
			notify("ファイルのメタデータを更新しました");
			onSaved();
		} catch (error) {
			notify((error as Error).message);
		} finally {
			setBusy(false);
		}
	}

	return (
		<Modal title="ファイルのメタデータ" close={close}>
			<form className="form-stack" onSubmit={save}>
				<div className="detail-summary">
					<b>{file.artifact_title || "作品情報なし"}</b>
					<small>{file.id}</small>
				</div>
				<label>
					ファイル名 *
					<input required value={form.original_filename} onChange={(event) => setForm({ ...form, original_filename: event.target.value })} />
				</label>
				<label>
					ファイルの説明
					<textarea rows={3} value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} />
				</label>
				<label>
					MIME タイプ
					<input value={form.media_type} onChange={(event) => setForm({ ...form, media_type: event.target.value })} placeholder="application/zip" />
				</label>
				<label>
					ファイル取得元URL
					<input type="url" value={form.source_url} onChange={(event) => setForm({ ...form, source_url: event.target.value })} />
				</label>
				<div className="modal-actions">
					<button type="button" className="secondary" onClick={close}>キャンセル</button>
					<button className="primary" disabled={busy}>{busy ? "保存中…" : "ファイル情報を保存"}</button>
				</div>
			</form>
		</Modal>
	);
}
