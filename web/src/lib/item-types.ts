import { useQuery } from "@tanstack/react-query";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";

export type ItemTypeDef = components["schemas"]["ItemTypeDef"];
export type ItemTypeInput = components["schemas"]["ItemTypeCreate"];
export const itemTypeKeys = { all: ["item-types"] as const };

/** All item types (built-in and custom). Anyone signed in may read them; only admins change them. */
export function useItemTypes(enabled = true) {
  return useQuery({ queryKey: itemTypeKeys.all, enabled, staleTime: 60_000,
    queryFn: async () => (await api<{ items: ItemTypeDef[] }>("/api/v1/item-types")).items });
}

/** The panel blocks a layout can place; the API accepts the same names plus custom field ids. */
export const LAYOUT_BLOCKS = ["status", "importance", "dates", "assignees", "effort", "description", "locations", "subitems", "attachments", "comments", "approvals", "activity"] as const;
