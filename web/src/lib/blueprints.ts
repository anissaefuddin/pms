import { useQuery } from "@tanstack/react-query";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";

export type Blueprint = components["schemas"]["Blueprint"];
export type BlueprintTree = components["schemas"]["BlueprintTree"];
export type BlueprintPreviewItem = components["schemas"]["BlueprintPreviewItem"];
export type BlueprintDetail = Blueprint & { tree: BlueprintTree };
export type BlueprintNode = BlueprintTree["nodes"][number];

export const blueprintKey = (spaceID: string) => ["blueprints", spaceID] as const;
export const blueprintDetailKey = (id: string) => ["blueprint", id] as const;

export const useBlueprints = (spaceID: string, enabled = true) =>
  useQuery({
    queryKey: blueprintKey(spaceID), enabled: enabled && !!spaceID,
    queryFn: async () => (await api<{ items: Blueprint[] }>(`/api/v1/spaces/${spaceID}/blueprints`)).items,
  });

export const useBlueprintDetail = (id: string) =>
  useQuery({
    queryKey: blueprintDetailKey(id), enabled: !!id,
    queryFn: () => api<BlueprintDetail>(`/api/v1/blueprints/${id}`),
  });

/** Save a blueprint's name and/or tree. `version` is the one the editor read; a different one is a 409 (ApiError). */
export const saveBlueprint = (id: string, body: { version: number; name?: string; tree?: BlueprintTree }) =>
  api<Blueprint>(`/api/v1/blueprints/${id}`, { method: "PATCH", body: JSON.stringify(body) });

/** Preview what using the blueprint with these dates would create. Writes nothing. */
export const previewBlueprint = (id: string, body: { start_date?: string; finish_date?: string }) =>
  api<{ items: BlueprintPreviewItem[] }>(`/api/v1/blueprints/${id}/preview`, { method: "POST", body: JSON.stringify(body) });
