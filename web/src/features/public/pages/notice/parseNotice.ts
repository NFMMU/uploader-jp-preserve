import { parse } from "yaml";

export type Notice = {
	slug: string;
	title: string;
	date: string;
	updatedAt?: string;
	tags: string[];
	pinned: boolean;
	published: boolean;
	summary: string;
	body: string;
};

export function isValidNoticeDate(value: unknown): value is string {
	if (typeof value !== "string" || !/^\d{4}-\d{2}-\d{2}$/.test(value))
		return false;
	const date = new Date(value + "T00:00:00Z");
	return (
		!Number.isNaN(date.getTime()) && date.toISOString().slice(0, 10) === value
	);
}

function plainText(markdown: string): string {
	return markdown
		.replace(/^\s*#+\s+/gm, "")
		.replace(/!\[([^\]]*)\]\([^)]*\)/g, "$1")
		.replace(/\[([^\]]+)\]\([^)]*\)/g, "$1")
		.replace(/[*_`>~]/g, "")
		.replace(/\s+/g, " ")
		.trim();
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
	if (!metadata || typeof metadata !== "object" || Array.isArray(metadata))
		return null;

	const fields = metadata as Record<string, unknown>;
	const text = (value: unknown) =>
		typeof value === "string" ? value.trim() : "";
	const body = source.slice(match[0].length).trim();
	const slug = path.split("/").pop()?.replace(/\.md$/, "") || path;
	const specifiedTitle = text(fields.title);
	const heading = body.match(/^#\s+([^\r\n]+)(?:\r?\n|$)/);
	const title = specifiedTitle || heading?.[1] || slug;
	const content =
		!specifiedTitle && heading ? body.slice(heading[0].length).trim() : body;
	const rawTags = Array.isArray(fields.tags) ? fields.tags : [];
	const tags = [
		...new Set(
			rawTags
				.filter((tag): tag is string => typeof tag === "string")
				.map((tag) => tag.trim())
				.filter(Boolean),
		),
	];

	return {
		slug,
		title,
		date: isValidNoticeDate(fields.date) ? fields.date : "1970-01-01",
		updatedAt: isValidNoticeDate(fields.updated) ? fields.updated : undefined,
		tags,
		pinned: fields.pinned === true,
		published: fields.published !== false,
		summary: text(fields.summary) || plainText(content).slice(0, 180),
		body: content,
	};
}
