// Logika murni untuk header project, KPI, Gantt, dan halaman Blueprints (tanpa import agar bisa dites dengan node --test).

export type MetricItem = {
  id: string;
  type?: string;
  is_milestone?: boolean;
  progress_percent?: number | null;
  due_date?: string | null;
  status?: { id?: string; group?: string } | null;
};

export const isDone = (item: MetricItem) => item.status?.group === "completed";
/** Active = any status in the workflow's active group, including the initial status (WA-D02). */
export const isActive = (item: MetricItem) => item.status?.group === "active";

/** Persentase item selesai dari daftar anak; 0 bila kosong. */
export function completionPercent(items: MetricItem[]): number {
  if (items.length === 0) return 0;
  return Math.round((items.filter(isDone).length / items.length) * 100);
}

/** Progres project: nilai tersimpan dari server bila ada, selain itu dihitung dari anak. */
export function projectProgressPercent(project: Pick<MetricItem, "progress_percent">, children: MetricItem[]): number {
  return project.progress_percent ?? completionPercent(children);
}

/**
 * Stable ids of the In progress statuses: every active status of a workflow except its initial one (the first active status in order).
 * Matched by id, never by name (WA-D02). An active status the workflow does not define is never counted.
 */
export function inProgressStatusIds(workflows: { statuses: { id: string; group: string }[] }[]): Set<string> {
  const ids = new Set<string>();
  for (const workflow of workflows) {
    workflow.statuses.filter((status) => status.group === "active").slice(1).forEach((status) => ids.add(status.id));
  }
  return ids;
}

/** In progress: the item's status id is one of the workflow's non-initial active statuses. Empty input gives 0 (WA-D02). */
export const isInProgress = (item: MetricItem, inProgressIds: ReadonlySet<string>) =>
  !!item.status?.id && inProgressIds.has(item.status.id);

/** KPI counts for a project's direct children. `active` counts every active item; `inProgress` only the non-initial ones. */
export function workItemCounts(children: MetricItem[], inProgressIds: ReadonlySet<string> = new Set()) {
  return {
    total: children.length,
    done: children.filter(isDone).length,
    active: children.filter(isActive).length,
    inProgress: children.filter((item) => isInProgress(item, inProgressIds)).length,
  };
}

/** Milestone yang belum selesai dan jatuh tempo pada `today` (YYYY-MM-DD) atau sesudahnya. */
export function upcomingMilestones<T extends MetricItem>(children: T[], today: string): T[] {
  return children
    .filter((item) => item.is_milestone && !isDone(item) && !!item.due_date && item.due_date >= today)
    .sort((a, b) => (a.due_date ?? "").localeCompare(b.due_date ?? ""));
}

export type BlueprintLike = { id: string; name: string; space_id: string; item_type: string; item_count: number };

/** Blueprint dikelompokkan per Space sesuai urutan Space; Space tanpa blueprint tetap muncul dengan daftar kosong. */
export function groupBlueprintsBySpace<T extends BlueprintLike>(blueprints: T[], spaces: { id: string; name: string }[]) {
  return spaces.map((space) => ({
    spaceId: space.id,
    spaceName: space.name,
    items: blueprints.filter((blueprint) => blueprint.space_id === space.id).sort((a, b) => a.name.localeCompare(b.name)),
  }));
}

export type SuggestedBlueprint = { id: "software" | "client" | "internal"; name: string; description: string; itemType: "project" | "folder" };

/** Tiga titik awal di sisi klien; hanya mengisi modal, tidak disimpan ke server. */
export const suggestedBlueprints: SuggestedBlueprint[] = [
  { id: "software", name: "Software delivery", description: "Sprint, backlog, and release milestones for a product team.", itemType: "project" },
  { id: "client", name: "Client implementation", description: "Kickoff, configuration, training, and go-live for a client.", itemType: "project" },
  { id: "internal", name: "Internal initiative", description: "A folder of workstreams for a company-internal goal.", itemType: "folder" },
];

/** Token cat-* untuk bar Gantt berdasarkan grup status. */
export function ganttTone(group?: string): "cat-5" | "cat-2" | "cat-4" | "cat-3" | "cat-1" {
  switch (group) {
    case "completed": return "cat-2";
    case "deferred": return "cat-4";
    case "cancelled": return "cat-3";
    case "active": return "cat-5";
    default: return "cat-1";
  }
}
