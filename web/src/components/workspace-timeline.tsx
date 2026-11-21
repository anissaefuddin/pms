import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronRight } from "lucide-react";
import { TIMELINE_DAYS, useOverviewTimeline, type TimelineProject, type TimelineSpace, type TimelineTask } from "@/lib/overview";
import { formatDate, useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { PeriodNav } from "@/components/overview-period-nav";

/** Read-only list of Projects and Tasks due in a range, grouped by Space. Nothing can be dragged or rescheduled here. */
export function WorkspaceTimeline() {
  const { t } = useTranslation();
  const [from, setFrom] = useState<string | null>(null);
  const q = useOverviewTimeline(from);
  if (q.isPending) return <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.loading")}</p>;
  if (q.isError) return <div><p role="alert">{t("dashboards.overview.error")}</p>
    <Button variant="outline" className="mt-3" onClick={() => q.refetch()}>{t("dashboards.overview.retry")}</Button></div>;
  const data = q.data;
  return <div className="space-y-4">
    <PeriodNav start={data.from} end={data.to} days={TIMELINE_DAYS} onMove={setFrom} />
    {data.truncated && <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.timeline.truncated")}</p>}
    {data.spaces.length === 0 && <p className="text-sm text-muted-foreground">{t("dashboards.overview.timeline.empty")}</p>}
    {data.spaces.map((space) => <SpaceBlock key={space.id} space={space} />)}
  </div>;
}

function SpaceBlock({ space }: { space: TimelineSpace }) {
  const { t } = useTranslation();
  return <section aria-label={space.name} className="rounded-lg border bg-card">
    <header className="flex items-center justify-between gap-3 border-b px-4 py-2">
      <h2 className="text-sm font-semibold"><Link to="/spaces/$spaceId" params={{ spaceId: space.id }} className="underline">{space.name}</Link></h2>
      <span className="text-xs text-muted-foreground">{t(`dashboards.overview.access.${space.effective_access}`)}</span>
    </header>
    <ul className="divide-y">
      {space.projects.map((project) => <ProjectRow key={project.id} project={project} />)}
      {space.unassigned_tasks.length > 0 && <li className="px-4 py-2">
        <p className="text-xs font-medium text-muted-foreground">{t("dashboards.overview.timeline.unassigned")}</p>
        <TaskList tasks={space.unassigned_tasks} />
      </li>}
    </ul>
  </section>;
}

function ProjectRow({ project }: { project: TimelineProject }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const [open, setOpen] = useState(false);
  const Chevron = open ? ChevronDown : ChevronRight;
  return <li className="px-4 py-2">
    <button type="button" aria-expanded={open} onClick={() => setOpen(!open)}
      className="flex w-full items-center gap-2 rounded-sm text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
      <Chevron className="size-4 shrink-0" aria-hidden />
      <span className="flex-1 font-medium">{project.title}</span>
      <span className="text-xs text-muted-foreground">{t("dashboards.overview.timeline.milestone", { date: formatDate(project.due_date, profile) })}</span>
      <span className="text-xs text-muted-foreground">{t(`dashboards.overview.status.${project.status_group}`)}</span>
    </button>
    {open && <div className="ml-6 mt-2">
      {project.tasks.length === 0
        ? <p className="text-xs text-muted-foreground">{t("dashboards.overview.timeline.noTasks")}</p>
        : <TaskList tasks={project.tasks} />}
    </div>}
  </li>;
}

function TaskList({ tasks }: { tasks: TimelineTask[] }) {
  const profile = useProfile();
  return <ul className="space-y-1 text-sm">
    {tasks.map((task) => <li key={task.id} className="flex items-center justify-between gap-3">
      <span className="min-w-0 truncate">{task.title}</span>
      <span className="whitespace-nowrap text-xs text-muted-foreground">{formatDate(task.due_date, profile)}</span>
    </li>)}
  </ul>;
}
