import assert from "node:assert/strict";
import test from "node:test";
import { isPublished } from "./publicNotices.ts";

test("only explicitly unpublished articles are excluded", () => {
	assert.equal(isPublished('---\ntitle: "Public"\npublished: true\n---\nPublic body'), true);
	assert.equal(isPublished('---\ntitle: "Default is public"\n---\nPublic body'), true);
	assert.equal(isPublished('---\ntitle: "Private"\npublished: false\n---\nPrivate body'), false);
});

test("invalid front matter is not included in public build", () => {
	assert.equal(isPublished("# No front matter"), false);
	assert.equal(isPublished("---\ntitle: [\n---\nMalformed"), false);
	assert.equal(isPublished("---\n- array\n---\nInvalid metadata"), false);
});
