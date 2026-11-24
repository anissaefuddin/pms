// Pure helpers for Gantt timeline create (Batch 3C). Kept free of React and imports so node --test can run them.

/** Pixels per day on the day scale (ItemGantt cellWidth). Drag and double-click work on this scale only. */
export const dayCellWidth = 36;
/** A pointer move shorter than this is a click, not a drag (so a double-click never becomes a range). */
export const dragThreshold = 4;

/**
 * Whole days from the timeline origin under a pointer, or null when the pointer is left of the origin.
 * clientX: pointer x; chartLeft: the timeline's left edge on screen; scrollLeft: its horizontal scroll.
 */
export function dayIndexAt(clientX: number, chartLeft: number, scrollLeft: number, cellWidth = dayCellWidth): number | null {
  const x = clientX - chartLeft + scrollLeft;
  if (x < 0) return null;
  return Math.floor(x / cellWidth);
}

/** True once the pointer has moved far enough to be a drag. */
export function isDragStep(fromX: number, toX: number, threshold = dragThreshold): boolean {
  return Math.abs(toX - fromX) >= threshold;
}

/** Inclusive range between two day indexes, in either order: { first, last } with first <= last. */
export function dayRange(a: number, b: number): { first: number; last: number } {
  return { first: Math.min(a, b), last: Math.max(a, b) };
}
