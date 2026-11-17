import assert from "node:assert/strict";
import test from "node:test";
import { createServer } from "vite";

test("SSR notice metadata uses the detail parser and safe defaults", async () => {
	const server = await createServer({
		configFile: false,
		server: { middlewareMode: true, watch: null },
		plugins: [
			{
				name: "notice-fixtures",
				resolveId(id) {
					if (id === "virtual:public-notices") return "\0notice-fixtures";
				},
				load(id) {
					if (id !== "\0notice-fixtures") return;
					return `export default ${JSON.stringify([
						{
							path: "news.md",
							source:
								'---\ntitle: "Article title"\nsummary: "Article summary"\n---\n',
						},
						{
							path: "日本語.md",
							source:
								'---\ndate: "2026-11-17"\n---\n# Heading title\n\nSummary from **body**.',
						},
						{ path: "invalid.md", source: "No front matter" },
					])}`;
				},
			},
		],
	});
	try {
		const { data } = await server.ssrLoadModule("/pages/+data.ts");
		const { metadata } = await data({ urlPathname: "/notices/news" });
		assert.equal(
			metadata.title,
			"Article title - お知らせ - uploader.jp 保全プロジェクト",
		);
		assert.equal(metadata.description, "Article summary");
		assert.equal(metadata.type, "article");
		const image = new URL(metadata.image);
		assert.equal(image.searchParams.get("title"), "Article title");
		assert.equal(image.searchParams.get("description"), "Article summary");
		const fallback = await data({
			urlPathname: `/notices/${encodeURIComponent("日本語")}/`,
		});
		assert.ok(fallback.metadata.title.startsWith("Heading title -"));
		assert.equal(fallback.metadata.description, "Summary from body.");
		for (const urlPathname of [
			"/notices/missing",
			"/notices/invalid",
			"/notices/%ZZ",
			"/notices",
		]) {
			const result = await data({ urlPathname });
			assert.equal(result.metadata.type, "website");
			assert.equal(result.metadata.title, "uploader.jp 保全プロジェクト");
		}
	} finally {
		await server.close();
	}
});
