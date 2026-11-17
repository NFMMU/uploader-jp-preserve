import { toString } from "mdast-util-to-string";
import type { Root } from "mdast";
import { unified, type Plugin } from "unified";
import remarkParse from "remark-parse";

export type NoticeHeading = {
	level: number;
	title: string;
	id: string;
};

function slugifyHeading(value: string): string {
	return value
		.toLocaleLowerCase()
		.replace(/[^\p{Letter}\p{Number}\s-]/gu, "")
		.replace(/[\s-]+/g, "-") || "section";
}

function visitHeadings(tree: Root, collect: boolean): NoticeHeading[] {
	const headings: NoticeHeading[] = [];
	const usedIds = new Map<string, number>();

	for (const node of tree.children) {
		if (node.type !== "heading" || (node.depth !== 2 && node.depth !== 3)) continue;

		const title = toString(node);
		const baseId = slugifyHeading(title);
		const count = usedIds.get(baseId) ?? 0;
		usedIds.set(baseId, count + 1);
		const id = count === 0 ? baseId : baseId + "-" + (count + 1);

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

export function remarkNoticeHeadingIds(): Plugin<[], Root> {
	return (tree) => {
		visitHeadings(tree, false);
	};
}
