import { createContext, useContext, useEffect } from "react";
import type { CurrentUser } from "@/lib/api";

export const ProfileContext = createContext<CurrentUser | undefined>(undefined);
export const useProfile = () => useContext(ProfileContext);

export function useAppearance(profile?: CurrentUser) {
  const theme = profile?.theme;
  const density = profile?.density;
  useEffect(() => {
    if (!theme && !density) return;
    const root = document.documentElement;
    const preference = window.matchMedia("(prefers-color-scheme: dark)");
    const previousDensity = root.dataset.density;
    const previousDark = root.classList.contains("dark");
    function apply() {
      if (density) root.dataset.density = density;
      if (theme) root.classList.toggle("dark", theme === "dark" || theme === "system" && preference.matches);
    }
    apply();
    preference.addEventListener("change", apply);
    return () => {
      preference.removeEventListener("change", apply);
      if (previousDensity === undefined) delete root.dataset.density;
      else root.dataset.density = previousDensity;
      root.classList.toggle("dark", previousDark);
    };
  }, [theme, density]);
}

export function formatDate(value?: string | null, profile?: CurrentUser) {
  if (!value) return "";
  const date = /^\d{4}-\d{2}-\d{2}$/.test(value);
  let [year, month, day] = value.split("-");
  if (!date) {
    const formatter = new Intl.DateTimeFormat(profile?.locale ?? "en", {
      day: "2-digit", month: "2-digit", year: "numeric", timeZone: profile?.timezone ?? "UTC",
    });
    const parts = Object.fromEntries(formatter.formatToParts(new Date(value)).map(({ type, value }) => [type, value]));
    ({ year, month, day } = parts);
  }
  if (profile?.date_format === "dd/mm/yyyy") return `${day}/${month}/${year}`;
  if (profile?.date_format === "mm/dd/yyyy") return `${month}/${day}/${year}`;
  return `${year}-${month}-${day}`;
}

export function formatDateTime(value: string, profile?: CurrentUser) {
  const time = new Intl.DateTimeFormat(profile?.locale ?? "en", {
    hour: "2-digit", minute: "2-digit", timeZone: profile?.timezone ?? "UTC",
    hourCycle: profile?.time_format === "12h" ? "h12" : "h23",
  }).format(new Date(value));
  return `${formatDate(value, profile)} ${time}`;
}

export function formatNumber(value: number, profile?: CurrentUser) {
  return new Intl.NumberFormat(profile?.locale ?? "en").format(value);
}
