import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { sortSpaces, spaceKeys, type Space } from "@/lib/spaces";

export const useSpaces = (enabled = true) =>
  useQuery({ queryKey: spaceKeys.all, enabled, queryFn: async () => sortSpaces((await api<{ items: Space[] }>("/api/v1/spaces")).items) });

export const useSpace = (id: string) =>
  useQuery({ queryKey: spaceKeys.one(id), queryFn: () => api<Space>(`/api/v1/spaces/${id}`) });

export function useDebounced<T>(value: T, ms: number) {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => { const id = setTimeout(() => setDebounced(value), ms); return () => clearTimeout(id); }, [value, ms]);
  return debounced;
}
