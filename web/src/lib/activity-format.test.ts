import assert from "node:assert/strict";
import { test } from "node:test";
import { dayKey, formatChange, groupByDay, showValue } from "./activity-format.ts";

test("D9: a change reads as from → to, and empty values show a dash", () => {
  assert.equal(formatChange({ from: "New", to: "Done" }), "New → Done");
  assert.equal(formatChange({ from: null, to: "Ann" }), "— → Ann");
  assert.equal(formatChange({ to: 5 }), "— → 5");
});

test("D9: long or object values are shortened", () => {
  assert.equal(showValue("x".repeat(80)).length, 58);
  assert.equal(showValue({ a: 1 }), '{"a":1}');
});

test("D9: entries group by the day in the user's time zone", () => {
  // 23:30 UTC on 16 Nov is already 17 Nov in Jakarta (UTC+7).
  assert.equal(dayKey("2026-11-16T23:30:00Z", "Asia/Jakarta"), "2026-11-17");
  assert.equal(dayKey("2026-11-16T23:30:00Z", "UTC"), "2026-11-16");
  const groups = groupByDay([
    { id: 3, created_at: "2026-11-17T08:00:00Z" },
    { id: 2, created_at: "2026-11-17T07:00:00Z" },
    { id: 1, created_at: "2026-11-15T09:00:00Z" },
  ], "UTC");
  assert.deepEqual(groups.map((g) => [g.day, g.items.map((i) => i.id)]), [["2026-11-17", [3, 2]], ["2026-11-15", [1]]]);
});
