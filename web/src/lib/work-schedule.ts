import type { components } from "@/api/schema";
import { api } from "@/lib/api";

export type WorkSchedule = components["schemas"]["WorkSchedule"];
export type WorkScheduleInput = components["schemas"]["WorkScheduleInput"];

/** One query for the work schedule, shared by the Calendar, the Effort block and the settings page: they all read the same cache entry,
 * so they must all store the same shape ({ data, etag }). Storing a different shape under this key crashed the page. */
export const workScheduleKey = ["work-schedule"] as const;
export async function loadWorkSchedule() {
  let etag = "";
  const data = await api<WorkSchedule>("/api/v1/settings/work-schedule", undefined, (headers) => { etag = headers.get("ETag") ?? ""; });
  return { data, etag };
}
