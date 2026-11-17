import assert from "node:assert/strict";
import test from "node:test";
import { parseNoticeHeadings } from "./markdownHeadings.ts";

test("heading ids use Markdown text rather than Markdown source syntax", () => {
	const headings = parseNoticeHeadings(
		"## **Bold** and _italic_\n\n### [A link](https://example.com)",
	);
	assert.deepEqual(headings, [
		{ level: 2, title: "Bold and italic", id: "bold-and-italic" },
		{ level: 3, title: "A link", id: "a-link" },
	]);
});

test("duplicate headings receive stable unique ids", () => {
	const headings = parseNoticeHeadings(
		"## Overview\n\n## Overview\n\n### Overview",
	);
	assert.deepEqual(
		headings.map((heading) => heading.id),
		["overview", "overview-2", "overview-3"],
	);
});

test("generated suffixes do not collide with literal heading slugs", () => {
	for (const markdown of [
		"## Overview\n\n## Overview\n\n## Overview 2",
		"## Overview 2\n\n## Overview\n\n## Overview",
	]) {
		const ids = parseNoticeHeadings(markdown).map((heading) => heading.id);
		assert.equal(new Set(ids).size, 3);
	}
	assert.deepEqual(
		parseNoticeHeadings("## Overview\n\n## Overview\n\n## Overview 2").map(
			(heading) => heading.id,
		),
		["overview", "overview-2", "overview-2-2"],
	);
});
