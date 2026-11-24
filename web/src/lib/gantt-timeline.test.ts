import assert from "node:assert/strict";
import { test } from "node:test";
import { dayCellWidth, dayIndexAt, dayRange, dragThreshold, isDragStep } from "./gantt-timeline.ts";

test("G4: a pointer maps to the day under it, from the timeline's visible left edge", () => {
  assert.equal(dayIndexAt(100, 100, 0), 0, "the left edge is day 0");
  assert.equal(dayIndexAt(100 + dayCellWidth * 3 + 5, 100, 0), 3);
  assert.equal(dayIndexAt(100 + dayCellWidth * 2, 100, dayCellWidth * 10), 12, "scrolling moves the origin");
  assert.equal(dayIndexAt(99, 100, 0), null, "left of the origin is not a day");
});

test("G4: a move under the threshold is a click, not a drag", () => {
  assert.equal(isDragStep(50, 50 + dragThreshold - 1), false);
  assert.equal(isDragStep(50, 50 + dragThreshold), true);
  assert.equal(isDragStep(50, 20), true, "dragging left counts too");
});

test("G1: the range is inclusive and order-independent (Fri to Mon keeps the weekend inside)", () => {
  assert.deepEqual(dayRange(4, 0), { first: 0, last: 4 });
  assert.deepEqual(dayRange(3, 3), { first: 3, last: 3 }, "a double-click or a single day is one day");
});
