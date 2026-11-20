// Format riwayat item (D9): perubahan dibaca sebagai "field: dari → ke" dan entri dikelompokkan per hari di zona waktu user.
// Logika murni agar bisa dites tanpa React.

export type ChangeValue = { from?: unknown; to?: unknown };

/** Nilai ditampilkan singkat: teks apa adanya, kosong sebagai "—", objek sebagai JSON yang dipotong. */
export function showValue(value: unknown): string {
  if (value === null || value === undefined || value === "") return "—";
  const text = typeof value === "string" ? value : JSON.stringify(value);
  return text.length > 60 ? `${text.slice(0, 57)}…` : text;
}

/** Satu perubahan menjadi "from → to". Perubahan tanpa nilai lama (nilai pertama) hanya menampilkan nilai baru. */
export function formatChange(change: ChangeValue): string {
  return `${showValue(change.from)} → ${showValue(change.to)}`;
}

/** Kolom internal: tetap tercatat di audit, tidak ditampilkan ke pengguna. */
const hiddenFields = new Set(["id", "seq", "created_at", "created_by", "deleted_at", "deleted_by", "deleted_root_id", "status_changed_at"]);

/** Label i18n untuk kolom item. Kolom yang tidak dikenal tampil sebagai "Kolom lain", bukan nama kolom database. */
const fieldKeys: Record<string, string> = {
  title: "views.title",
  status_id: "views.columns.status",
  importance: "views.columns.importance",
  assignee_ids: "views.columns.assignee",
  start_date: "views.columns.start_date",
  due_date: "views.columns.due_date",
  progress_percent: "views.columns.progress",
  progress_method: "views.columns.progress",
  description: "items.activity.field.description",
  completed_at: "items.activity.field.completed_at",
  is_milestone: "items.activity.field.is_milestone",
  owner_id: "items.activity.field.owner_id",
  type_id: "items.activity.field.type_id",
  type: "items.activity.field.type",
  default_view_type: "items.activity.field.default_view_type",
  default_workflow_id: "items.activity.field.default_workflow_id",
  effort_minutes: "items.activity.field.effort_minutes",
  duration_days: "items.activity.field.duration_days",
  fields: "items.activity.field.fields",
};

/** Label satu kolom, atau null bila kolom itu disembunyikan. */
export function fieldLabel(field: string, t: (key: string) => string): string | null {
  if (hiddenFields.has(field)) return null;
  return t(fieldKeys[field] ?? "items.activity.field.other");
}

/** Label unik untuk daftar kolom yang berubah, dengan urutan asli. */
export function changedFieldLabels(fields: string[], t: (key: string) => string): string[] {
  const labels = new Set<string>();
  for (const field of fields) {
    const label = fieldLabel(field, t);
    if (label) labels.add(label);
  }
  return [...labels];
}

/** Kunci hari (YYYY-MM-DD) pada zona waktu user, supaya kejadian dekat tengah malam jatuh di hari yang benar. */
export function dayKey(createdAt: string, timeZone?: string): string {
  try {
    return new Date(createdAt).toLocaleDateString("en-CA", { timeZone: timeZone || undefined });
  } catch {
    return createdAt.slice(0, 10);
  }
}

/** Kelompokkan entri (urutan dipertahankan, terbaru dulu) per hari. */
export function groupByDay<T extends { created_at: string }>(entries: T[], timeZone?: string): { day: string; items: T[] }[] {
  const groups: { day: string; items: T[] }[] = [];
  for (const entry of entries) {
    const day = dayKey(entry.created_at, timeZone);
    const last = groups[groups.length - 1];
    if (last && last.day === day) last.items.push(entry);
    else groups.push({ day, items: [entry] });
  }
  return groups;
}
