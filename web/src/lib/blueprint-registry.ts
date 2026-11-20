// Pure helpers for the Settings → Blueprints registry: filter, sort, and group rows loaded from the API.

export type RegistryRow = {
  id: string;
  name: string;
  item_type: string;
  scope: string;
  space_id: string;
  spaceName: string;
  item_count: number;
  updated_at: string;
  canManage: boolean;
};

export type RegistryFilter = { query: string; type: string; scope: string };
export type RegistrySort = "name" | "updated_desc" | "updated_asc";
export type RegistryGroup = "none" | "scope" | "space" | "type";

export function filterRows(rows: RegistryRow[], f: RegistryFilter): RegistryRow[] {
  const q = f.query.trim().toLowerCase();
  return rows.filter((row) =>
    (f.type === "all" || row.item_type === f.type)
    && (f.scope === "all" || row.scope === f.scope)
    && (q === "" || row.name.toLowerCase().includes(q) || row.spaceName.toLowerCase().includes(q)));
}

export function sortRows(rows: RegistryRow[], sort: RegistrySort): RegistryRow[] {
  const out = [...rows];
  if (sort === "name") out.sort((a, b) => a.name.localeCompare(b.name) || a.id.localeCompare(b.id));
  else if (sort === "updated_desc") out.sort((a, b) => b.updated_at.localeCompare(a.updated_at) || a.id.localeCompare(b.id));
  else out.sort((a, b) => a.updated_at.localeCompare(b.updated_at) || a.id.localeCompare(b.id));
  return out;
}

/** Groups keep the order of first appearance, so the caller's sort also orders the groups. */
export function groupRows(rows: RegistryRow[], by: RegistryGroup): { key: string; rows: RegistryRow[] }[] {
  if (by === "none") return [{ key: "", rows }];
  const groups = new Map<string, RegistryRow[]>();
  for (const row of rows) {
    const key = by === "scope" ? row.scope : by === "type" ? row.item_type : row.space_id;
    groups.set(key, [...(groups.get(key) ?? []), row]);
  }
  return [...groups.entries()].map(([key, list]) => ({ key, rows: list }));
}
