import assert from "node:assert/strict";
import test from "node:test";
import { parseNoticeHeadings } from "./markdownHeadings.ts";

test("heading ids use Markdown text rather than Markdown source syntax", () => {
	const headings = parseNoticeHeadings("## **Bold** and _italic_\n\n### [A link](https://example.com)");
	assert.deepEqual(headings, [
		{ level: 2, title: "Bold and italic", id: "bold-and-italic" },
		{ level: 3, title: "A link", id: "a-link" },
	]);
});

test("duplicate headings receive stable unique ids", () => {
	const headings = parseNoticeHeadings("## Overview\n\n## Overview\n\n### Overview");
	assert.deepEqual(headings.map((heading) => heading.id), [
		"overview", "overview-2", "overview-3",
	]);
});
