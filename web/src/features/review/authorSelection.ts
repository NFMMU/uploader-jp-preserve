import type { Obj } from "@/types/common";

/**
 * Keep the selected author available in the select even when the visible
 * author list is refreshed or paginated back to an earlier page.
 */
export function includeSelectedAuthor(
	authors: Obj[],
	selectedAuthor: Obj | null,
): Obj[] {
	if (!selectedAuthor) return authors;
	if (authors.some((author) => String(author.id) === String(selectedAuthor.id))) {
		return authors;
	}
	return [selectedAuthor, ...authors];
}
