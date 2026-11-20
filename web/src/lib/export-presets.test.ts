import { test } from "node:test";
import assert from "node:assert/strict";
import { loadPresets, presetKey, removePreset, upsertPreset } from "./export-presets.ts";

// A minimal localStorage, so the module runs under node:test.
const store = new Map<string, string>();
Object.assign(globalThis, { localStorage: { getItem: (k: string) => store.get(k) ?? null, setItem: (k: string, v: string) => void store.set(k, v), removeItem: (k: string) => void store.delete(k) } });

test("presets are namespaced by user, Space, and view", () => {
  store.clear();
  const mine = presetKey("u1", "s1", "v1");
  assert.notEqual(mine, presetKey("u2", "s1", "v1"), "another user");
  assert.notEqual(mine, presetKey("u1", "s2", "v1"), "another workspace");
  assert.notEqual(mine, presetKey("u1", "s1", "v2"), "another view");
  upsertPreset(mine, { name: "Lean", scope: "", columns: ["status"] });
  assert.deepEqual(loadPresets(presetKey("u2", "s1", "v1")), []);
  assert.deepEqual(loadPresets(mine).map((p) => p.name), ["Lean"]);
});

test("saving a name again replaces the preset, and delete removes it", () => {
  store.clear();
  const key = presetKey("u1", "s1", "item");
  upsertPreset(key, { name: "Lean", scope: "", columns: ["status"] });
  upsertPreset(key, { name: "Lean", scope: "active", columns: ["due_date"] });
  assert.deepEqual(loadPresets(key), [{ name: "Lean", scope: "active", columns: ["due_date"] }]);
  assert.deepEqual(removePreset(key, "Lean"), []);
});

test("corrupt or foreign storage is ignored instead of breaking the dialog", () => {
  store.clear();
  const key = presetKey("u1", "s1", "item");
  store.set(key, "{not json");
  assert.deepEqual(loadPresets(key), []);
  store.set(key, JSON.stringify([{ name: 1 }, { name: "ok", scope: "", columns: [] }]));
  assert.deepEqual(loadPresets(key).map((p) => p.name), ["ok"]);
});
