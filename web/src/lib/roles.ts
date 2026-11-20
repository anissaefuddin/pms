import { useQuery } from "@tanstack/react-query";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";

export type CustomRole = components["schemas"]["CustomRole"];
export type Capability = components["schemas"]["Capability"];
export const CAPABILITIES: Capability[] = ["view", "comment", "change_status", "attach", "edit", "create", "delete", "share", "use_blueprint", "create_blueprint", "manage_blueprint"];

export const roleKeys = { all: ["custom-roles"] as const };
export const useCustomRoles = (enabled = true) => useQuery({
  queryKey: roleKeys.all, enabled, staleTime: 30_000,
  queryFn: async () => (await api<{ items: CustomRole[] }>("/api/v1/custom-roles")).items,
});
