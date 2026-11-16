import { Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, Info, MoreHorizontal, Plus, Share2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import { useSpace } from "@/lib/space-queries";
import type { Item } from "@/lib/items";
import { inProgressStatusIds, projectProgressPercent, tallyCounts, upcomingMilestones } from "@/lib/workspace-metrics";
import type { components as analyticsTypes } from "@/api/schema";
import { useId, useState } from "react";
import { Button } from "@/components/ui/button";
import { AvatarStack } from "@/components/ui/avatar-stack";
import { KpiCard } from "@/components/ui/kpi-card";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { WorkspaceHeader } from "@/components/workspace-header";
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
  const progress = projectProgressPercent(item, kids);
  const today = new Date().toISOString().slice(0, 10);
  const milestones = upcomingMilestones(kids, today);
  const range = [item.start_date, item.due_date].filter(Boolean).map((d) => formatDate(d, profile)).join(" – ");
  const meta = [workflow?.name, range].filter(Boolean).join(" · ");
  const detailsId = useId();
  const percent = (value: number) => t("views.project.percent", { value });
  const milestoneHint = milestones[0]
    ? `${milestones[0].title} · ${formatDate(milestones[0].due_date, profile)}`
    : t("views.project.noMilestones");

  const breadcrumb = <nav aria-label={t("views.project.breadcrumb")}>
    <ol className="flex flex-wrap items-center gap-1 text-sm text-muted-foreground">
      <li>{space.data && <Link to="/spaces/$spaceId" params={{ spaceId }} className="hover:text-foreground hover:underline">{space.data.name}</Link>}</li>
      <li aria-hidden="true">/</li>
      <li aria-current="page" className="min-w-0 truncate text-foreground">{item.title}</li>
    </ol>
  </nav>;
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
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
      <span className="tabular-nums"><span className="text-muted-foreground">{t("views.project.progress")} </span><strong className="font-semibold">{percent(progress)}</strong></span>
      <span className="tabular-nums"><span className="text-muted-foreground">{t("views.project.workItems")} </span><strong className="font-semibold">{counts?.total ?? "…"}</strong></span>
      <span className="tabular-nums"><span className="text-muted-foreground">{t("views.project.inProgress")} </span><strong className="font-semibold">{counts?.inProgress ?? "…"}</strong></span>
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
