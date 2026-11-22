import { useState } from "react";
import { publicAPI } from "@/api/admin";
import { Modal } from "@/components/common/Modal";
import type { Obj } from "@/types/common";

export function FileReplaceModal({
	file,
	onClose,
	onReplaced,
}: {
	file: Obj;
	onClose: () => void;
	onReplaced: () => void | Promise<void>;
}) {
	const [form, setForm] = useState({
		original_filename: file.original_filename || "",
		size_bytes: String(file.size_bytes || 0),
		sha256: "",
		media_type: file.media_type || "",
		source_url: file.source_url || "",
		description: "",
		redistribution_policy: file.redistribution_policy || "unknown",
	});
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");

	async function submit(e: React.FormEvent) {
		e.preventDefault();
		setBusy(true);
		setError("");
		try {
			await publicAPI(`/catalog/files/${encodeURIComponent(file.id)}/replace`, {
				method: "POST",
				body: JSON.stringify({
					original_filename: form.original_filename,
					size_bytes: Number(form.size_bytes) || 0,
					sha256: form.sha256.trim() || undefined,
					media_type: form.media_type.trim() || undefined,
					source_url: form.source_url.trim() || undefined,
					description: form.description.trim() || undefined,
					redistribution_policy: form.redistribution_policy,
				}),
			});
			await onReplaced();
			onClose();
		} catch (e) {
			setError(e instanceof Error ? e.message : "差し替えに失敗しました。");
		} finally {
			setBusy(false);
		}
	}

	return (
		<Modal title="ファイルを差し替え" close={onClose}>
			<form className="form-stack" onSubmit={submit}>
				<p className="muted">
					現在のファイル: <b>{file.original_filename}</b>
				</p>
				<p className="muted">
					新しいファイルを登録します。旧ファイルは、新しいファイルの保全が検証された後に自動で削除されます。
				</p>
				<label>
					新しいファイル名 *
					<input
						required
						value={form.original_filename}
						onChange={(e) =>
							setForm({ ...form, original_filename: e.target.value })
						}
					/>
				</label>
				<label>
					サイズ（バイト）
					<input
						type="number"
						min={0}
						value={form.size_bytes}
						onChange={(e) => setForm({ ...form, size_bytes: e.target.value })}
					/>
				</label>
				<label>
					SHA-256（任意）
					<input
						value={form.sha256}
						maxLength={64}
						placeholder="64桁の16進数"
						onChange={(e) => setForm({ ...form, sha256: e.target.value })}
					/>
				</label>
				<label>
					元URL（任意）
					<input
						type="url"
						value={form.source_url}
						placeholder="https://uu.getuploader.com/..."
						onChange={(e) => setForm({ ...form, source_url: e.target.value })}
					/>
				</label>
				<label>
					説明（任意）
					<textarea
						rows={3}
						value={form.description}
						onChange={(e) => setForm({ ...form, description: e.target.value })}
					/>
				</label>
				<label>
					再配布ポリシー
					<select
						value={form.redistribution_policy}
						onChange={(e) =>
							setForm({ ...form, redistribution_policy: e.target.value })
						}
					>
						<option value="unknown">未確認</option>
						<option value="pending">確認待ち</option>
						<option value="in_progress">確認中</option>
						<option value="allowed">許可</option>
						<option value="restricted">制限付き許可</option>
					</select>
				</label>
				{error && <p className="policy-warning">{error}</p>}
				<div className="modal-actions">
					<button
						type="button"
						className="secondary"
						disabled={busy}
						onClick={onClose}
					>
						キャンセル
					</button>
					<button type="submit" className="primary" disabled={busy}>
						{busy ? "差し替え中…" : "差し替える"}
					</button>
				</div>
			</form>
		</Modal>
	);
}
