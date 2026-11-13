import type { components } from "@/api/schema";

export type AnalyticsResult = components["schemas"]["AnalyticsResult"];
type Row = AnalyticsResult["rows"][number];

export const OTHERS_KEY = "__others__";
export const MAX_SLICES = 8;

/** Keeps the largest slices and folds the rest into one "Others" slice, so a pie never has dozens of unreadable wedges. */
export function foldOthers(rows: { key: string; label: string; value: number }[], limit = MAX_SLICES, othersLabel = "Others") {
  const positive = rows.filter((row) => row.value > 0).sort((a, b) => b.value - a.value || a.label.localeCompare(b.label));
  if (positive.length <= limit) return positive;
  const kept = positive.slice(0, limit - 1);
  const rest = positive.slice(limit - 1).reduce((sum, row) => sum + row.value, 0);
  return [...kept, { key: OTHERS_KEY, label: othersLabel, value: rest }];
}

export type Series = { name: string; values: number[] };
export type Grid = { categories: { key: string; label: string }[]; series: Series[] };

/** Pivots result rows into categories (group by) and series (break by). Without break by there is a single series. */
export function toGrid(rows: Row[], label: (key: string, text: string, field: "group" | "break") => string, seriesName: string): Grid {
  const categories: Grid["categories"] = [];
  const names = new Map<string, string>();
  const cells = new Map<string, number>();
  for (const row of rows) {
    if (!categories.some((c) => c.key === row.group.key)) categories.push({ key: row.group.key, label: label(row.group.key, row.group.label, "group") });
    const series = row.break?.key ?? "";
    if (!names.has(series)) names.set(series, row.break ? label(row.break.key, row.break.label, "break") : seriesName);
    cells.set(`${row.group.key}\u0000${series}`, row.value);
  }
  return {
    categories,
    series: [...names].map(([key, name]) => ({ name, values: categories.map((c) => cells.get(`${c.key}\u0000${key}`) ?? 0) })),
  };
}

function channel(hex: string, at: number) {
  const v = parseInt(hex.slice(at, at + 2), 16) / 255;
  return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
}
const luminance = (hex: string) => 0.2126 * channel(hex, 1) + 0.7152 * channel(hex, 3) + 0.0722 * channel(hex, 5);

/** WCAG contrast ratio of two #RRGGBB colors. */
export function contrast(a: string, b: string) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

/** Series colors come from the design tokens --cat-1..6 so charts follow the theme. */
export function palette(el: Element = document.documentElement): string[] {
  const style = getComputedStyle(el);
  return [1, 2, 3, 4, 5, 6].map((n) => style.getPropertyValue(`--cat-${n}`).trim()).filter(Boolean);
}
