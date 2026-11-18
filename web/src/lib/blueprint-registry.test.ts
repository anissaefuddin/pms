import { test } from "node:test";
import assert from "node:assert/strict";
import { filterRows, groupRows, sortRows, type RegistryRow } from "./blueprint-registry.ts";

const rows: RegistryRow[] = [
  { id: "1", name: "Launch kit", item_type: "project", scope: "space", space_id: "s1", spaceName: "Marketing", item_count: 3, updated_at: "2026-11-10T10:00:00Z", canManage: true },
  { id: "2", name: "Onboarding", item_type: "task", scope: "account", space_id: "p1", spaceName: "Personal", item_count: 1, updated_at: "2026-11-12T10:00:00Z", canManage: true },
  { id: "3", name: "Audit", item_type: "folder", scope: "space", space_id: "s2", spaceName: "Operations", item_count: 2, updated_at: "2026-11-01T10:00:00Z", canManage: false },
];

test("filters by type, scope, and text in the name or Space", () => {
  assert.deepEqual(filterRows(rows, { query: "", type: "all", scope: "all" }).map((r) => r.id), ["1", "2", "3"]);
  assert.deepEqual(filterRows(rows, { query: "", type: "all", scope: "account" }).map((r) => r.id), ["2"]);
  assert.deepEqual(filterRows(rows, { query: "oper", type: "all", scope: "all" }).map((r) => r.id), ["3"]);
  assert.deepEqual(filterRows(rows, { query: "", type: "folder", scope: "space" }).map((r) => r.id), ["3"]);
});

test("sorts by name and by last update in both directions", () => {
  assert.deepEqual(sortRows(rows, "name").map((r) => r.name), ["Audit", "Launch kit", "Onboarding"]);
  assert.deepEqual(sortRows(rows, "updated_desc").map((r) => r.id), ["2", "1", "3"]);
  assert.deepEqual(sortRows(rows, "updated_asc").map((r) => r.id), ["3", "1", "2"]);
});

test("groups by scope, Space, or type and keeps the sorted order inside groups", () => {
  assert.deepEqual(groupRows(rows, "none").map((g) => g.rows.length), [3]);
  assert.deepEqual(groupRows(rows, "scope").map((g) => [g.key, g.rows.map((r) => r.id)]), [["space", ["1", "3"]], ["account", ["2"]]]);
  assert.deepEqual(groupRows(rows, "space").map((g) => g.key), ["s1", "p1", "s2"]);
  assert.deepEqual(groupRows(rows, "type").map((g) => g.key), ["project", "task", "folder"]);
});
