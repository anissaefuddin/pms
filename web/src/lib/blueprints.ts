import { useQuery } from "@tanstack/react-query";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";

export type Blueprint = components["schemas"]["Blueprint"];

export const blueprintKey = (spaceID: string) => ["blueprints", spaceID] as const;

export const useBlueprints = (spaceID: string, enabled = true) =>
  useQuery({
    queryKey: blueprintKey(spaceID), enabled: enabled && !!spaceID,
    queryFn: async () => (await api<{ items: Blueprint[] }>(`/api/v1/spaces/${spaceID}/blueprints`)).items,
  });
