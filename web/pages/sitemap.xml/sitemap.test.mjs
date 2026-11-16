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

test("URL cursor boundaries keep later pages stable after earlier URLs are deleted and cache expires", async () => {
	items[0].files = Array.from({ length: 50_001 }, (_, id) => ({
		id: String(id).padStart(5, "0"),
	}));
	const pages = locations(await request("/sitemap.xml"));
	assert.equal(pages.length, 2);
	const first = locations(await request(pages[0]));
	const secondBeforeMutation = locations(await request(pages[1]));
	assert.equal(first.length, 50_000);
	assert.equal(secondBeforeMutation.length, 10);
	assert.equal(requests, 3);

	// Remove an entry from the first page and force the ten-minute cache window
	// to elapse. The second page must still use the original URL boundaries.
	items[0].files = items[0].files.filter((file) => file.id !== "00000");
	now += 11 * 60 * 1000;
	const secondAfterMutation = locations(await request(pages[1]));
	assert.deepEqual(secondAfterMutation, secondBeforeMutation);
	assert.equal(requests, 4);
});

test("URL cursor boundaries work across independent module instances", async () => {
	items[0].files = Array.from({ length: 50_001 }, (_, id) => ({
		id: String(id).padStart(5, "0"),
	}));
	const pages = locations(await request("/sitemap.xml"));
	const first = locations(await request(pages[0]));
	items[0].files = items[0].files.filter((file) => file.id !== "00000");

	const freshModule = await import(`./+onRenderHtml.ts?fresh-process=${Math.random()}`);
	const context = {
		urlOriginal: new URL(pages[1]).pathname + new URL(pages[1]).search,
		headersResponse: new Headers(),
	};
	await freshModule.onRenderHtml(context);
	const second = locations(context.content);
	assert.ok(second.includes("https://uploaderjp.udonya.moe/files/50000"));
	assert.equal(new Set([...first, ...second]).size, first.length + second.length);
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
		request("/sitemap.xml?page=1&through=https%3A%2F%2Fuploaderjp.udonya.moe%2Ffiles%2Ffile"),
		/Artifact API: HTTP 503/,
	);
});
