import assert from "node:assert/strict";
import test from "node:test";
import { parseNotice } from "../notice/parseNotice.ts";

const notice = (metadata, body = "本文") =>
	parseNotice("/notices/example.md", `---\n${metadata}\n---\n\n${body}`);

test("parses YAML strings and a boolean without display quotes", () => {
	const result = notice(`title: "運営: お知らせ"
category: '重要'
date: "2026-11-17"
pinned: true`);
	assert.equal(result.title, "運営: お知らせ");
	assert.equal(result.category, "重要");
	assert.equal(result.date, "2026-11-17");
	assert.equal(result.pinned, true);
	assert.equal(notice("date: 2026-11-17").date, "2026-11-17");
	assert.equal(notice('pinned: "true"').pinned, false);
});

test("preserves body headings when a title is specified", () => {
	assert.equal(
		notice('title: "指定タイトル"', "# 本文の見出し\n\n本文").body,
		"# 本文の見出し\n\n本文",
	);
});

test("removes only a leading heading used as a fallback title", () => {
	const result = notice("title: null", "# 代替タイトル\n\n本文\n\n# 続き");
	assert.equal(result.title, "代替タイトル");
	assert.equal(result.body, "本文\n\n# 続き");
	const laterHeading = notice("{}", "導入\n\n# 後の見出し");
	assert.equal(laterHeading.title, "example");
	assert.equal(laterHeading.body, "導入\n\n# 後の見出し");
});

test("uses safe defaults for missing or incorrectly typed values", () => {
	const result = notice("title: [a, b]\ncategory: 123\ndate: null");
	assert.equal(result.title, "example");
	assert.equal(result.category, "お知らせ");
	assert.equal(result.date, "1970-01-01");
});

test("accepts only real calendar dates in YYYY-MM-DD string form", () => {
	for (const date of [
		'"2026-02-29"',
		'"2026-04-31"',
		'"2026-13-01"',
		'"2026-1-01"',
		'"2026-11-17T00:00:00Z"',
		"123",
		"true",
		"[2026-11-17]",
	]) {
		assert.equal(notice(`date: ${date}`).date, "1970-01-01", date);
	}
	assert.equal(notice('date: "2024-02-29"').date, "2024-02-29");
	const dates = ["2026-01-01", "2026-11-17"].map(
		(date) => notice(`date: '${date}'`).date,
	);
	assert.deepEqual(
		dates.sort((a, b) => b.localeCompare(a)),
		["2026-11-17", "2026-01-01"],
	);
});

test("ignores malformed front matter without throwing", () => {
	for (const source of [
		"# 本文",
		"---\ntitle: [\n---\n本文",
		"---\n- item\n---\n本文",
	]) {
		assert.equal(parseNotice("example.md", source), null);
	}
});
