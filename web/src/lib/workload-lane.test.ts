import assert from "node:assert/strict";
import { test } from "node:test";
import { laneCreateBody, laneMinutesPerDay, laneOverCapacity, laneWorkingDays } from "./workload-lane.ts";

// Mon..Sun from index 0: days 5 and 6 are the weekend (capacity 0).
const capacity = [480, 480, 480, 480, 480, 0, 0, 480];

test("W4: the drag range is inclusive, in either direction, and skips non-working days", () => {
  assert.deepEqual(laneWorkingDays(capacity, 0, 2), [0, 1, 2]);
  assert.deepEqual(laneWorkingDays(capacity, 2, 0), [0, 1, 2], "dragging right to left");
  assert.deepEqual(laneWorkingDays(capacity, 3, 7), [3, 4, 7], "the weekend is skipped");
  assert.deepEqual(laneWorkingDays(capacity, 5, 6), [], "a weekend only is nothing to create");
});

test("W2: a day over capacity is reported, the lane is not blocked", () => {
  const person = { capacity, planned: [420, 0, 0, 0, 0, 0, 0, 0], booked: [0, 0, 0, 0, 0, 0, 0, 0] };
  assert.deepEqual(laneOverCapacity(person, [0, 1], laneMinutesPerDay), [], "420 + 60 = 480 is still within capacity");
  person.planned[0] = 430;
  assert.deepEqual(laneOverCapacity(person, [0, 1]), [0], "430 + 60 is over 480 on day 0 only");
  person.booked[1] = 425;
  assert.deepEqual(laneOverCapacity(person, [0, 1]), [0, 1], "booked minutes count toward capacity");
});

test("W1: the create body carries the assignee and a 60 minute per day allocation", () => {
  const body = laneCreateBody({ title: "  Review  ", spaceId: "s1", personId: "p1", startDay: "2026-11-23", dueDay: "2026-11-25" });
  assert.deepEqual(body.assignee_ids, ["p1"]);
  assert.deepEqual(body.assignee_allocations, [{ assignee_id: "p1", allocation: { unit: "minutes_per_day", value: 60 } }]);
  assert.equal(body.title, "Review", "title is trimmed");
  assert.deepEqual(body.locations, [{ space_id: "s1" }]);
});
