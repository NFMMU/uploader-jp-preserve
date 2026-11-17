import { readdirSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import { parse } from "yaml";
import type { Plugin } from "vite";

const publicNoticesId = "virtual:public-notices";
const resolvedPublicNoticesId = "\0" + publicNoticesId;

export function isPublished(source: string): boolean {
	const match = source.match(/^---[ \t]*\r?\n([\s\S]*?)\r?\n---[ \t]*(?:\r?\n|$)/);
	if (!match) return false;
	try {
		const metadata: unknown = parse(match[1]);
		if (!metadata || typeof metadata !== "object" || Array.isArray(metadata)) return false;
		return (metadata as Record<string, unknown>).published !== false;
	} catch {
		return false;
	}
}

export function publicNoticesPlugin(): Plugin {
	let noticesDirectory = "";

	return {
		name: "public-notices",
		enforce: "pre",
		configResolved(config) {
			noticesDirectory = resolve(config.root, "src/content/notices");
		},
		resolveId(id) {
			if (id === publicNoticesId) return resolvedPublicNoticesId;
		},
		load(id) {
			if (id !== resolvedPublicNoticesId) return;
			const files = readdirSync(noticesDirectory, { withFileTypes: true })
				.filter((entry) => entry.isFile() && entry.name.endsWith(".md"))
				.map((entry) => resolve(noticesDirectory, entry.name))
				.sort();
			const publishedFiles = files.filter((file) => isPublished(readFileSync(file, "utf8")));
			const imports = publishedFiles.map((file, index) =>
				"import notice" + index + " from " + JSON.stringify(file + "?raw") + ";",
			);
			const entries = publishedFiles.map((file, index) =>
				"  { path: " + JSON.stringify(file) + ", source: notice" + index + " },",
			);
			return [...imports, "export default [", ...entries, "];"].join("\n");
		},
		handleHotUpdate({ file, server }) {
			if (!file.startsWith(noticesDirectory) || !file.endsWith(".md")) return;
			const virtualModule = server.moduleGraph.getModuleById(resolvedPublicNoticesId);
			if (virtualModule) {
				server.moduleGraph.invalidateModule(virtualModule);
				return [virtualModule];
			}
			return;
		},
	};
}
