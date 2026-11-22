import assert from "node:assert/strict";
import { test } from "node:test";
import { clampedWidth, clampWidths } from "./view-widths.ts";

test("saved view widths are migrated to their current minimums", () => {
  assert.deepEqual(clampWidths({ name: 64, status: 180 }, { name: 240, status: 112 }), { name: 240, status: 180 });
  assert.equal(clampedWidth("name", {}, { name: 200 }, { name: 240 }), 240);
});
