import { describe, expect, it } from "vitest";
import { includeSelectedAuthor } from "./authorSelection";

const author = (id: string) => ({
	id,
	display_name: `Author ${id}`,
	email: `author-${id}@example.jp`,
});

describe("includeSelectedAuthor", () => {
	it("keeps a selected author when the refreshed page does not contain them", () => {
		const selected = author("author-150");
		const refreshedPage = [author("author-1"), author("author-2")];

		expect(includeSelectedAuthor(refreshedPage, selected)).toEqual([
			selected,
			...refreshedPage,
		]);
	});

	it("does not duplicate the selected author when it is already in the list", () => {
		const selected = author("author-2");
		const refreshedPage = [author("author-1"), selected];

		expect(includeSelectedAuthor(refreshedPage, selected)).toEqual(refreshedPage);
	});

	it("returns the refreshed list when there is no selected author", () => {
		const refreshedPage = [author("author-1")];

		expect(includeSelectedAuthor(refreshedPage, null)).toEqual(refreshedPage);
	});
});
