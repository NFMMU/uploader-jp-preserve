import assert from "node:assert/strict";
import test from "node:test";
import { isPublished } from "./publicNotices.ts";

test("only explicitly unpublished articles are excluded", () => {
	assert.equal(
		isPublished('---\ntitle: "Public"\npublished: true\n---\nPublic body'),
		true,
	);
	assert.equal(
		isPublished('---\ntitle: "Default is public"\n---\nPublic body'),
		true,
	);
	assert.equal(
		isPublished('---\ntitle: "Private"\npublished: false\n---\nPrivate body'),
		false,
	);
});

test("invalid front matter is not included in public build", () => {
	assert.equal(isPublished("# No front matter"), false);
	assert.equal(isPublished("---\ntitle: [\n---\nMalformed"), false);
	assert.equal(isPublished("---\n- array\n---\nInvalid metadata"), false);
});

test("create and update invalidate the virtual module in each environment", async () => {
	const { publicNoticesPlugin } = await import("./publicNotices.ts");
	const plugin = publicNoticesPlugin();
	plugin.configResolved({ root: "/project" });
	for (const type of ["create", "update", "delete"]) {
		for (const name of ["client", "ssr"]) {
			const virtualModule = { id: "\0virtual:public-notices" };
			const invalidated = [];
			const context = {
				environment: {
					name,
					moduleGraph: {
						getModuleById: (id) => {
							assert.equal(id, virtualModule.id);
							return virtualModule;
						},
						invalidateModule: (module) => invalidated.push(module),
					},
				},
			};
			assert.deepEqual(
				plugin.hotUpdate.call(context, {
					type,
					file: "/project/src/content/notices/new.md",
				}),
				[virtualModule],
			);
			assert.deepEqual(invalidated, [virtualModule]);
			assert.equal(
				plugin.hotUpdate.call(context, {
					type,
					file: "/project/src/content/notices-other/new.md",
				}),
				undefined,
			);
		}
	}
});
