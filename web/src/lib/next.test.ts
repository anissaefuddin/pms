import assert from "node:assert/strict";
import { test } from "node:test";
import { safeNext } from "./next.ts";

test("accepts internal paths", () => {
  assert.equal(safeNext("/settings"), "/settings");
  assert.equal(safeNext("/a/b?x=1#h"), "/a/b?x=1#h");
});

test("rejects anything that could leave the app", () => {
  for (const bad of ["https://evil.test", "//evil.test", "/\\evil.test", "javascript:alert(1)", "evil", "", null, undefined, "/\nfoo"]) {
    assert.equal(safeNext(bad as string), "/", String(bad));
  }
});
