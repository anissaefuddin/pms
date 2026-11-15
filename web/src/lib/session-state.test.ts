import assert from "node:assert/strict";
import { test } from "node:test";
import { clearSessionState, lastSpaceKey } from "./session-state.ts";

function fakeStorage(entries: Record<string, string>) {
  const map = new Map(Object.entries(entries));
  return { removeItem: (key: string) => { map.delete(key); }, getItem: (key: string) => map.get(key) ?? null, map };
}

test("WA-D06: sign-out clears only this user's last Space and keeps saved preferences", () => {
  const storage = fakeStorage({
    [lastSpaceKey("alice")]: "space-1", [lastSpaceKey("bob")]: "space-2",
    "pms:sidebar-width": "270", "pms:sidebar-spaces:alice": "[]",
  });
  clearSessionState(storage, "alice");
  assert.equal(storage.getItem(lastSpaceKey("alice")), null);
  assert.equal(storage.getItem(lastSpaceKey("bob")), "space-2");
  assert.equal(storage.getItem("pms:sidebar-width"), "270");
  assert.equal(storage.getItem("pms:sidebar-spaces:alice"), "[]");
});

test("WA-D06: blocked or missing storage does not throw", () => {
  assert.doesNotThrow(() => clearSessionState(null, "alice"));
  assert.doesNotThrow(() => clearSessionState({ removeItem: () => { throw new Error("blocked"); } }, "alice"));
});
