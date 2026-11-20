import type { QueryClient } from "@tanstack/react-query";
import { api, ApiError, type CurrentUser } from "@/lib/api";
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

/**
 * Creates an item with a client id, so a retry after a lost response cannot store a second item. On 409 for that id the
 * item may already exist from the first attempt: it is accepted only when it is this user's item with the same title.
 * Otherwise the 409 is shown. Every item list is refreshed either way.
 */
export async function createItemOnce(qc: QueryClient, input: ItemCreate & { id: string }, me: CurrentUser): Promise<Item> {
  const refresh = () => qc.invalidateQueries({ queryKey: itemKeys.all, predicate: ({ queryKey }) => queryKey[1] !== "detail" });
  try {
    const item = await api<Item>("/api/v1/items", { method: "POST", body: JSON.stringify(input) });
    await refresh();
    return item;
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      const existing = await api<Item>(`/api/v1/items/${input.id}`).catch(() => undefined);
      if (existing && existing.title === input.title && existing.created_by?.id === me.id) {
        await refresh();
        return existing;
      }
    }
    throw err;
  }
}
