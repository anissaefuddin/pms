import { useSyncExternalStore } from "react";

/** Sidebar colour themes from the Settings V2 prototype. Stored per browser (localStorage), not on the server. */
export type SidebarTheme = { id: string; name: string; nav: string; text: string; muted: string; hover: string; active: string; accent: string };

export const SIDEBAR_THEMES: SidebarTheme[] = [
  { id: "losaka-navy", name: "Losaka Navy", nav: "#192238", text: "#e2e8f3", muted: "#a5b3c9", hover: "#ffffff14", active: "#354866", accent: "#2563eb" },
  { id: "arctic", name: "Arctic", nav: "#edf1f6", text: "#243247", muted: "#5f6d82", hover: "#00000010", active: "#d6e1ee", accent: "#2563eb" },
  { id: "slate", name: "Slate", nav: "#344155", text: "#e6ebf2", muted: "#aab6c8", hover: "#ffffff14", active: "#4a5a72", accent: "#3c75b8" },
  { id: "forest", name: "Forest", nav: "#153d36", text: "#e3f2ee", muted: "#9cc7bb", hover: "#ffffff14", active: "#24584e", accent: "#16834d" },
  { id: "indigo", name: "Indigo", nav: "#272951", text: "#e8e9f8", muted: "#aeb0d6", hover: "#ffffff14", active: "#3b3e72", accent: "#6557ce" },
  { id: "sand", name: "Sand", nav: "#ece8dd", text: "#3b3324", muted: "#7a6e58", hover: "#00000010", active: "#d9d1bd", accent: "#956b35" },
];

const KEY = "lens-sidebar-theme";
const DEFAULT = SIDEBAR_THEMES[0].id;
const listeners = new Set<() => void>();

function read(): string {
  try {
    const value = globalThis.localStorage?.getItem(KEY);
    return SIDEBAR_THEMES.some((theme) => theme.id === value) ? value! : DEFAULT;
  } catch {
    return DEFAULT;
  }
}

export function setSidebarTheme(id: string) {
  try { globalThis.localStorage?.setItem(KEY, id); } catch { /* private mode: keep the in-memory choice */ }
  listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}

export function useSidebarTheme(): SidebarTheme {
  const id = useSyncExternalStore(subscribe, read, () => DEFAULT);
  return SIDEBAR_THEMES.find((theme) => theme.id === id) ?? SIDEBAR_THEMES[0];
}

/** CSS variables for the sidebar. They are set inline, so they override the default navy tokens. */
export function sidebarThemeVars(theme: SidebarTheme): Record<string, string> {
  return { "--nav": theme.nav, "--nav-text": theme.text, "--nav-muted": theme.muted, "--nav-hover": theme.hover, "--nav-active": theme.active };
}
