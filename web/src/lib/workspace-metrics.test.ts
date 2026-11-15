import assert from "node:assert/strict";
import { test } from "node:test";
import { completionPercent, ganttTone, groupBlueprintsBySpace, projectProgressPercent, suggestedBlueprints, upcomingMilestones, workItemCounts } from "./workspace-metrics.ts";

const done = { id: "d", status: { group: "completed" } };
const active = { id: "a", status: { group: "active" } };
const open = { id: "o", status: { group: "deferred" } };

test("completionPercent: persentase selesai, nol bila kosong", () => {
  assert.equal(completionPercent([]), 0);
  assert.equal(completionPercent([done, active, open]), 33);
  assert.equal(completionPercent([done, done]), 100);
});

test("projectProgressPercent: nilai server dipakai bila ada, selain itu hitung dari anak", () => {
  assert.equal(projectProgressPercent({ progress_percent: 70 }, [done]), 70);
  assert.equal(projectProgressPercent({ progress_percent: null }, [done, active]), 50);
});

test("workItemCounts: total, selesai, dan sedang berjalan", () => {
  assert.deepEqual(workItemCounts([done, active, open]), { total: 3, done: 1, inProgress: 1 });
});

test("upcomingMilestones: hanya milestone belum selesai yang belum lewat, urut tanggal", () => {
  const items = [
    { id: "m2", is_milestone: true, due_date: "2026-12-02", status: { group: "active" } },
    { id: "m1", is_milestone: true, due_date: "2026-11-20", status: { group: "active" } },
    { id: "past", is_milestone: true, due_date: "2026-11-01", status: { group: "active" } },
    { id: "fin", is_milestone: true, due_date: "2026-11-30", status: { group: "completed" } },
    { id: "task", is_milestone: false, due_date: "2026-11-25", status: { group: "active" } },
  ];
  assert.deepEqual(upcomingMilestones(items, "2026-11-15").map((m) => m.id), ["m1", "m2"]);
});

test("groupBlueprintsBySpace: ikut urutan Space, blueprint Space lain tidak masuk", () => {
  const groups = groupBlueprintsBySpace(
    [
      { id: "b1", name: "Zeta", space_id: "s2", item_type: "project", item_count: 1 },
      { id: "b2", name: "Alpha", space_id: "s2", item_type: "task", item_count: 2 },
      { id: "b3", name: "Other", space_id: "s9", item_type: "project", item_count: 1 },
    ],
    [{ id: "s1", name: "Personal" }, { id: "s2", name: "Shared" }],
  );
  assert.deepEqual(groups.map((g) => [g.spaceId, g.items.map((i) => i.id)]), [["s1", []], ["s2", ["b2", "b1"]]]);
});

test("suggestedBlueprints: tiga preset dengan nama yang diharapkan", () => {
  assert.deepEqual(suggestedBlueprints.map((b) => b.name), ["Software delivery", "Client implementation", "Internal initiative"]);
});

test("ganttTone: setiap grup status punya token cat-*", () => {
  assert.equal(ganttTone("completed"), "cat-2");
  assert.equal(ganttTone(undefined), "cat-1");
});
