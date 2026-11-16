import assert from "node:assert/strict";
import test from "node:test";
import { includeSelectedAuthor } from "./authorSelection.js";

const author = (id) => ({
	id,
	display_name: `Author ${id}`,
	email: `author-${id}@example.jp`,
});

test("retains a selected author missing from a refreshed page", () => {
	const selected = author("author-150");
	const refreshedPage = [author("author-1"), author("author-2")];

	assert.deepEqual(includeSelectedAuthor(refreshedPage, selected), [
		selected,
		...refreshedPage,
	]);
});

test("does not duplicate a selected author already in the refreshed list", () => {
	const selected = author("author-2");
	const refreshedPage = [author("author-1"), selected];

	assert.deepEqual(
		includeSelectedAuthor(refreshedPage, selected),
		refreshedPage,
	);
});

test("returns the refreshed list when no author is selected", () => {
	const refreshedPage = [author("author-1")];

	assert.deepEqual(includeSelectedAuthor(refreshedPage, null), refreshedPage);
});
