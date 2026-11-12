import type { components } from "@/api/schema";
import type { Query } from "@tanstack/react-query";
import { api } from "@/lib/api";

export type Item = components["schemas"]["Item"];
export type ItemType = components["schemas"]["ItemType"];
export type ItemPage = components["schemas"]["ItemPage"];
export type Location = components["schemas"]["Location"];
export type ItemCreate = components["schemas"]["ItemCreate"];
export type ItemUpdate = components["schemas"]["ItemUpdate"];
export type ItemStatus = components["schemas"]["Status"];
export type SavedView = components["schemas"]["View"];
export const itemKeys = {
  all: ["items"] as const,
  detail: (id: string) => ["items", "detail", id] as const,
  root: (id: string) => ["items", "root", id] as const,
  children: (id: string) => ["items", "children", id] as const,
};

export async function getItem(id: string) {
  let etag: string | null = null;
  const item = await api<Item>(`/api/v1/items/${encodeURIComponent(id)}`, undefined, (headers) => {
    etag = headers.get("ETag");
  });
  return { item, etag };
}

export function isProjectDetail(query: Query) {
  return query.queryKey[0] === "items" && query.queryKey[1] === "detail"
    && (query.state.data as Awaited<ReturnType<typeof getItem>> | undefined)?.item.type === "project";
}

export async function listItems(path: string, sort?: string, type?: string, field?: string): Promise<Item[]> {
  const items: Item[] = [];
  const seen = new Set<string>();
  let cursor: string | null | undefined;
  do {
    const params: URLSearchParams = new URLSearchParams();
    if (cursor) params.set("cursor", cursor);
    if (sort) params.set("sort", sort);
    if (type) params.set("type", type);
    if (field) params.set("field", field);
    const query = params.size ? `?${params.toString()}` : "";
    const page: ItemPage = await api<ItemPage>(path + query);
    items.push(...page.items);
    cursor = page.next_cursor;
    if (cursor && seen.has(cursor)) throw new Error("Repeated item cursor");
    if (cursor) seen.add(cursor);
  } while (cursor);
  return items;
}

export function isWorkday(value: string) {
  const day = new Date(`${value}T00:00:00Z`).getUTCDay();
  return day >= 1 && day <= 5;
}
