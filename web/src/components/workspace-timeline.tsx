import { useState, type CSSProperties, type ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronRight } from "lucide-react";
import { addDays, TIMELINE_DAYS, useOverviewTimeline, type TimelineProject, type TimelineSpace, type TimelineTask } from "@/lib/overview";
import { formatDate, useProfile } from "@/lib/profile";
import type { CurrentUser } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { PeriodNav } from "@/components/overview-period-nav";

const DAY_PX = 32;
const LABEL_PX = 260;
const DAY_MS = 86_400_000;
const utc = (date: string) => Date.parse(`${date}T00:00:00Z`);
const todayKey = () => {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};
const isWeekend = (date: string) => [0, 6].includes(new Date(utc(date)).getUTCDay());
const dayNumber = (date: string) => new Date(utc(date)).getUTCDate();

/** Read-only Gantt-style grid: one column per day, Projects and Tasks placed on their due date. Nothing can be dragged here. */
export function WorkspaceTimeline() {
  const { t } = useTranslation();
  const profile = useProfile();
  const [from, setFrom] = useState<string | null>(null);
  const q = useOverviewTimeline(from);
  if (q.isPending) return <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.loading")}</p>;
  if (q.isError) return <div><p role="alert">{t("dashboards.overview.error")}</p>
    <Button variant="outline" className="mt-3" onClick={() => q.refetch()}>{t("dashboards.overview.retry")}</Button></div>;
  const data = q.data;
  const dates = Array.from({ length: TIMELINE_DAYS }, (_, i) => addDays(data.from, i));
  const ctx: Ctx = { from: data.from, dates, profile };
  return <div className="space-y-4">
    <PeriodNav start={data.from} end={data.to} days={TIMELINE_DAYS} onMove={setFrom} />
    <div className="flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
      <span className="flex items-center gap-1.5"><span className="h-3 w-5 rounded-md bg-sky-500" />{t("dashboards.overview.timeline.legend.project")}</span>
      <span className="flex items-center gap-1.5"><span className="h-3 w-5 rounded-md bg-muted-foreground/50" />{t("dashboards.overview.timeline.legend.deferred")}</span>
      <span className="flex items-center gap-1.5"><span className="h-3 w-5 rounded-sm bg-primary" />{t("dashboards.overview.timeline.legend.task")}</span>
    </div>
    {data.truncated && <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.timeline.truncated")}</p>}
    {data.spaces.length === 0 && <p className="text-sm text-muted-foreground">{t("dashboards.overview.timeline.empty")}</p>}
    {data.spaces.length > 0 && <div role="region" tabIndex={0} aria-label={t("dashboards.overview.timeline.region")}
      className="overflow-x-auto rounded-xl border bg-card outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
      <div style={{ minWidth: LABEL_PX + TIMELINE_DAYS * DAY_PX }}>
        <Header ctx={ctx} />
        {data.spaces.map((space) => <SpaceRows key={space.id} space={space} ctx={ctx} />)}
      </div>
    </div>}
  </div>;
}

type Ctx = { from: string; dates: string[]; profile?: CurrentUser };

function Header({ ctx }: { ctx: Ctx }) {
  const { t } = useTranslation();
  const today = todayKey();
  const months: { label: string; span: number }[] = [];
  for (const date of ctx.dates) {
    const label = new Intl.DateTimeFormat(ctx.profile?.locale ?? "en", { month: "short", year: "numeric", timeZone: "UTC" }).format(new Date(utc(date)));
    const last = months[months.length - 1];
    if (last?.label === label) last.span++; else months.push({ label, span: 1 });
  }
  return <div className="border-b text-xs text-muted-foreground">
    <div className="flex">
      <div className="sticky left-0 z-10 shrink-0 border-r bg-card" style={{ width: LABEL_PX }} />
      {months.map((m, i) => <div key={i} className="shrink-0 border-l px-2 py-1 font-medium text-foreground" style={{ width: m.span * DAY_PX }}>{m.label}</div>)}
    </div>
    <div className="flex">
      <div className="sticky left-0 z-10 flex shrink-0 items-center border-r bg-card px-4 py-1.5 font-medium text-foreground" style={{ width: LABEL_PX }}>
        {t("dashboards.overview.timeline.item")}</div>
      {ctx.dates.map((date) => <div key={date} style={{ width: DAY_PX }}
        className={cn("shrink-0 border-l py-1.5 text-center tabular-nums", isWeekend(date) && "bg-muted/40", date === today && "bg-primary/10 font-semibold text-foreground")}>
        {dayNumber(date)}</div>)}
    </div>
  </div>;
}

/** One line of the grid: a sticky label on the left, the day track on the right. */
function Row({ label, tone, children }: { label: ReactNode; tone?: string; children?: ReactNode }) {
  return <div className={cn("flex border-b last:border-b-0", tone)}>
    <div className="sticky left-0 z-10 flex shrink-0 items-center gap-2 border-r bg-card px-4 py-1.5" style={{ width: LABEL_PX }}>{label}</div>
    <div className="relative shrink-0" style={{
      width: TIMELINE_DAYS * DAY_PX, backgroundImage: "linear-gradient(to right, var(--border) 1px, transparent 1px)", backgroundSize: `${DAY_PX}px 100%`,
    }}>{children}</div>
  </div>;
}

function Marker({ ctx, date, className, style, title }: { ctx: Ctx; date: string; className: string; style: (left: number) => CSSProperties; title: string }) {
  const index = Math.round((utc(date) - utc(ctx.from)) / DAY_MS);
  if (index < 0 || index >= TIMELINE_DAYS) return null;
  return <div title={title} className={cn("absolute", className)} style={style(index * DAY_PX)} />;
}

/** A Space is collapsed by default; its Projects and its unassigned Tasks sit under one header. */
function SpaceRows({ space, ctx }: { space: TimelineSpace; ctx: Ctx }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [unassignedOpen, setUnassignedOpen] = useState(false);
  const Chevron = open ? ChevronDown : ChevronRight;
  const UnassignedChevron = unassignedOpen ? ChevronDown : ChevronRight;
  return <section aria-label={space.name}>
    <Row tone="bg-muted/50" label={<>
      <button type="button" aria-expanded={open} onClick={() => setOpen(!open)}
        className="flex min-w-0 flex-1 items-center gap-1.5 rounded-sm text-left font-semibold outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
        <Chevron className="size-4 shrink-0" aria-hidden />
        <span className="min-w-0 truncate">{space.name}</span>
      </button>
      <span className="shrink-0 text-xs text-muted-foreground">{t(`dashboards.overview.access.${space.effective_access}`)}</span>
      <Link to="/spaces/$spaceId" params={{ spaceId: space.id }} className="shrink-0 text-xs underline">{t("dashboards.overview.timeline.openSpace")}</Link></>} />
    {open && <>
      {space.projects.map((project) => <ProjectRows key={project.id} project={project} ctx={ctx} />)}
      {space.unassigned_tasks.length > 0 && <>
        <Row label={<button type="button" aria-expanded={unassignedOpen} onClick={() => setUnassignedOpen(!unassignedOpen)}
          className="flex min-w-0 flex-1 items-center gap-1.5 rounded-sm text-left text-xs font-medium text-muted-foreground outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
          <UnassignedChevron className="size-4 shrink-0" aria-hidden />
          <span className="min-w-0 truncate">{t("dashboards.overview.timeline.unassigned")}</span>
          <span className="ml-auto shrink-0 tabular-nums">{space.unassigned_tasks.length}</span></button>} />
        {unassignedOpen && space.unassigned_tasks.map((task) => <TaskRow key={task.id} task={task} ctx={ctx} indent />)}
      </>}
    </>}
  </section>;
}

function ProjectRows({ project, ctx }: { project: TimelineProject; ctx: Ctx }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const Chevron = open ? ChevronDown : ChevronRight;
  return <>
    <Row label={<button type="button" aria-expanded={open} onClick={() => setOpen(!open)}
      className="flex min-w-0 flex-1 items-center gap-1.5 rounded-sm text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
      <Chevron className="size-4 shrink-0" aria-hidden />
      <span className="min-w-0 truncate font-medium">{project.title}</span>
      <span className="ml-auto shrink-0 text-xs text-muted-foreground">{formatDate(project.due_date, ctx.profile)}</span>
      <span className="sr-only">{t(`dashboards.overview.status.${project.status_group}`)}</span></button>}>
      <Marker ctx={ctx} date={project.due_date} className={cn("top-1/2 h-4 -translate-y-1/2 rounded-md", project.status_group === "deferred" ? "bg-muted-foreground/50" : "bg-sky-500")} title={t("dashboards.overview.timeline.milestone", { date: formatDate(project.due_date, ctx.profile) })}
        style={(left) => ({ left: left + 4, width: DAY_PX - 8 })} />
    </Row>
    {open && (project.tasks.length === 0
      ? <Row label={<span className="pl-6 text-xs text-muted-foreground">{t("dashboards.overview.timeline.noTasks")}</span>} />
      : project.tasks.map((task) => <TaskRow key={task.id} task={task} ctx={ctx} indent />))}
  </>;
}

function TaskRow({ task, ctx, indent }: { task: TimelineTask; ctx: Ctx; indent?: boolean }) {
  return <Row label={<>
    <span className={cn("min-w-0 truncate text-sm", indent && "pl-6")}>{task.title}</span>
    <span className="ml-auto shrink-0 text-xs text-muted-foreground">{formatDate(task.due_date, ctx.profile)}</span></>}>
    <Marker ctx={ctx} date={task.due_date} className="top-1/2 h-3 -translate-y-1/2 rounded-sm bg-primary"
      title={task.title} style={(left) => ({ left: left + 4, width: DAY_PX - 8 })} />
  </Row>;
}
