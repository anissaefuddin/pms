// Pure rules for the reusable Customer Picker (B2-08). No imports, so node --test can check them without the app.

/** Options requested per lookup. The server caps at 200; the picker asks for less and says so when the page is full. */
export const CUSTOMER_PICKER_LIMIT = 50;

export type CustomerLookupScope = "membership" | "administrative";

/**
 * Query key for a lookup. The signed-in user and the scope are part of the key, so one user's options are never served
 * to another user or to the other lookup mode. Options are kept in memory only (never localStorage) and are cleared on logout.
 */
export function eligibleCustomersKey(userId: string, scope: CustomerLookupScope, search: string) {
  return ["clients", "eligible", userId, scope, search] as const;
}

export type SelectionStatus = "none" | "selectable" | "unavailable" | "unknown";

/**
 * What the picker may say about the selected Customer id.
 * - none: nothing selected.
 * - selectable: the id is in the options the server returned for this lookup.
 * - unavailable: the full, unfiltered, non-truncated list was loaded and the id is not in it. The id is not a valid
 *   selection, and the picker never swaps in another Customer.
 * - unknown: the list cannot prove anything (still loading, searching, or a full page). Stated as unknown, not as valid.
 */
export function selectionStatus(
  value: string | null,
  options: readonly { id: string }[],
  state: { loaded: boolean; searching: boolean; truncated: boolean },
): SelectionStatus {
  if (!value) return "none";
  if (options.some((option) => option.id === value)) return "selectable";
  if (!state.loaded || state.searching || state.truncated) return "unknown";
  return "unavailable";
}

/** True when a page is as large as the requested limit, so more Customers may exist than are shown. */
export function isTruncated(count: number, limit = CUSTOMER_PICKER_LIMIT) {
  return count >= limit;
}
