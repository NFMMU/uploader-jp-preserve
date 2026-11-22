import type { PageContextServer } from "vike/types";

const SITE_URL = "https://uploaderjp.udonya.moe";
const API_ORIGIN = (process.env.API_ORIGIN || "http://api:8080").replace(
	/\/$/,
	"",
);

const STATIC_PATHS = [
	"/",
	"/requests",
	"/preservation-requests",
	"/preservation-flow",
	"/copyright-holders",
	//"/redistribution-support",
	"/publisher-support",
	"/relocation-info",
	//"/participate",
	"/terms",
	"/privacy",
	"/disclaimer",
];

const PAGE_SIZE = 100;
const MAX_URLS = 50_000;
const MAX_ARTIFACTS = 1_000_000;
const SNAPSHOT_TTL_MS = 10 * 60 * 1000;

type SitemapSnapshot = { urls: string[]; expiresAt: number };
let snapshot: SitemapSnapshot | undefined;

type Artifact = { id: string; files?: { id: string }[] };
type ApiResponse = {
	items?: Artifact[];
	total?: number;
};

function escapeXml(value: string) {
	return value
		.replace(/&/g, "&amp;")
		.replace(/</g, "&lt;")
		.replace(/>/g, "&gt;")
		.replace(/"/g, "&quot;")
		.replace(/'/g, "&apos;");
}

async function getPublicUrls(): Promise<string[]> {
	const artifactIds: string[] = [];
	const fileIds: string[] = [];
	let total: number | undefined;

	for (let page = 1; page <= MAX_ARTIFACTS / PAGE_SIZE; page++) {
		const url = new URL(`${API_ORIGIN}/api/v1/public/artifacts`);
		url.searchParams.set("page", String(page));
		url.searchParams.set("limit", String(PAGE_SIZE));

		const response = await fetch(url, {
			headers: { Accept: "application/json" },
			signal: AbortSignal.timeout(10_000),
			credentials: "same-origin",
		});

		if (!response.ok) {
			throw new Error(`Artifact API: HTTP ${response.status}`);
		}

		const body = (await response.json()) as ApiResponse;
		if (!Array.isArray(body.items)) {
			throw new Error("Artifact API response has no items array");
		}
		if (typeof body.total === "number") {
			total = body.total;
			if (total > MAX_ARTIFACTS) {
				throw new Error(
					"Public artifact count exceeds sitemap collection limit",
				);
			}
		}

		for (const item of body.items) {
			if (typeof item.id !== "string" || item.id.length === 0) continue;
			artifactIds.push(item.id);
			for (const file of item.files || []) {
				if (typeof file.id === "string" && file.id.length > 0) {
					fileIds.push(file.id);
				}
			}
		}

		if (
			body.items.length === 0 ||
			(total !== undefined && page * PAGE_SIZE >= total) ||
			(total === undefined && body.items.length < PAGE_SIZE)
		) {
			break;
		}
	}

	if (total !== undefined && artifactIds.length < total) {
		throw new Error("Failed to retrieve all public artifacts");
	}

	return [
		...new Set([
			...STATIC_PATHS.map((path) => `${SITE_URL}${path}`),
			...Array.from(
				new Set(artifactIds),
				(id) => `${SITE_URL}/artifacts/${encodeURIComponent(id)}`,
			),
			...Array.from(
				new Set(fileIds),
				(id) => `${SITE_URL}/files/${encodeURIComponent(id)}`,
			),
		]),
	].sort();
}

function renderUrlset(urls: string[]) {
	const entries = urls
		.map((url) => `  <url>\n    <loc>${escapeXml(url)}</loc>\n  </url>`)
		.join("\n");
	return [
		'<?xml version="1.0" encoding="UTF-8"?>',
		'<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
		entries,
		"</urlset>",
		"",
	].join("\n");
}

function renderSitemapIndex(urls: string[]) {
	const pageCount = Math.max(1, Math.ceil(urls.length / MAX_URLS));
	const entries = Array.from({ length: pageCount }, (_, index) => {
		const start = index * MAX_URLS;
		const end = Math.min(start + MAX_URLS, urls.length);
		const params = new URLSearchParams({
			page: String(index + 1),
			through: urls[end - 1] || "",
		});
		if (start > 0) params.set("after", urls[start - 1]);
		const loc = `${SITE_URL}/sitemap.xml?${params.toString()}`;
		return `  <sitemap>\n    <loc>${escapeXml(loc)}</loc>\n  </sitemap>`;
	}).join("\n");
	return [
		'<?xml version="1.0" encoding="UTF-8"?>',
		'<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
		entries,
		"</sitemapindex>",
		"",
	].join("\n");
}

export async function onRenderHtml(pageContext: PageContextServer) {
	const requestURL = new URL(pageContext.urlOriginal, SITE_URL);
	const requestedPage = Number(requestURL.searchParams.get("page") || "0");
	if (!Number.isInteger(requestedPage) || requestedPage < 0) {
		throw new Error("Invalid sitemap page");
	}

	// The index publishes immutable URL boundaries. Page requests use those
	// boundaries against a freshly collected list, so process-local cache expiry
	// and requests routed to another process cannot shift page membership.
	if (requestedPage === 0) {
		const urls = [...new Set(await getPublicUrls())].sort();
		pageContext.content = renderSitemapIndex(urls);
	} else {
		const after = requestURL.searchParams.get("after") || "";
		const through = requestURL.searchParams.get("through");
		if (through === null || (after && after >= through)) {
			throw new Error("Invalid sitemap page boundaries");
		}

		const urls = [...new Set(await getPublicUrls())].sort();
		const pageURLs = urls.filter((url) => url > after && url <= through);
		if (pageURLs.length > MAX_URLS) {
			throw new Error(
				"Sitemap page URL limit exceeded; refresh the sitemap index",
			);
		}
		pageContext.content = renderUrlset(pageURLs);
	}

	pageContext.headersResponse.set(
		"Cache-Control",
		"public, max-age=300, s-maxage=300",
	);
}
