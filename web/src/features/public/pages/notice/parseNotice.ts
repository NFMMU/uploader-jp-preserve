import { parse } from "yaml";

export type Notice = {
	slug: string;
	title: string;
	date: string;
	category: string;
	pinned: boolean;
	body: string;
};

function validDate(value: unknown): value is string {
	if (typeof value !== "string" || !/^\d{4}-\d{2}-\d{2}$/.test(value)) {
		return false;
	}
	const date = new Date(`${value}T00:00:00Z`);
	return (
		!Number.isNaN(date.getTime()) && date.toISOString().slice(0, 10) === value
	);
}

export function parseNotice(path: string, source: string): Notice | null {
	const match = source.match(
		/^---[ \t]*\r?\n([\s\S]*?)\r?\n---[ \t]*(?:\r?\n|$)/,
	);
	if (!match) return null;
	let metadata: unknown;
	try {
		metadata = parse(match[1]);
	} catch {
		return null;
	}
	if (!metadata || typeof metadata !== "object" || Array.isArray(metadata)) {
		return null;
	}
	const fields = metadata as Record<string, unknown>;
	const text = (value: unknown) =>
		typeof value === "string" ? value.trim() : "";
	const body = source.slice(match[0].length).trim();
	const slug = path.split("/").pop()?.replace(/\.md$/, "") || path;
	const specifiedTitle = text(fields.title);
	const heading = body.match(/^#\s+([^\r\n]+)(?:\r?\n|$)/);
	return {
		slug,
		title: specifiedTitle || heading?.[1] || slug,
		date: validDate(fields.date) ? fields.date : "1970-01-01",
		category: text(fields.category) || "お知らせ",
		pinned: fields.pinned === true,
		body:
			!specifiedTitle && heading ? body.slice(heading[0].length).trim() : body,
	};
}
