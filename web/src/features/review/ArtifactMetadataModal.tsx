import type React from "react";
import { useState } from "react";
import { Modal } from "@/components/common/Modal";
import type { Obj } from "@/types/common";

type Props = {
	artifact: Obj;
	api: (path: string, options?: Obj) => Promise<any>;
	notify: (message: string) => void;
	busy: boolean;
	setBusy: (busy: boolean) => void;
	close: () => void;
	onSaved: () => void;
};

export function ArtifactMetadataModal({
	artifact,
	api,
	notify,
	busy,
	setBusy,
	close,
	onSaved,
}: Props) {
	const [form, setForm] = useState({
		title: artifact.title || "",
		creator_name: artifact.creator_name || "",
		source_url: artifact.source_url || "",
		description: artifact.description || "",
	});

	async function save(event: React.FormEvent) {
		event.preventDefault();
		const reason = window.prompt("作品メタデータの変更理由を入力してください");
		if (!reason || reason.trim().length < 3) return;
		setBusy(true);
		try {
			await api(`/catalog/artifacts/${artifact.id}`, {
				method: "PATCH",
				body: JSON.stringify({ ...form, reason: reason.trim() }),
			});
			notify("作品のメタデータを更新しました");
			onSaved();
		} catch (error) {
			notify((error as Error).message);
		} finally {
			setBusy(false);
		}
	}

	return (
		<Modal title="作品情報を編集" close={close}>
			<form className="form-stack" onSubmit={save}>
				<label>
					作品名 *
					<input
						required
						value={form.title}
						onChange={(event) =>
							setForm({ ...form, title: event.target.value })
						}
					/>
				</label>
				<label>
					作者・配布者
					<input
						value={form.creator_name}
						onChange={(event) =>
							setForm({ ...form, creator_name: event.target.value })
						}
					/>
				</label>
				<label>
					作品の元ページURL *
					<input
						required
						type="url"
						value={form.source_url}
						onChange={(event) =>
							setForm({ ...form, source_url: event.target.value })
						}
					/>
				</label>
				<label>
					作品の説明
					<textarea
						rows={4}
						value={form.description}
						onChange={(event) =>
							setForm({ ...form, description: event.target.value })
						}
					/>
				</label>
				<div className="modal-actions">
					<button type="button" className="secondary" onClick={close}>
						キャンセル
					</button>
					<button type="submit" className="primary" disabled={busy}>
						{busy ? "保存中…" : "作品情報を保存"}
					</button>
				</div>
			</form>
		</Modal>
	);
}
