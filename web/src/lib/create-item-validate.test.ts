import assert from "node:assert/strict";
import { test } from "node:test";
import { SUPPORTED_DEFAULT_VIEWS, canAddSubitem, isWorkday, newItemId, payloadDates, validateCreate, type CreateDraft } from "./create-item-validate.ts";

const base: CreateDraft = { kind: "project", title: "Rilis", start: "", due: "", spaceId: "s1" };

test("nama kosong atau hanya spasi ditolak di field title", () => {
  assert.deepEqual(validateCreate({ ...base, title: "" }), { field: "title", code: "nameRequired" });
  assert.deepEqual(validateCreate({ ...base, title: "   " }), { field: "title", code: "nameRequired" });
  assert.equal(validateCreate(base), null);
});

test("enam view default sama dengan yang diterima API", () => {
  assert.deepEqual([...SUPPORTED_DEFAULT_VIEWS], ["table", "board", "gantt", "calendar", "files", "chart"]);
});

test("Space wajib dipilih", () => {
  assert.deepEqual(validateCreate({ ...base, spaceId: "" }), { field: "location", code: "locationRequired" });
});

test("finish sebelum start ditolak; sama dengan start diterima", () => {
  assert.deepEqual(validateCreate({ ...base, start: "2026-11-16", due: "2026-11-13" }), { field: "due_date", code: "dateOrder" });
  assert.equal(validateCreate({ ...base, start: "2026-11-16", due: "2026-11-16" }), null);
});

test("tanggal akhir pekan ditolak", () => {
  assert.equal(isWorkday("2026-11-14"), false); // Sabtu
  assert.deepEqual(validateCreate({ ...base, due: "2026-11-14" }), { field: "due_date", code: "workingDays" });
});

test("Folder tanpa tanggal: tanggal dibuang dari payload dan tidak memicu error", () => {
  assert.deepEqual(payloadDates("folder", "2026-11-16", "2026-11-20"), {});
  assert.equal(validateCreate({ ...base, kind: "folder", start: "2026-11-20", due: "2026-11-16" }), null);
  assert.deepEqual(payloadDates("project", "2026-11-16", ""), { start_date: "2026-11-16" });
});

test("Task boleh di bawah Project, Folder (allowed_children di server), dan Task (nested subtask)", () => {
  const task = { ...base, kind: "task" as const };
  assert.equal(validateCreate({ ...task, parent: { type: "project", spaceId: "s1" } }), null);
  assert.equal(validateCreate({ ...task, parent: { type: "folder", spaceId: "s1" } }), null, "tipe folder dinilai server");
  assert.equal(validateCreate({ ...task, parent: { type: "task", spaceId: "s1" } }), null, "Task → Task (PRD TASK-02)");
  assert.equal(validateCreate(task), null, "Task di root Space tetap diizinkan");
});

test("Task sebagai parent hanya boleh berisi Task: Project dan Folder ditolak", () => {
  const parent = { type: "task" as const, spaceId: "s1" };
  assert.deepEqual(validateCreate({ ...base, kind: "project", parent }), { field: "parent", code: "taskChildOnly" });
  assert.deepEqual(validateCreate({ ...base, kind: "folder", parent }), { field: "parent", code: "taskChildOnly" });
  assert.equal(validateCreate({ ...base, kind: "task", parent }), null);
});

test("Parent di Space lain selalu ditolak", () => {
  assert.deepEqual(validateCreate({ ...base, kind: "task", parent: { type: "project", spaceId: "s2" } }), { field: "parent", code: "taskParent" });
  assert.deepEqual(validateCreate({ ...base, kind: "task", parent: { type: "task", spaceId: "s2" } }), { field: "parent", code: "taskParent" });
});

test("Add subitem: satu aturan untuk Table dan Item Detail (folder, project, task)", () => {
  assert.equal(canAddSubitem(["view", "create"]), true, "pengguna yang boleh create di item");
  assert.equal(canAddSubitem(["view", "comment"]), false, "tanpa create: tidak ada Add subitem");
  assert.equal(canAddSubitem([]), false);
});

test("newItemId: UUIDv7 (version 7, RFC variant), unik, dan diurutkan menurut waktu", () => {
  const a = newItemId();
  const b = newItemId();
  assert.match(a, /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
  assert.notEqual(a, b);
});
