/** Keeps persisted column widths usable after minimum-width rules change. */
export function clampWidths(widths: Record<string, number>, minimums: Record<string, number>): Record<string, number> {
  return Object.fromEntries(Object.entries(widths).map(([id, width]) => [id, Math.max(minimums[id] ?? 80, width)]));
}

export function clampedWidth(id: string, widths: Record<string, number>, defaults: Record<string, number>, minimums: Record<string, number>): number {
  return Math.max(minimums[id] ?? 80, widths[id] ?? defaults[id] ?? 128);
}
