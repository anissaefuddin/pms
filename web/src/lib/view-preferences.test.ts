import assert from "node:assert/strict";
import { test } from "node:test";
import { arrangeViews, hideView, moveView, moveViewBefore, restoreView, viewPrefsFrom, emptyViewPrefs } from "./view-preferences.ts";

const view = (n: number, extra: Partial<{ is_default: boolean }> = {}) => ({ id: `v${n}`, is_default: n === 1, ...extra });
const seven = [1, 2, 3, 4, 5, 6, 7].map((n) => view(n));

test("WA-D07: with more than six views, six tabs stay visible and the rest are overflow", () => {
  const { visible, overflow, hidden } = arrangeViews(seven, emptyViewPrefs);
  assert.equal(visible.length, 6);
  assert.deepEqual(overflow.map((v) => v.id), ["v7"]);
  assert.deepEqual(hidden, []);
});

test("WA-D07: a hidden view is not a tab, but it is kept and can be restored", () => {
  const hidden = hideView(emptyViewPrefs, "v2", seven);
  const arranged = arrangeViews(seven, hidden);
  assert.deepEqual(arranged.hidden.map((v) => v.id), ["v2"]);
  assert.ok(!arranged.visible.some((v) => v.id === "v2"));
  assert.equal(arranged.visible.length, 6); // six regular views remain, so six tabs and no overflow
  assert.equal(arranged.overflow.length, 0);
  const restored = arrangeViews(seven, restoreView(hidden, "v2"));
  assert.equal(restored.hidden.length, 0);
  assert.ok(restored.visible.some((v) => v.id === "v2") || restored.overflow.some((v) => v.id === "v2"));
});

test("WA-D07: the effective default view cannot be hidden", () => {
  const prefs = hideView(emptyViewPrefs, "v1", seven);
  assert.deepEqual(prefs.hidden, []);
  const forced = arrangeViews(seven, { hidden: ["v1"], order: [] });
  assert.ok(forced.visible.some((v) => v.id === "v1"));
});

test("WA-D07: a stale preference for a deleted view is ignored, not restored", () => {
  const arranged = arrangeViews(seven, { hidden: ["gone"], order: ["gone", "v3"] });
  assert.equal(arranged.hidden.length, 0);
  assert.equal(arranged.order[0], "v3");
  assert.ok(!arranged.order.includes("gone"));
});

test("WA-D07: move left and right change only the personal order", () => {
  const prefs = moveView(emptyViewPrefs, seven, "v3", -1);
  assert.deepEqual(prefs.order.slice(0, 3), ["v1", "v3", "v2"]);
  assert.equal(moveView(prefs, seven, "v1", -1), prefs); // at the edge: unchanged
  assert.deepEqual(seven.map((v) => v.id), ["v1", "v2", "v3", "v4", "v5", "v6", "v7"]); // shared list untouched
});

test("WA-D07: the selected view stays visible even beyond the six-tab limit", () => {
  const arranged = arrangeViews(seven, emptyViewPrefs, "v7");
  assert.equal(arranged.visible.length, 6);
  assert.ok(arranged.visible.some((v) => v.id === "v7"));
});

test("WA-D07: saved preferences are read per workspace and corrupt values are ignored", () => {
  const stored = { s1: { hidden: ["a", 3], order: ["b"] }, s2: "bad" };
  assert.deepEqual(viewPrefsFrom(stored, "s1"), { hidden: ["a"], order: ["b"] });
  assert.deepEqual(viewPrefsFrom(stored, "s2"), emptyViewPrefs);
  assert.deepEqual(viewPrefsFrom(stored, "missing"), emptyViewPrefs);
});

test("WA-D07: dragging a view before another changes only the personal order", () => {
  const prefs = moveViewBefore(emptyViewPrefs, seven, "v4", "v2");
  assert.deepEqual(prefs.order.slice(0, 4), ["v1", "v4", "v2", "v3"]);
  assert.equal(moveViewBefore(prefs, seven, "v4", "v4"), prefs); // dropped on itself: unchanged
  assert.equal(moveViewBefore(prefs, seven, "v4", "missing"), prefs);
});
