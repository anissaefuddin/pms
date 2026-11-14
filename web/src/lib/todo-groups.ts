export type Bucket = "overdue" | "today" | "week" | "later" | "none";
const order: Bucket[] = ["overdue", "today", "week", "later", "none"];

/** Buckets a YYYY-MM-DD due date relative to `today` (same format). */
export function dueBucket(due: string | null | undefined, today: string): Bucket {
  if (!due) return "none";
  if (due < today) return "overdue";
  if (due === today) return "today";
  const limit = new Date(`${today}T00:00:00Z`);
  limit.setUTCDate(limit.getUTCDate() + 7);
  return due <= limit.toISOString().slice(0, 10) ? "week" : "later";
}

/** Groups keep first-seen order, except date buckets which follow the fixed bucket order. */
export function groupBy<T>(rows: T[], key: (row: T) => string, sortKeys?: string[]): { key: string; rows: T[] }[] {
  const map = new Map<string, T[]>();
  for (const row of rows) map.set(key(row), [...(map.get(key(row)) ?? []), row]);
  const groups = [...map].map(([k, r]) => ({ key: k, rows: r }));
  return sortKeys ? groups.sort((a, b) => sortKeys.indexOf(a.key) - sortKeys.indexOf(b.key)) : groups;
}
export const bucketOrder: string[] = order;
