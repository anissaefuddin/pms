// Ekspansi hierarki per view (WA-D05), disimpan di server pada profil user. Parser murni agar bisa dites tanpa React.

export type ViewExpansion = { rows: string[]; gantt_collapsed: boolean };

export const emptyViewExpansion: ViewExpansion = { rows: [], gantt_collapsed: false };

/** Ambil ekspansi satu view dari profil. Nilai yang rusak diabaikan; id baris yang sudah tidak ada ditangani pemanggil. */
export function viewExpansionFrom(stored: unknown, viewId: string): ViewExpansion {
  const entry = stored && typeof stored === "object" ? (stored as Record<string, unknown>)[viewId] : undefined;
  if (!entry || typeof entry !== "object") return emptyViewExpansion;
  const { rows, gantt_collapsed } = entry as { rows?: unknown; gantt_collapsed?: unknown };
  return {
    rows: Array.isArray(rows) ? rows.filter((row): row is string => typeof row === "string") : [],
    gantt_collapsed: gantt_collapsed === true,
  };
}
