import type { APIError } from "../types/common";

const apiBase = "/api/v1";

export async function publicAPI<T>(
	path: string,
	token: string,
	options: RequestInit = {},
): Promise<T> {
	const response = await fetch(`${apiBase}${path}`, {
		...options,
		headers: {
			Authorization: "Bearer " + token,
			...(options.body ? { "Content-Type": "application/json" } : {}),
			...options.headers,
		},
	});
	const text = await response.text();
	let data: any = {};
	try {
		data = text ? JSON.parse(text) : {};
	} catch {
		data = { message: text };
	}
	if (!response.ok)
		throw new Error(
			(data as APIError).error ||
				(data as APIError).message ||
				`HTTP ${response.status}`,
		);
	return data as T;
}

export async function downloadExport(
	format: string,
	token: string,
	notify: (s: string) => void,
) {
	try {
		const r = await fetch(`${apiBase}/export?format=${format}`, {
			headers: { Authorization: `Bearer ${token}` },
		});
		if (!r.ok) throw new Error("エクスポートに失敗しました");
		const blob = await r.blob();
		const url = URL.createObjectURL(blob);
		const a = document.createElement("a");
		a.href = url;
		a.download = `archive-catalog.${format}`;
		a.click();
		URL.revokeObjectURL(url);
		notify("カタログを出力しました");
	} catch (e) {
		notify((e as Error).message);
	}
}

export async function downloadRequestIntakeFile(
	id: string,
	token: string,
	notify: (s: string) => void,
	filename: string,
) {
	try {
		const response = await fetch(
			`/api/v1/public-requests/${encodeURIComponent(id)}/intake-file`,
			{ headers: { Authorization: `Bearer ${token}` } },
		);
		if (!response.ok) {
			let detail = "";
			try {
				detail = (await response.json()).error || "";
			} catch {}
			throw new Error(detail || "添付ファイルを取得できませんでした");
		}
		const blob = await response.blob();
		const url = URL.createObjectURL(blob);
		const anchor = document.createElement("a");
		anchor.href = url;
		anchor.download = filename || "intake-file";
		anchor.click();
		window.setTimeout(() => URL.revokeObjectURL(url), 1000);
		notify("添付ファイルを取得しました");
	} catch (e) {
		notify((e as Error).message);
	}
}
