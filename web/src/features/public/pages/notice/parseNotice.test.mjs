import assert from "node:assert/strict";
import test from "node:test";
import { parseNotice } from "./parseNotice.ts";

const notice = (metadata, body = "本文") =>
	parseNotice("/notices/example.md", `---\n${metadata}\n---\n\n${body}`);

test("parses tags, optional metadata and publication state", () => {
	const result = notice(`title: "運営からのお知らせ"
date: "2026-11-17"
updated: "2026-11-18"
summary: "記事の概要"
tags:
  - 重要
  - メンテナンス
  - 重要
pinned: true`);
	assert.equal(result.title, "運営からのお知らせ");
	assert.deepEqual(result.tags, ["重要", "メンテナンス"]);
	assert.equal(result.date, "2026-11-17");
	assert.equal(result.updatedAt, "2026-11-18");
	assert.equal(result.summary, "記事の概要");
	assert.equal(result.pinned, true);
	assert.equal(result.published, true);
});

test("hides explicitly unpublished notices", () => {
	assert.equal(notice("published: false").published, false);
	assert.equal(notice('published: "false"').published, true);
});

test("preserves body headings when a title is specified", () => {
	assert.equal(
		notice('title: "指定タイトル"', "# 本文の見出し\n\n本文").body,
		"# 本文の見出し\n\n本文",
	);
});

test("uses a leading heading as title when metadata title is absent", () => {
	const result = notice("title: null", "# 代替タイトル\n\n本文\n\n## 続き");
	assert.equal(result.title, "代替タイトル");
	assert.equal(result.body, "本文\n\n## 続き");
});

test("uses safe defaults for missing or incorrectly typed values", () => {
	const result = notice("title: [a, b]\ntags: invalid\ndate: null");
	assert.equal(result.title, "example");
	assert.deepEqual(result.tags, []);
	assert.equal(result.date, "1970-01-01");
	assert.equal(result.summary, "本文");
});

test("accepts only real calendar dates in YYYY-MM-DD string form", () => {
	for (const date of ['"2026-02-29"', '"2026-04-31"', '"2026-13-01"', '"2026-1-01"', "123", "true"]) {
		assert.equal(notice(`date: ${date}`).date, "1970-01-01", date);
	}
	assert.equal(notice('date: "2024-02-29"').date, "2024-02-29");
});

test("ignores malformed front matter without throwing", () => {
	for (const source of ["# 本文", "---\ntitle: [\n---\n本文", "---\n- item\n---\n本文"]) {
		assert.equal(parseNotice("example.md", source), null);
	}
});
