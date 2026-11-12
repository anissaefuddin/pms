import type { components } from "@/api/schema";
import { api } from "@/lib/api";

export type Notification = components["schemas"]["Notification"];
export const notificationKey = ["notifications"] as const;

export function listNotifications(limit: number, unread = false, cursor?: string) {
  const params = new URLSearchParams({ limit: String(limit), unread_only: String(unread) });
  if (cursor) params.set("cursor", cursor);
  return api<components["schemas"]["NotificationPage"]>(`/api/v1/notifications?${params}`);
}
