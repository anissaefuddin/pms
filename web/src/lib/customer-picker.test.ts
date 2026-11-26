import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { CUSTOMER_PICKER_LIMIT, eligibleCustomersKey, isTruncated, selectionStatus } from "./customer-picker.ts";

const options = [{ id: "a" }, { id: "b" }];
const loaded = { loaded: true, searching: false, truncated: false };

// PK-07: a previously selected customer is never shown as valid unless the full, current list contains it.
test("PK-07: a selected customer missing from the full list is unavailable, not silently valid or substituted", () => {
  assert.equal(selectionStatus("gone", options, loaded), "unavailable");
  assert.equal(selectionStatus("a", options, loaded), "selectable");
});

test("PK-07: while searching or on a full page the picker cannot prove availability, so it says unknown", () => {
  assert.equal(selectionStatus("gone", options, { ...loaded, searching: true }), "unknown");
  assert.equal(selectionStatus("gone", options, { ...loaded, truncated: true }), "unknown");
  assert.equal(selectionStatus("gone", [], { ...loaded, loaded: false }), "unknown");
});

test("PK-07: no selection and a selection still loading are handled without claiming validity", () => {
  assert.equal(selectionStatus(null, options, loaded), "none");
  assert.equal(selectionStatus("a", [], { loaded: false, searching: false, truncated: false }), "unknown");
});

// PK-06 and PK-08: administrative and membership options never share a cache entry, and neither do two users.
test("PK-06: the lookup mode is part of the cache key", () => {
  assert.notDeepEqual(eligibleCustomersKey("u1", "membership", ""), eligibleCustomersKey("u1", "administrative", ""));
});

test("PK-08: another signed-in user never gets the previous user's cached options", () => {
  assert.notDeepEqual(eligibleCustomersKey("u1", "membership", ""), eligibleCustomersKey("u2", "membership", ""));
  assert.deepEqual(eligibleCustomersKey("u1", "membership", "x"), ["clients", "eligible", "u1", "membership", "x"]);
});

test("the search text is part of the key, so each search is its own cache entry", () => {
  assert.notDeepEqual(eligibleCustomersKey("u1", "membership", "a"), eligibleCustomersKey("u1", "membership", "b"));
});

test("a full page says more customers may exist", () => {
  assert.equal(isTruncated(CUSTOMER_PICKER_LIMIT), true);
  assert.equal(isTruncated(CUSTOMER_PICKER_LIMIT - 1), false);
});

// PK-09: the picker's EN and ID labels exist and cover the same keys.
test("PK-09: the picker has EN and ID labels for every key", () => {
  const load = (lang: string) => JSON.parse(readFileSync(new URL(`../i18n/${lang}/settings.json`, import.meta.url), "utf8")) as Record<string, string>;
  const en = load("en");
  const id = load("id");
  const picker = Object.keys(en).filter((key) => key.startsWith("customers.picker."));
  assert.ok(picker.length >= 10, `expected the picker keys, found ${picker.length}`);
  for (const key of picker) {
    assert.ok(id[key], `missing Indonesian label for ${key}`);
    assert.ok(en[key].length > 0, `empty English label for ${key}`);
  }
  assert.deepEqual(Object.keys(id).filter((key) => key.startsWith("customers.picker.")).sort(), picker.slice().sort());
});
