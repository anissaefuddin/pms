import type { QueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { itemKeys, type Item } from "@/lib/items";

export const recycleKey = ["recycle-bin"] as const;

export async function restoreItem(client: QueryClient, id: string) {
  const item = await api<Item>(`/api/v1/recycle-bin/${id}/restore`, { method: "POST" });
  await Promise.all([
    client.invalidateQueries({ queryKey: itemKeys.all }),
    client.invalidateQueries({ queryKey: recycleKey }),
    client.invalidateQueries({ queryKey: ["shared-items"] }),
    client.invalidateQueries({ queryKey: ["notifications"] }),
  ]);
  return item;
}

export async function emptyRecycleBin(client: QueryClient) {
  const result = await api<{ deleted: number }>("/api/v1/recycle-bin", { method: "DELETE" });
  await client.invalidateQueries({ queryKey: recycleKey });
  return result.deleted;
}
