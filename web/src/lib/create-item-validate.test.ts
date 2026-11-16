import assert from "node:assert/strict";
import { test } from "node:test";
import { SUPPORTED_DEFAULT_VIEWS, isWorkday, payloadDates, validateCreate, type CreateDraft } from "./create-item-validate.ts";

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

test("Task dengan parent wajib Project di Space yang sama", () => {
  const task = { ...base, kind: "task" as const };
  assert.equal(validateCreate({ ...task, parent: { type: "project", spaceId: "s1" } }), null);
  assert.deepEqual(validateCreate({ ...task, parent: { type: "folder", spaceId: "s1" } }), { field: "parent", code: "taskParent" });
  assert.deepEqual(validateCreate({ ...task, parent: { type: "project", spaceId: "s2" } }), { field: "parent", code: "taskParent" });
  assert.equal(validateCreate(task), null, "Task di root Space tetap diizinkan seperti sebelumnya");
});
