import type { components } from "@/api/schema";

export type Workflow = components["schemas"]["Workflow"];
export type WorkflowStatus = components["schemas"]["Status"];

/** The fixed palette tokens the API accepts; static class strings so Tailwind keeps them. */
export const statusColors = {
  slate: "bg-slate-500", blue: "bg-blue-500", sky: "bg-sky-500", teal: "bg-teal-500",
  green: "bg-green-500", lime: "bg-lime-500", amber: "bg-amber-500", orange: "bg-orange-500",
  red: "bg-red-500", pink: "bg-pink-500", purple: "bg-purple-500", indigo: "bg-indigo-500",
} as const;

export const statusColorTokens = Object.keys(statusColors) as (keyof typeof statusColors)[];
export const statusGroups = ["active", "completed", "deferred", "cancelled"] as const;

export const statusDotClass = (token: string) => statusColors[token as keyof typeof statusColors] ?? statusColors.slate;

/** Returns ids with `id` moved one step (-1 up, +1 down); unchanged at the edges. */
export function moveId(ids: string[], id: string, delta: -1 | 1): string[] {
  const from = ids.indexOf(id), to = from + delta;
  if (from < 0 || to < 0 || to >= ids.length) return ids;
  const next = [...ids];
  next.splice(to, 0, ...next.splice(from, 1));
  return next;
}
