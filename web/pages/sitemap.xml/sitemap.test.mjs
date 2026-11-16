import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";

let render;
let now;
let items;
let requests;
const originalFetch = globalThis.fetch;
const originalNow = Date.now;

beforeEach(async () => {
	now = 1_000_000;
	items = [{ id: "artifact", files: [{ id: "file" }] }];
	requests = 0;
	Date.now = () => now;
	globalThis.fetch = async () => {
		requests++;
		return Response.json({ items, total: items.length });
	};
	({ onRenderHtml: render } = await import(
		`./+onRenderHtml.ts?test=${Math.random()}`
	));
});

afterEach(() => {
	globalThis.fetch = originalFetch;
	Date.now = originalNow;
});

async function request(url) {
	const context = { urlOriginal: url, headersResponse: new Headers() };
	await render(context);
	assert.equal(
		context.headersResponse.get("Cache-Control"),
		"public, max-age=300, s-maxage=300",
	);
	return context.content;
}

function locations(xml) {
	return [...xml.matchAll(/<loc>(.*?)<\/loc>/g)].map((match) =>
		match[1].replaceAll("&amp;", "&"),
	);
}

test("published split URLs regenerate after ten minutes without another index request", async () => {
	items[0].files = Array.from({ length: 50_001 }, (_, id) => ({
		id: String(id),
	}));
	const pages = locations(await request("/sitemap.xml"));
	assert.equal(pages.length, 2);
	assert.equal(pages[1], "https://uploaderjp.udonya.moe/sitemap.xml?page=2");
	const first = locations(await request(pages[0]));
	const second = locations(await request(pages[1]));
	assert.equal(first.length, 50_000);
	assert.equal(
		new Set([...first, ...second]).size,
		first.length + second.length,
	);
	assert.equal(requests, 1);
	now += 11 * 60 * 1000;
	assert.deepEqual(locations(await request(pages[1])), second);
	assert.equal(requests, 2);
});

test("split requests work in a fresh process cache, including legacy generations", async () => {
	const xml = await request("/sitemap.xml?page=1&generation=previous-process");
	assert.ok(
		locations(xml).includes("https://uploaderjp.udonya.moe/files/file"),
	);
	assert.equal(requests, 1);
});

test("previously published pages still return XML after the catalog shrinks", async () => {
	items[0].files = Array.from({ length: 50_001 }, (_, id) => ({
		id: String(id),
	}));
	const pages = locations(await request("/sitemap.xml"));
	items = [];
	now += 11 * 60 * 1000;
	const xml = await request(pages[1]);
	assert.match(xml, /<urlset /);
	assert.deepEqual(locations(xml), []);
});

test("invalid pages and API failures still fail without publishing partial results", async () => {
	for (const page of ["-1", "1.5", "NaN"]) {
		await assert.rejects(
			request(`/sitemap.xml?page=${page}`),
			/Invalid sitemap page/,
		);
	}
	globalThis.fetch = async () => new Response(null, { status: 503 });
	await assert.rejects(
		request("/sitemap.xml?page=1"),
		/Artifact API: HTTP 503/,
	);
});
