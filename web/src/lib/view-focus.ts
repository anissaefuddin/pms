// Focus mode is one app-wide setting, like the sidebar toggle: it hides the Space header and collapses the sidebar
// on every page, and it stays on until the user turns it off.
const storageKey = "pms:focus";
export const FOCUS_EVENT = "pms:view-focus";

export function readFocus(): boolean {
  try { return localStorage.getItem(storageKey) === "1"; } catch { return false; } // storage can be blocked
}

export function setFocus(next: boolean, persist = true) {
  document.documentElement.dataset.viewFocus = String(next);
  if (persist) {
    try { localStorage.setItem(storageKey, next ? "1" : "0"); } catch { /* storage can be blocked */ }
  }
  window.dispatchEvent(new CustomEvent(FOCUS_EVENT, { detail: next }));
}
