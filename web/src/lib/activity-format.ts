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
