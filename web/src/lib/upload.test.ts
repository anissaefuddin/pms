import assert from "node:assert/strict";
import { test } from "node:test";
import { progressPercent } from "./upload-progress.ts";

test("D-05: progress is a real percentage of the bytes sent and never exceeds 100", () => {
  assert.equal(progressPercent(0, 1000), 0);
  assert.equal(progressPercent(250, 1000), 25);
  assert.equal(progressPercent(1000, 1000), 100);
  assert.equal(progressPercent(1200, 1000), 100);
  assert.equal(progressPercent(10, 0), 0); // unknown total: no fake percentage
});
