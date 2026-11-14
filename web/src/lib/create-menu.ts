import { useQuery } from "@tanstack/react-query";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";

export type CreateMenu = components["schemas"]["CreateMenu"];
export const createMenuKey = (spaceID: string) => ["create-menu", spaceID] as const;

/** Item types the "+" menu of a Space offers (SPC-14). A convenience only: the API accepts every type. */
export const useCreateMenu = (spaceID: string, enabled = true) =>
  useQuery({
    queryKey: createMenuKey(spaceID), enabled: enabled && !!spaceID,
    queryFn: () => api<CreateMenu>(`/api/v1/spaces/${spaceID}/create-menu`),
  });

/** Without a loaded menu every type is offered, which is also today's behaviour for a Space nobody configured. */
export const menuAllows = (menu: CreateMenu | undefined, key: string) => menu?.items.find((entry) => entry.key === key)?.enabled !== false;
