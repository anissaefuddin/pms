import assert from "node:assert/strict";
import test from "node:test";
import { moveId, statusDotClass } from "./status-colors.ts";

test("moveId swaps neighbours and stops at the edges", () => {
  assert.deepEqual(moveId(["a", "b", "c"], "b", -1), ["b", "a", "c"]);
  assert.deepEqual(moveId(["a", "b", "c"], "b", 1), ["a", "c", "b"]);
  assert.deepEqual(moveId(["a", "b"], "a", -1), ["a", "b"]);
  assert.deepEqual(moveId(["a", "b"], "b", 1), ["a", "b"]);
});

test("unknown color tokens fall back to slate", () => {
  assert.equal(statusDotClass("red"), "bg-red-500");
  assert.equal(statusDotClass("nope"), "bg-slate-500");
});
