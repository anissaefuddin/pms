// Logika murni untuk dialog create item (tanpa import agar bisa dites dengan node --test).

export type CreateKind = "project" | "folder" | "task";
export type CreateIssue = {
  field: "title" | "location" | "parent" | "due_date";
  code: "nameRequired" | "locationRequired" | "taskParent" | "taskChildOnly" | "dateOrder" | "workingDays";
};
export type CreateDraft = {
  kind: CreateKind;
  title: string;
  /** YYYY-MM-DD atau "" */
  start: string;
  due: string;
  spaceId: string;
  /** Parent yang dipilih; undefined bila di root Space. */
  parent?: { type: CreateKind; spaceId: string };
};

/** Pilihan view default yang diterima API (default_view_type). */
export const SUPPORTED_DEFAULT_VIEWS = ["table", "board", "gantt", "calendar", "files", "chart"] as const;
export type SupportedDefaultView = (typeof SUPPORTED_DEFAULT_VIEWS)[number];

export function isWorkday(value: string) {
  const day = new Date(`${value}T00:00:00Z`).getUTCDay();
  return day >= 1 && day <= 5;
}

/** Folder tidak punya tanggal; tanggal yang terisi diabaikan. */
export function payloadDates(kind: CreateKind, start: string, due: string): { start_date?: string; due_date?: string } {
  if (kind === "folder") return {};
  return { ...(start && { start_date: start }), ...(due && { due_date: due }) };
}

/**
 * "Add subitem" rule shared by Table rows and the Item Detail panel: offered for any item the user can create under.
 * Which child types appear is decided by the Create dialog (allowed_children) and enforced by the server.
 */
export function canAddSubitem(capabilities: readonly string[]): boolean {
  return capabilities.includes("create");
}

/** Pesan pertama yang harus ditampilkan, atau null bila draft valid. */
export function validateCreate(draft: CreateDraft): CreateIssue | null {
  if (!draft.title.trim()) return { field: "title", code: "nameRequired" };
  if (!draft.spaceId) return { field: "location", code: "locationRequired" };
  // Parent harus di Space yang sama. Aturan tipe mengikuti domain: Task hanya boleh berisi Task (create.go), Project dan Folder
  // dinilai server lewat allowed_children, jadi klien tidak membuat batas tipe sendiri.
  if (draft.parent && draft.parent.spaceId !== draft.spaceId) return { field: "parent", code: "taskParent" };
  if (draft.parent?.type === "task" && draft.kind !== "task") return { field: "parent", code: "taskChildOnly" };
  const dates = payloadDates(draft.kind, draft.start, draft.due);
  const start = dates.start_date ?? "";
  const due = dates.due_date ?? "";
  if (start && due && due < start) return { field: "due_date", code: "dateOrder" };
  if ([start, due].some((day) => day && !isWorkday(day))) return { field: "due_date", code: "workingDays" };
  return null;
}
