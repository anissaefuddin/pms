// Preferensi sidebar: urutan pin, lebar, dan Space yang terbuka. Logika murni di atas, akses localStorage dibungkus agar aman.
// Tanpa import agar bisa dites dengan node --test.

export const SHORTCUT_IDS = [
  "request_forms", "stream", "portfolio", "workload", "timesheets", "starred",
  "dashboards", "reports", "my_todo", "calendars", "created_by_me", "reviews", "recycle",
] as const;
export type ShortcutId = (typeof SHORTCUT_IDS)[number];

/** Kunci sidebar_hidden (profil) yang menyembunyikan shortcut; shortcut tanpa kunci selalu tampil. */
export const SHORTCUT_HIDE_KEY: Partial<Record<ShortcutId, string>> = {
  request_forms: "request_forms", stream: "stream", workload: "workload", timesheets: "timesheets",
  starred: "starred", reports: "reports", my_todo: "my_todo", calendars: "calendars", created_by_me: "created_by_me", portfolio: "portfolio", reviews: "reviews",
};

export const SHORTCUT_LABEL_KEY: Record<ShortcutId, string> = {
  request_forms: "nav.requests.label", stream: "nav.stream.label", portfolio: "nav.portfolio",
  workload: "nav.workload", timesheets: "time.timesheet", starred: "starred.nav",
  dashboards: "dashboards.nav.label", reports: "reports.nav.label", my_todo: "nav.todo.label", calendars: "nav.calendars.label", created_by_me: "nav.created.label",
  reviews: "items.fileReview.nav", recycle: "nav.recycle.label",
};

/** Selalu tampil di atas sidebar, urutannya Dashboards, Inbox, lalu Search. Tidak bisa disematkan atau dihapus. */
export const FIXED_SHORTCUT: ShortcutId = "dashboards";

/** Bawaan: Request forms dan Stream. */
export const DEFAULT_PINNED: readonly ShortcutId[] = ["request_forms", "stream"];

/** Hanya shortcut yang punya kunci API (sidebar_pinned) yang bisa disematkan; yang lain selalu di More. Dashboards tidak ada di sini karena selalu tampil. */
export const PINNABLE_IDS = SHORTCUT_IDS.filter((id) => SHORTCUT_HIDE_KEY[id] !== undefined);

export const WIDTH_KEY = "pms:sidebar-width";
// Default 296px, minimum 256px, maximum 400px: room for tables without giving up navigation. Kept per browser (localStorage).
export const SIDEBAR_WIDTH = { min: 256, max: 400, step: 8 } as const;

function store(): Storage | null {
  try {
    return globalThis.localStorage ?? null;
  } catch {
    return null;
  }
}

function read(key: string): string | null {
  try {
    return store()?.getItem(key) ?? null;
  } catch {
    return null;
  }
}

function write(key: string, value: string) {
  try {
    store()?.setItem(key, value);
  } catch {
    // penyimpanan penuh atau diblokir: sidebar tetap jalan dengan nilai di memori
  }
}

/** Dari profile.sidebar_pinned: buang id yang tidak dikenal dan duplikat, pertahankan urutan. Bila bukan array, pakai bawaan. */
export function sanitizePinOrder(value: unknown): ShortcutId[] {
  if (!Array.isArray(value)) return [...DEFAULT_PINNED];
  const seen = new Set<ShortcutId>();
  for (const entry of value) {
    if ((PINNABLE_IDS as readonly unknown[]).includes(entry)) seen.add(entry as ShortcutId);
  }
  return [...seen];
}

/** Geser shortcut yang tersemat satu langkah. Di batas atau bila tidak tersemat, urutan tidak berubah. */
export function movePin(order: readonly ShortcutId[], id: ShortcutId, delta: -1 | 1): ShortcutId[] {
  const from = order.indexOf(id);
  const to = from + delta;
  if (from < 0 || to < 0 || to >= order.length) return [...order];
  const next = [...order];
  [next[from], next[to]] = [next[to], next[from]];
  return next;
}

/** Tersemat menjadi tidak tersemat (dihapus), atau sebaliknya (ditambah di akhir). */
export function togglePin(order: readonly ShortcutId[], id: ShortcutId): ShortcutId[] {
  return order.includes(id) ? order.filter((entry) => entry !== id) : [...order, id];
}

/** Body PATCH /me untuk menyimpan pin: shortcut yang disematkan juga dibuka dari Preferences agar checklist dan sidebar sama. */
export function pinPatch(order: readonly ShortcutId[], hidden: readonly string[]) {
  const keys = sanitizePinOrder(order).map((id) => SHORTCUT_HIDE_KEY[id]);
  return { sidebar_pinned: sanitizePinOrder(order), sidebar_hidden: hidden.filter((key) => !keys.includes(key)) };
}

export function isShortcutVisible(id: ShortcutId, hidden: readonly string[]): boolean {
  const key = SHORTCUT_HIDE_KEY[id];
  return !key || !hidden.includes(key);
}

export function clampWidth(width: number): number {
  return Math.min(SIDEBAR_WIDTH.max, Math.max(SIDEBAR_WIDTH.min, Math.round(width)));
}

export function readWidth(fallback = 296): number {
  const raw = Number(read(WIDTH_KEY));
  return raw ? clampWidth(raw) : clampWidth(fallback);
}

export function writeWidth(width: number) {
  write(WIDTH_KEY, String(clampWidth(width)));
}

export const openSpacesKey = (userId: string) => `pms:sidebar-spaces:${userId}`;

export function readOpenSpaces(userId: string): string[] {
  try {
    const parsed = JSON.parse(read(openSpacesKey(userId)) ?? "[]");
    return Array.isArray(parsed) ? parsed.filter((entry): entry is string => typeof entry === "string") : [];
  } catch {
    return [];
  }
}

export function writeOpenSpaces(userId: string, ids: readonly string[]) {
  write(openSpacesKey(userId), JSON.stringify(ids));
}

export function toggleOpen(ids: readonly string[], id: string): string[] {
  return ids.includes(id) ? ids.filter((entry) => entry !== id) : [...ids, id];
}

export type RecentEntry = { id: string; title: string; type: string };
export const RECENT_MAX = 5;
export const recentKey = (userId: string) => `pms:sidebar-recent:${userId}`;

/** Item terakhir dibuka dari sidebar (per pengguna dan per perangkat). Entri rusak dibuang. */
export function readRecent(userId: string): RecentEntry[] {
  try {
    const parsed = JSON.parse(read(recentKey(userId)) ?? "[]");
    return Array.isArray(parsed)
      ? parsed.filter((e) => typeof e?.id === "string" && typeof e?.title === "string").slice(0, RECENT_MAX)
      : [];
  } catch {
    return [];
  }
}

export function writeRecent(userId: string, list: readonly RecentEntry[]) {
  write(recentKey(userId), JSON.stringify(list));
}

/** Item yang baru dibuka ke paling depan; satu item hanya muncul sekali. */
export function pushRecent(list: readonly RecentEntry[], entry: RecentEntry): RecentEntry[] {
  return [entry, ...list.filter((e) => e.id !== entry.id)].slice(0, RECENT_MAX);
}

/** Kelas tema sidebar. Default terang; ganti ke "sidebar-navy" setelah D-84 disetujui. */
export const SIDEBAR_THEME = "sidebar-navy";
