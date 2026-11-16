import assert from "node:assert/strict";
import { test } from "node:test";
import { layoutWeek } from "./calendar-layout.ts";

const day = (value: string) => new Date(`${value}T00:00:00`);
const week = day("2026-11-16"); // Monday
const item = (id: string, start: string, end: string) => ({ id, start: day(start), end: day(end) });

test("C2: a single-day item is one bar in its column", () => {
  const layout = layoutWeek([item("a", "2026-11-18", "2026-11-18")], week);
  assert.deepEqual(layout.bars, [{ id: "a", startCol: 2, endCol: 2, lane: 0, continuesBefore: false, continuesAfter: false }]);
});

test("C2: a multi-day item within one week is a single bar, not one per day", () => {
  const layout = layoutWeek([item("a", "2026-11-17", "2026-11-20")], week);
  assert.equal(layout.bars.length, 1);
  assert.deepEqual([layout.bars[0].startCol, layout.bars[0].endCol], [1, 4]);
});

test("C2: an item crossing the week boundary is clipped and marked as continuing", () => {
  const layout = layoutWeek([item("a", "2026-11-14", "2026-11-18")], week);
  assert.deepEqual(layout.bars[0], { id: "a", startCol: 0, endCol: 2, lane: 0, continuesBefore: true, continuesAfter: false });
  const next = layoutWeek([item("a", "2026-11-14", "2026-11-18")], day("2026-11-23"));
  assert.equal(next.bars.length, 0); // the item ends before the following week
  const spanning = layoutWeek([item("b", "2026-11-19", "2026-12-02")], week);
  assert.deepEqual([spanning.bars[0].continuesAfter, spanning.bars[0].endCol], [true, 6]);
});

test("C2: overlapping items use different lanes and non-overlapping items reuse a lane", () => {
  const layout = layoutWeek([item("a", "2026-11-16", "2026-11-18"), item("b", "2026-11-17", "2026-11-19"), item("c", "2026-11-19", "2026-11-20")], week);
  const lane = Object.fromEntries(layout.bars.map((bar) => [bar.id, bar.lane]));
  assert.notEqual(lane.a, lane.b);
  assert.equal(lane.c, lane.a); // a ends on 18, c starts on 19 and fits back into lane 0
});

test("C2: items beyond the visible lanes are kept for +N, with a count per day", () => {
  const items = [1, 2, 3, 4].map((n) => item(`x${n}`, "2026-11-16", "2026-11-17"));
  const layout = layoutWeek(items, week);
  assert.equal(layout.bars.length, 3);
  assert.deepEqual(layout.hidden.map((entry) => entry.id), ["x4"]);
  assert.deepEqual(layout.overflowByCol.slice(0, 3), [1, 1, 0]);
});

test("C2: a same-day milestone is one bar, and dates outside the week are ignored", () => {
  const layout = layoutWeek([item("m", "2026-11-25", "2026-11-25"), item("far", "2027-01-01", "2027-01-02")], week);
  assert.deepEqual(layout.bars.map((bar) => bar.id), []);
  const inside = layoutWeek([item("m", "2026-11-22", "2026-11-22")], week);
  assert.equal(inside.bars[0].startCol, 6);
});
