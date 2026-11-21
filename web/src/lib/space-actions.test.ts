import assert from "node:assert/strict";
import { test } from "node:test";
import { spaceActions } from "./space-actions.ts";

const shared = (caps: string[], extra: Record<string, unknown> = {}) =>
  ({ id: "s", kind: "shared", archived: false, is_space_admin: false, my_capabilities: caps, ...extra }) as never;

test("a read-only member can open, copy the link and read settings, but not change anything", () => {
  assert.deepEqual(spaceActions(shared(["view"])), ["open", "copy-link", "settings"]);
});

test("an editor can rename, and still cannot manage members or archive", () => {
  assert.deepEqual(spaceActions(shared(["view", "edit"])), ["open", "copy-link", "rename", "settings"]);
});

test("a Full share is not enough to archive: archive needs the explicit Space Admin role", () => {
  const full = spaceActions(shared(["view", "edit", "share", "delete"]));
  assert.ok(full.includes("members"));
  assert.ok(!full.includes("archive"));
});

test("an explicit Space Admin gets archive on a shared Space", () => {
  assert.ok(spaceActions(shared(["view", "edit", "share"], { is_space_admin: true })).includes("archive"));
});

test("a Personal Space offers no rename, members or archive", () => {
  const personal = spaceActions(shared(["view", "edit", "share"], { kind: "personal", is_space_admin: true }));
  assert.deepEqual(personal, ["open", "copy-link", "settings"]);
});

test("an archived Space is read-only: no rename, members or archive", () => {
  const archived = spaceActions(shared(["view", "edit", "share"], { archived: true, is_space_admin: true }));
  assert.deepEqual(archived, ["open", "copy-link", "settings"]);
});
