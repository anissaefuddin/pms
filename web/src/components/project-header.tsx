import { Link } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronDown, Info, MoreHorizontal, Plus, Share2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import { useSpace } from "@/lib/space-queries";
import { itemKeys, listSubtree, subtreeChildren, type Item } from "@/lib/items";
import { inProgressStatusIds, projectProgressPercent, tallyCounts, upcomingMilestones } from "@/lib/workspace-metrics";
import type { components as analyticsTypes } from "@/api/schema";
import { useId, useState } from "react";
import { Button } from "@/components/ui/button";
import { AvatarStack } from "@/components/ui/avatar-stack";
import { KpiCard } from "@/components/ui/kpi-card";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { WorkspaceHeader } from "@/components/workspace-header";
import { ItemPathBreadcrumb, useItemAncestors } from "@/components/item-path";
import { TypeIcon } from "@/components/type-icon";

/** Header project: breadcrumb, judul dengan Star, meta, anggota, aksi, dan empat KPI. */
export function ProjectHeader({ item, children, canCreate, infoOpen, onToggleInfo, onShare, onAddTask }: {
  item: Item;
  children: Item[];
  canCreate: boolean;
  infoOpen: boolean;
  onToggleInfo: () => void;
  onShare?: () => void;
  onAddTask: () => void;
}) {
  const { t } = useTranslation();
  const [detailsOpen, setDetailsOpen] = useState(false);
  const profile = useProfile();
  const spaceId = item.locations[0]?.space_id ?? "";
  const space = useSpace(spaceId);
  const workflows = useQuery({
    queryKey: ["workflows"], queryFn: () => api<{ items: components["schemas"]["Workflow"][] }>("/api/v1/workflows"),
  });
  const workflow = workflows.data?.items.find((wf) => wf.id === item.default_workflow_id);
  const kids = children;
  // F-01 / WA-D03: executable tasks across the whole project subtree, milestones excluded, counted by the server (not paginated here).
  const tally = useQuery({
    queryKey: ["project-tally", item.id, workflows.data?.items.length ?? 0],
    enabled: !!workflows.data,
    queryFn: () => api<analyticsTypes["schemas"]["AnalyticsResult"]>("/api/v1/analytics/query", { method: "POST", body: JSON.stringify({
      reporting_on: "tasks", metric: "count", group_by: "status_id", container_ids: [item.id], filters: { exclude_milestones: true },
    }) }),
  });
  const groups = new Map((workflows.data?.items ?? []).flatMap((wf) => wf.statuses.map((status) => [status.id, status.group] as const)));
  const counts = tally.data
    ? tallyCounts(tally.data.rows.map((row) => ({ key: row.group.key, value: row.value })), groups, inProgressStatusIds(workflows.data?.items ?? []))
    : undefined;
  // "…" while loading; "—" when the count failed, so a failure is not mistaken for a slow load.
  const placeholder = tally.isError ? "—" : "…";
  const progress = projectProgressPercent(item, kids);
  const today = new Date().toISOString().slice(0, 10);
  // A2: milestones anywhere in the subtree. The walk is shared with the Board and Calendar; the server has no milestone count.
  const qc = useQueryClient();
  const subtree = useQuery({
    queryKey: [...itemKeys.all, "milestones", item.id, kids.map((kid) => kid.id).join(",")],
    enabled: kids.length > 0,
    queryFn: () => listSubtree(kids, (id) => subtreeChildren(qc, id)),
  });
  const milestones = upcomingMilestones(subtree.data ?? [], today);
  const range = [item.start_date, item.due_date].filter(Boolean).map((d) => formatDate(d, profile)).join(" – ");
  // The Customer comes from the project's Space. A Space that is not a Customer Space is Internal; an archived Customer is marked.
  const customer = space.data?.customer;
  const customerLabel = !space.data ? "…"
    : customer ? `${customer.name}${customer.archived ? ` (${t("views.project.archivedCustomer")})` : ""}`
    : t("views.project.internal");
  const meta = [workflow?.name, range].filter(Boolean).join(" · ");
  const detailsId = useId();
  const percent = (value: number) => t("views.project.percent", { value });
  const milestoneHint = milestones[0]
    ? `${milestones[0].title} · ${formatDate(milestones[0].due_date, profile)}`
    : t("views.project.noMilestones");

  const ancestors = useItemAncestors(item);
  const breadcrumb = <ItemPathBreadcrumb item={item} ancestors={ancestors.data ?? []} />;
  const actions = <>
    <AvatarStack people={item.assignees} max={4} />
    <Button variant="outline" aria-pressed={infoOpen} onClick={onToggleInfo} className="max-md:hidden">
      <Info />{t("views.project.info")}
    </Button>
    {onShare && <Button variant="outline" onClick={onShare} className="max-md:hidden"><Share2 />{t("views.project.share")}</Button>}
    {canCreate && <Button onClick={onAddTask}><Plus />{t("views.project.addTask")}</Button>}
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="ghost" size="icon" aria-label={t("views.project.more")} className="md:hidden" />}><MoreHorizontal /></DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={onToggleInfo}>{t("views.project.info")}</DropdownMenuItem>
        {onShare && <DropdownMenuItem onClick={onShare}>{t("views.project.share")}</DropdownMenuItem>}
      </DropdownMenuContent>
    </DropdownMenu>
  </>;
  return <WorkspaceHeader breadcrumb={breadcrumb} icon={<TypeIcon name="folder-kanban" />} title={item.title} starItemId={item.id}
    meta={meta ? <p className="mt-1 text-sm text-muted-foreground">{meta}</p> : undefined} actions={actions}>
    {/* B5-01/B5-04: Customer is derived from the Space that holds the project; the Project Manager is the project owner. */}
    <dl className="flex flex-wrap gap-x-6 gap-y-1 text-sm">
      <div className="flex gap-1"><dt className="text-muted-foreground">{t("views.project.customer")}</dt>
        <dd className="font-medium">{customerLabel}</dd></div>
      <div className="flex gap-1"><dt className="text-muted-foreground">{t("views.project.projectManager")}</dt>
        <dd className="font-medium">{item.owner?.name ?? t("views.project.unassigned")}</dd></div>
      <div className="flex gap-1"><dt className="text-muted-foreground">{t("views.project.health")}</dt>
        <dd className="font-medium">{item.progress_risk ? t(`items.progress.risk.${item.progress_risk}`) : t("views.project.noHealth")}</dd></div>
      <div className="flex gap-1"><dt className="text-muted-foreground">{t("views.project.lastUpdate")}</dt>
        <dd className="font-medium tabular-nums">{item.updated_at ? formatDate(item.updated_at.slice(0, 10), profile) : "—"}</dd></div>
    </dl>
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
      <span className="tabular-nums"><span className="text-muted-foreground">{t("views.project.progress")} </span><strong className="font-semibold">{percent(progress)}</strong></span>
      <span className="tabular-nums"><span className="text-muted-foreground">{t("views.project.workItems")} </span><strong className="font-semibold">{counts?.total ?? placeholder}</strong></span>
      <span className="tabular-nums"><span className="text-muted-foreground">{t("views.project.inProgress")} </span><strong className="font-semibold">{counts?.inProgress ?? placeholder}</strong></span>
      <span className="tabular-nums"><span className="text-muted-foreground">{t("views.project.milestones")} </span><strong className="font-semibold">{milestones.length}</strong></span>
      <Button variant="ghost" size="sm" aria-expanded={detailsOpen} aria-controls={detailsId} onClick={() => setDetailsOpen((open) => !open)}>
        {t("views.project.summary")}<ChevronDown className={detailsOpen ? "rotate-180" : undefined} aria-hidden="true" />
      </Button>
    </div>
    {/* F1: the scope is visible next to the numbers, not only in the Summary details. */}
    {counts && <p className="-mt-2 text-xs text-muted-foreground">{t("views.project.subtreeScope")}</p>}
    {detailsOpen && <div id={detailsId} className="-mx-4 flex gap-3 overflow-x-auto px-4 pb-1 md:mx-0 md:grid md:grid-cols-4 md:overflow-visible md:px-0">
      <KpiCard className="min-w-44 shrink-0 md:min-w-0" label={t("views.project.progress")} value={percent(progress)} />
      <KpiCard className="min-w-44 shrink-0 md:min-w-0" label={t("views.project.workItems")} value={counts?.total ?? "…"}
        hint={counts ? `${t("views.project.done", { count: counts.done })} · ${t("views.project.subtreeScope")}` : undefined} />
      <KpiCard className="min-w-44 shrink-0 md:min-w-0" label={t("views.project.inProgress")} value={counts?.inProgress ?? "…"} />
      <KpiCard className="min-w-44 shrink-0 md:min-w-0" label={t("views.project.milestones")} value={milestones.length} hint={milestoneHint} />
    </div>}
  </WorkspaceHeader>;
}
