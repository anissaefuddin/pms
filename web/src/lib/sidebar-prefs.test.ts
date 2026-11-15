import assert from "node:assert/strict";
import test from "node:test";
import {
  DEFAULT_PINNED, PINNABLE_IDS, clampWidth, isShortcutVisible, movePin, readWidth, sanitizePinOrder, togglePin, toggleOpen,
  type ShortcutId,
} from "./sidebar-prefs.ts";

const order: ShortcutId[] = ["request_forms", "stream", "starred"];

test("movePin swaps with the neighbour in both directions", () => {
  assert.deepEqual(movePin(order, "stream", -1), ["stream", "request_forms", "starred"]);
  assert.deepEqual(movePin(order, "stream", 1), ["request_forms", "starred", "stream"]);
});

test("movePin does nothing at the edges, for unpinned ids, and never mutates the input", () => {
  assert.deepEqual(movePin(order, "request_forms", -1), order);
  assert.deepEqual(movePin(order, "starred", 1), order);
  assert.deepEqual(movePin(order, "portfolio", 1), order);
  const copy = movePin(order, "stream", 1);
  assert.notEqual(copy, order);
  assert.deepEqual(order, ["request_forms", "stream", "starred"]);
});

test("togglePin removes a pinned id and appends an unpinned one", () => {
  assert.deepEqual(togglePin(order, "stream"), ["request_forms", "starred"]);
  assert.deepEqual(togglePin(order, "portfolio"), ["request_forms", "stream", "starred", "portfolio"]);
  assert.deepEqual(togglePin(togglePin(order, "stream"), "stream"), ["request_forms", "starred", "stream"]);
});

test("sanitizePinOrder drops unknown and duplicate ids and keeps the first position", () => {
  assert.deepEqual(sanitizePinOrder(["stream", "bogus", "stream", "reports"]), ["stream", "reports"]);
  assert.deepEqual(sanitizePinOrder("not an array"), [...DEFAULT_PINNED]);
});

test("only shortcuts with an API key can be pinned, so the others never reach sidebar_pinned", () => {
  assert.deepEqual(sanitizePinOrder(["portfolio", "stream", "recycle"]), ["portfolio", "stream"]);
  assert.equal(PINNABLE_IDS.includes("reviews"), true);
  assert.equal(PINNABLE_IDS.includes("recycle"), false);
  assert.deepEqual(sanitizePinOrder(undefined), [...DEFAULT_PINNED]);
});

test("visibility follows sidebar_hidden and ignores shortcuts without a key", () => {
  assert.equal(isShortcutVisible("stream", ["stream"]), false);
  assert.equal(isShortcutVisible("stream", ["workload"]), true);
  assert.equal(isShortcutVisible("recycle", ["stream", "workload"]), true);
});

test("width is clamped to 220–400 and rounded", () => {
  assert.equal(clampWidth(100), 220);
  assert.equal(clampWidth(999), 400);
  assert.equal(clampWidth(301.4), 301);
  assert.equal(readWidth(), 270);
});

test("toggleOpen flips a Space id without touching the rest", () => {
  assert.deepEqual(toggleOpen(["a", "b"], "a"), ["b"]);
  assert.deepEqual(toggleOpen(["a"], "c"), ["a", "c"]);
});
