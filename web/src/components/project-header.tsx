import { Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { Info, MoreHorizontal, Plus, Share2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import { useSpace } from "@/lib/space-queries";
import type { Item } from "@/lib/items";
import { projectProgressPercent, upcomingMilestones, workItemCounts } from "@/lib/workspace-metrics";
import { Button } from "@/components/ui/button";
import { AvatarStack } from "@/components/ui/avatar-stack";
import { KpiCard } from "@/components/ui/kpi-card";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { StarButton } from "@/components/star-button";

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
  const profile = useProfile();
  const spaceId = item.locations[0]?.space_id ?? "";
  const space = useSpace(spaceId);
  const workflows = useQuery({
    queryKey: ["workflows"], queryFn: () => api<{ items: components["schemas"]["Workflow"][] }>("/api/v1/workflows"),
  });
  const workflow = workflows.data?.items.find((wf) => wf.id === item.default_workflow_id);
  const kids = children;
  const counts = workItemCounts(kids);
  const progress = projectProgressPercent(item, kids);
  const today = new Date().toISOString().slice(0, 10);
  const milestones = upcomingMilestones(kids, today);
  const names = item.assignees.map((person) => person.name);
  const range = [item.start_date, item.due_date].filter(Boolean).map((d) => formatDate(d, profile)).join(" – ");
  const meta = [workflow?.name, range].filter(Boolean).join(" · ");
  const percent = (value: number) => t("views.project.percent", { value });
  const milestoneHint = milestones[0]
    ? `${milestones[0].title} · ${formatDate(milestones[0].due_date, profile)}`
    : t("views.project.noMilestones");

  return <header className="space-y-4 border-b pb-5">
    <nav aria-label={t("views.project.breadcrumb")}>
      <ol className="flex flex-wrap items-center gap-1 text-sm text-muted-foreground">
        <li>{space.data && <Link to="/spaces/$spaceId" params={{ spaceId }} className="hover:text-foreground hover:underline">{space.data.name}</Link>}</li>
        <li aria-hidden="true">/</li>
        <li aria-current="page" className="min-w-0 truncate text-foreground">{item.title}</li>
      </ol>
    </nav>
    <div className="flex flex-wrap items-start justify-between gap-4">
      <div className="min-w-0">
        <div className="flex min-w-0 items-center gap-2">
          <h1 className="truncate workspace-title">{item.title}</h1>
          <StarButton itemId={item.id} />
        </div>
        {meta && <p className="mt-1 text-sm text-muted-foreground">{meta}</p>}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <AvatarStack names={names} max={4} />
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
      </div>
    </div>
    <div className="-mx-4 flex gap-3 overflow-x-auto px-4 pb-1 md:mx-0 md:grid md:grid-cols-4 md:overflow-visible md:px-0">
      <KpiCard className="min-w-44 shrink-0 md:min-w-0" label={t("views.project.progress")} value={percent(progress)} />
      <KpiCard className="min-w-44 shrink-0 md:min-w-0" label={t("views.project.workItems")} value={counts.total}
        hint={t("views.project.done", { count: counts.done })} />
      <KpiCard className="min-w-44 shrink-0 md:min-w-0" label={t("views.project.inProgress")} value={counts.inProgress} />
      <KpiCard className="min-w-44 shrink-0 md:min-w-0" label={t("views.project.milestones")} value={milestones.length} hint={milestoneHint} />
    </div>
  </header>;
}
