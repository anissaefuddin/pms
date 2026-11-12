import type { CustomField } from "@/lib/custom-fields";

export const columnGroups = [
  { id: "management", columns: ["status", "assignee"] },
  { id: "schedule", columns: ["start_date", "due_date"] },
  { id: "other", columns: ["progress"] },
] as const;
export const defaultColumns = ["status", "assignee", "start_date", "due_date"];

export const fieldColumn = (id: string) => `field:${id}`;
export const fieldSort = (id: string) => `field:${id}`;
/** Multiple select and People have no single order, so the server refuses to sort by them. */
export const sortableField = (field: CustomField) => field.type !== "multi_select" && field.type !== "people" && !field.archived_at;

export function savedColumns(config?: Record<string, unknown> | null): string[] | undefined {
  const columns = config?.columns;
  return Array.isArray(columns) && columns.every((entry) => typeof entry === "string") ? columns : undefined;
}

export function visibleColumns(config: Record<string, unknown> | null | undefined, hasProjects: boolean) {
  return savedColumns(config) ?? [...defaultColumns, ...(hasProjects ? ["progress"] : [])];
}

export const fieldFilterRef = (field?: string) => {
  const [id, ...value] = (field ?? "").split(":");
  return { id: id ?? "", value: value.join(":") };
};
