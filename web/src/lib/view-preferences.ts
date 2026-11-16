// Preferensi view personal (WA-D07): view tersembunyi dan urutan tab per workspace. Logika murni, tanpa React.
// Tidak mengubah konfigurasi bersama (position, default, visibility). View yang sudah tidak ada tidak dipulihkan dari preferensi lama.

export const MAX_VISIBLE_TABS = 6;

export type ViewPrefs = { hidden: string[]; order: string[] };
export const emptyViewPrefs: ViewPrefs = { hidden: [], order: [] };

/** Preferensi satu workspace dari profil. Nilai rusak diabaikan. */
export function viewPrefsFrom(stored: unknown, scopeId: string): ViewPrefs {
  const entry = stored && typeof stored === "object" ? (stored as Record<string, unknown>)[scopeId] : undefined;
  if (!entry || typeof entry !== "object") return emptyViewPrefs;
  const { hidden, order } = entry as { hidden?: unknown; order?: unknown };
  const strings = (value: unknown) => Array.isArray(value) ? value.filter((id): id is string => typeof id === "string") : [];
  return { hidden: strings(hidden), order: strings(order) };
}

type ViewLike = { id: string; is_default?: boolean };

/**
 * Susun view untuk tampilan. Urutan: preferensi personal dulu, sisanya mengikuti urutan server.
 * Hidden: view yang disembunyikan user, kecuali view default efektif (default selalu terlihat).
 * Visible: maksimal 6 tab; view terpilih selalu terlihat, menggantikan tab terakhir bila perlu.
 * Overflow: view reguler yang tidak muat di tab, dibuka lewat "All views".
 */
export function arrangeViews<T extends ViewLike>(views: T[], prefs: ViewPrefs, selectedId?: string) {
  const byId = new Map(views.map((view) => [view.id, view]));
  // Hanya id yang masih ada yang dipakai; yang sudah dihapus atau tidak bisa diakses diabaikan.
  const ordered = [
    ...prefs.order.map((id) => byId.get(id)).filter((view): view is T => !!view),
    ...views.filter((view) => !prefs.order.includes(view.id)),
  ];
  const seen = new Set<string>();
  const unique = ordered.filter((view) => (seen.has(view.id) ? false : (seen.add(view.id), true)));
  const hiddenSet = new Set(prefs.hidden.filter((id) => byId.has(id) && !byId.get(id)?.is_default));
  const hidden = unique.filter((view) => hiddenSet.has(view.id));
  const regular = unique.filter((view) => !hiddenSet.has(view.id));
  let visible = regular.slice(0, MAX_VISIBLE_TABS);
  const selected = regular.find((view) => view.id === selectedId);
  if (selected && !visible.includes(selected)) visible = [...visible.slice(0, MAX_VISIBLE_TABS - 1), selected];
  const overflow = regular.filter((view) => !visible.includes(view));
  return { visible, overflow, hidden, order: unique.map((view) => view.id) };
}

/** Sembunyikan satu view. View default efektif tidak bisa disembunyikan (WA-D07). */
export function hideView(prefs: ViewPrefs, id: string, views: ViewLike[]): ViewPrefs {
  const view = views.find((candidate) => candidate.id === id);
  if (!view || view.is_default) return prefs;
  return { ...prefs, hidden: prefs.hidden.includes(id) ? prefs.hidden : [...prefs.hidden, id] };
}

/** Pulihkan view yang disembunyikan. */
export function restoreView(prefs: ViewPrefs, id: string): ViewPrefs {
  return { ...prefs, hidden: prefs.hidden.filter((entry) => entry !== id) };
}

/** Pindahkan view ke posisi view target (drag-and-drop). Sama dengan view itu sendiri atau target tak dikenal: tidak berubah. */
export function moveViewBefore(prefs: ViewPrefs, views: ViewLike[], id: string, targetId: string): ViewPrefs {
  const { order } = arrangeViews(views, prefs);
  if (id === targetId || !order.includes(id) || !order.includes(targetId)) return prefs;
  const next = order.filter((entry) => entry !== id);
  next.splice(next.indexOf(targetId), 0, id);
  return { ...prefs, order: next };
}

/** Pindah satu langkah dalam urutan personal. Di batas, urutan tidak berubah. */
export function moveView(prefs: ViewPrefs, views: ViewLike[], id: string, delta: -1 | 1): ViewPrefs {
  const { order } = arrangeViews(views, prefs);
  const from = order.indexOf(id), to = from + delta;
  if (from < 0 || to < 0 || to >= order.length) return prefs;
  const next = [...order];
  [next[from], next[to]] = [next[to], next[from]];
  return { ...prefs, order: next };
}
