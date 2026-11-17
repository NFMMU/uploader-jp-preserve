import { unified } from "unified";
import remarkParse from "remark-parse";
import { toString } from "mdast-util-to-string";

export type NoticeHeading = {
	level: number;
	title: string;
	id: string;
};

type MarkdownNode = {
	type?: string;
	depth?: number;
	children?: MarkdownNode[];
	data?: {
		hProperties?: Record<string, unknown>;
	};
};

function slugifyHeading(value: string): string {
	return value
		.toLocaleLowerCase()
		.replace(/[^\p{Letter}\p{Number}\s-]/gu, "")
		.replace(/[\s-]+/g, "-") || "section";
}

function visitHeadings(tree: MarkdownNode, collect: boolean): NoticeHeading[] {
	const headings: NoticeHeading[] = [];
	const usedIds = new Map<string, number>();

	for (const node of tree.children ?? []) {
		if (node.type !== "heading" || (node.depth !== 2 && node.depth !== 3)) continue;

		const title = toString(node as never);
		const baseId = slugifyHeading(title);
		const count = usedIds.get(baseId) ?? 0;
		usedIds.set(baseId, count + 1);
		const id = count === 0 ? baseId : baseId + "-" + (count + 1);

		node.data ??= {};
		node.data.hProperties ??= {};
		node.data.hProperties.id = id;

		if (collect) headings.push({ level: node.depth, title, id });
	}

	return headings;
}

export function parseNoticeHeadings(markdown: string): NoticeHeading[] {
	const tree = unified().use(remarkParse).parse(markdown) as unknown as MarkdownNode;
	return visitHeadings(tree, true);
}

export function remarkNoticeHeadingIds() {
	return (tree: MarkdownNode) => {
		visitHeadings(tree, false);
	};
}
