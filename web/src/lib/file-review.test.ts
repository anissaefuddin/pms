import assert from "node:assert/strict";
import { test } from "node:test";
import { reviewStateFor } from "./file-review.ts";

const file = "f1";
const approval = (id: string, status: "pending" | "approved" | "rejected", version: number, created_at: string, attachment_id = file) =>
  ({ id, status, attachment_id, attachment_version: version, created_at });

test("WA-D08: no review for a version is 'not requested', distinct from 'unavailable'", () => {
  assert.equal(reviewStateFor([], file, 1, "ok").state, "notRequested");
  assert.equal(reviewStateFor(undefined, file, 1, "error").state, "unavailable");
  assert.equal(reviewStateFor(undefined, file, 1, "loading").state, "loading");
});

test("WA-D08: a pending review means in review, and it is tied to its own version", () => {
  const list = [approval("a1", "pending", 2, "2026-11-15T10:00:00Z")];
  assert.deepEqual(reviewStateFor(list, file, 2, "ok"), { state: "inReview", approvalId: "a1" });
  assert.equal(reviewStateFor(list, file, 1, "ok").state, "notRequested");
});

test("WA-D08: a new version does not inherit an older approval", () => {
  const list = [approval("a1", "approved", 1, "2026-11-10T10:00:00Z")];
  assert.equal(reviewStateFor(list, file, 1, "ok").state, "approved");
  assert.equal(reviewStateFor(list, file, 2, "ok").state, "notRequested");
});

test("WA-D08: approved and rejected come from the latest decided record for that version", () => {
  const list = [approval("old", "rejected", 1, "2026-11-10T10:00:00Z"), approval("new", "approved", 1, "2026-11-12T10:00:00Z")];
  assert.deepEqual(reviewStateFor(list, file, 1, "ok"), { state: "approved", approvalId: "new" });
  assert.equal(reviewStateFor([approval("r", "rejected", 1, "2026-11-12T10:00:00Z")], file, 1, "ok").state, "rejected");
});

test("WA-D08: an open review wins over an older decision for the same version", () => {
  const list = [approval("done", "approved", 1, "2026-11-10T10:00:00Z"), approval("open", "pending", 1, "2026-11-11T10:00:00Z")];
  assert.equal(reviewStateFor(list, file, 1, "ok").state, "inReview");
});

test("WA-D08: approvals for other files are ignored", () => {
  assert.equal(reviewStateFor([approval("x", "approved", 1, "2026-11-10T10:00:00Z", "other")], file, 1, "ok").state, "notRequested");
});
