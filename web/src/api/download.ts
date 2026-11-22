import { publicAPI } from "./admin";

type SaveFilePicker = (options: { suggestedName?: string }) => Promise<{
	createWritable: () => Promise<WritableStream<Uint8Array>>;
}>;

type DownloadTarget = {
	id: string;
	original_filename?: string;
};

/**
 * Downloads a catalog file through the authenticated reviewer endpoint.
 * The file picker is opened before the download job is created because
 * browsers only allow it from a user gesture.
 */
export async function saveCatalogFileContent(
	file: DownloadTarget,
	onMessage?: (message: string) => void,
): Promise<void> {
	const message = (text: string) => onMessage?.(text);
	let writable: WritableStream<Uint8Array> | undefined;
	try {
		const pickerWindow = window as Window & {
			showSaveFilePicker?: SaveFilePicker;
		};
		if (!pickerWindow.showSaveFilePicker)
			throw new Error(
				"このブラウザーはストリーム保存に対応していません。HTTPS接続でChromeまたはEdgeをご利用ください。",
			);
		message("準備中…");
		const handle = await pickerWindow.showSaveFilePicker({
			suggestedName: file.original_filename,
		});
		const created = await publicAPI<{
			job: { id: string };
			access_token: string;
		}>(`/catalog/files/${file.id}/downloads`, { method: "POST" });
		let ready = false;
		for (let attempt = 0; attempt < 90; attempt += 1) {
			await new Promise((resolve) => window.setTimeout(resolve, 2000));
			const status = await publicAPI<{ status: string; last_error?: string }>(
				`/catalog/downloads/${created.job.id}`,
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
			message("準備中…");
		}
		if (!ready)
			throw new Error(
				"準備に時間がかかっています。時間をおいて再度お試しください。",
			);
		writable = await handle.createWritable();
		const response = await fetch(
			`/api/v1/catalog/downloads/${created.job.id}/content`,
			{
				headers: {
					"Download-Authorization": `Bearer ${created.access_token}`,
				},
				credentials: "same-origin",
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
		message("保存中…");
		await response.body.pipeTo(writable);
		message("ファイルを保存しました");
	} catch (e) {
		if (writable) await writable.abort().catch(() => {});
		message(
			e instanceof DOMException && e.name === "AbortError"
				? "ダウンロードをキャンセルしました"
				: e instanceof Error
					? e.message
					: "エラーが発生しました",
		);
		throw e;
	}
}
