import type { components } from "@/api/schema";
import { api } from "@/lib/api";

export type TimeEntry = components["schemas"]["TimeEntry"];
export type TimeEntryPage = components["schemas"]["TimeEntryPage"];

export function timeEntryPage(params: URLSearchParams, signal?: AbortSignal) {
  return api<TimeEntryPage>(`/api/v1/time-entries?${params}`, { signal });
}

export function formatMinutes(minutes: number) {
  return `${Math.floor(minutes / 60)}:${String(minutes % 60).padStart(2, "0")}`;
}

export function parseMinutes(value: string) {
  const input = value.trim();
  if (!input) return 0;
  const match = /^(\d{1,2}):([0-5]\d)$/.exec(input);
  if (match) return +match[1] * 60 + +match[2];
  if (/^\d{1,4}$/.test(input)) return +input;
  return NaN;
}
