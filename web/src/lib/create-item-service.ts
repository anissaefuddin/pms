import type { QueryClient } from "@tanstack/react-query";
import { api, type CurrentUser } from "@/lib/api";
import { itemKeys, type Item, type ItemCreate } from "@/lib/items";

/**
 * Payload for a quick Task (name plus location, optional status). It uses the same endpoint and the same
 * user defaults as the Create dialog (SET-03), so both paths store the same item.
 */
export function quickTaskInput(me: CurrentUser, spaceId: string, parentId: string | undefined, title: string, statusId?: string): ItemCreate {
  const input: ItemCreate = { type: "task", title, locations: [{ space_id: spaceId, parent_item_id: parentId || null }] };
  if (statusId) input.status_id = statusId;
  if (me.default_task_duration_days != null) input.duration_days = me.default_task_duration_days;
  if (me.default_task_effort_minutes != null) input.effort_minutes = me.default_task_effort_minutes;
  return input;
}

/** Posts the item and refreshes every item list, so the caller can check whether the new item is in the current list. */
export async function createQuickTask(qc: QueryClient, input: ItemCreate): Promise<Item> {
  const item = await api<Item>("/api/v1/items", { method: "POST", body: JSON.stringify(input) });
  await qc.invalidateQueries({ queryKey: itemKeys.all, predicate: ({ queryKey }) => queryKey[1] !== "detail" });
  return item;
}
