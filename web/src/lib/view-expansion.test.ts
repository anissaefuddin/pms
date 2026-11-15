import assert from "node:assert/strict";
import { test } from "node:test";
import { emptyViewExpansion, viewExpansionFrom } from "./view-expansion.ts";

test("WA-D05: reads only the requested view and ignores corrupt entries", () => {
  const stored = { v1: { rows: ["a", 3, "b"], gantt_collapsed: true }, v2: { rows: ["x"], gantt_collapsed: false } };
  assert.deepEqual(viewExpansionFrom(stored, "v1"), { rows: ["a", "b"], gantt_collapsed: true });
  assert.deepEqual(viewExpansionFrom(stored, "missing"), emptyViewExpansion);
  assert.deepEqual(viewExpansionFrom(null, "v1"), emptyViewExpansion);
  assert.deepEqual(viewExpansionFrom({ v1: "bad" }, "v1"), emptyViewExpansion);
  assert.deepEqual(viewExpansionFrom({ v1: { rows: "nope", gantt_collapsed: "yes" } }, "v1"), emptyViewExpansion);
});
