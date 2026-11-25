import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { sortSpaces, spaceKeys, type Space } from "@/lib/spaces";

export const useSpaces = (enabled = true) =>
  useQuery({ queryKey: spaceKeys.all, enabled, queryFn: async () => sortSpaces((await api<{ items: Space[] }>("/api/v1/spaces")).items) });

/** Item locations arrive without space_name: resolve it from the Spaces this user can see. Unknown stays undefined. */
export function useLocationName() {
  const { data } = useSpaces();
  return (location: { space_id: string; space_name?: string }) => location.space_name || data?.find((space) => space.id === location.space_id)?.name;
}

export const useSpace = (id: string) =>
  useQuery({ queryKey: spaceKeys.one(id), queryFn: () => api<Space>(`/api/v1/spaces/${id}`) });

export function useDebounced<T>(value: T, ms: number) {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => { const id = setTimeout(() => setDebounced(value), ms); return () => clearTimeout(id); }, [value, ms]);
  return debounced;
}
