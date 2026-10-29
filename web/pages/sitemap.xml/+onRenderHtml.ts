import type { PageContextServer } from "vike/types";

const SITE_URL = "https://uploaderjp.udonya.moe";
const API_ORIGIN = (
  process.env.API_ORIGIN || "http://api:8080"
).replace(/\/$/, "");

const STATIC_PATHS = [
  "/",
  "/requests",
  "/preservation-requests",
  "/redistribution-support",
  "/publisher-support",
  "/relocation-info",
  "/participate",
  "/terms",
  "/privacy",
  "/disclaimer",
];

const PAGE_SIZE = 100;
const MAX_URLS = 50_000;

type Artifact = { id: string };
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

async function getArtifactIds(): Promise<string[]> {
  const ids: string[] = [];
  let total: number | undefined;

  for (let page = 1; page <= MAX_URLS / PAGE_SIZE; page++) {
    const url = new URL(`${API_ORIGIN}/api/v1/public/artifacts`);
    url.searchParams.set("page", String(page));
    url.searchParams.set("limit", String(PAGE_SIZE));

    const response = await fetch(url, {
      headers: { Accept: "application/json" },
      signal: AbortSignal.timeout(10_000),
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
      if (total > MAX_URLS - STATIC_PATHS.length) {
        throw new Error("Sitemap URL limit exceeded");
      }
    }

    for (const item of body.items) {
      if (typeof item.id === "string" && item.id.length > 0) {
        ids.push(item.id);
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

  if (total !== undefined && ids.length < total) {
    throw new Error("Failed to retrieve all public artifacts");
  }

  return [...new Set(ids)];
}

export async function onRenderHtml(
  pageContext: PageContextServer,
) {
  const artifactIds = await getArtifactIds();

  const urls = [
    ...STATIC_PATHS.map((path) => `${SITE_URL}${path}`),
    ...artifactIds.map(
      (id) =>
        `${SITE_URL}/artifacts/${encodeURIComponent(id)}`,
    ),
  ];

  if (urls.length > MAX_URLS) {
    throw new Error("Sitemap URL limit exceeded");
  }

  const entries = urls
    .map(
      (url) =>
        `  <url>\n    <loc>${escapeXml(url)}</loc>\n  </url>`,
    )
    .join("\n");

  pageContext.content = [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    entries,
    "</urlset>",
    "",
  ].join("\n");

  pageContext.headersResponse.set(
    "Cache-Control",
    "public, max-age=300, s-maxage=300",
  );
}