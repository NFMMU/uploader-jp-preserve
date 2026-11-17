import type { Root } from "mdast";
import { toString as mdToString } from "mdast-util-to-string";
import remarkParse from "remark-parse";
import { type Plugin, unified } from "unified";

export type NoticeHeading = {
	level: number;
	title: string;
	id: string;
};

function slugifyHeading(value: string): string {
	return (
		value
			.toLocaleLowerCase()
			.replace(/[^\p{Letter}\p{Number}\s-]/gu, "")
			.replace(/[\s-]+/g, "-") || "section"
	);
}

function visitHeadings(tree: Root, collect: boolean): NoticeHeading[] {
	const headings: NoticeHeading[] = [];
	const usedIds = new Set<string>();

	for (const node of tree.children) {
		if (node.type !== "heading" || (node.depth !== 2 && node.depth !== 3))
			continue;

		const title = mdToString(node);
		const baseId = slugifyHeading(title);
		let id = baseId;
		let suffix = 2;
		while (usedIds.has(id)) id = baseId + "-" + suffix++;
		usedIds.add(id);

		const data = node.data as
			| (typeof node.data & { hProperties?: Record<string, unknown> })
			| undefined;
		node.data = {
			...data,
			hProperties: { ...data?.hProperties, id },
		};

		if (collect) headings.push({ level: node.depth, title, id });
	}

	return headings;
}

export function parseNoticeHeadings(markdown: string): NoticeHeading[] {
	const tree = unified().use(remarkParse).parse(markdown);
	return visitHeadings(tree, true);
}

export const remarkNoticeHeadingIds: Plugin<[], Root> = () => (tree) => {
	visitHeadings(tree, false);
};
