import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { formatDate, formatNumber, useProfile } from "@/lib/profile";
import { useOverviewSummary, useOverviewTasks, type TasksFilter, type TasksPreset } from "@/lib/overview";
import { ItemUserPicker, type PersonOption } from "@/components/item-user-picker";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

const STATUSES = ["active", "deferred", "completed", "cancelled"] as const;
const PRESETS: { value: string; preset: TasksPreset }[] = [
  { value: "all", preset: "" },
  { value: "overdue", preset: "overdue" },
  { value: "completed_week", preset: "completed_week" },
];

/** Where the Tasks tab starts: a Summary figure (overdue or completed this week), optionally for one Space. */
export type TasksSeed = { preset?: TasksPreset; spaceId?: string };

/** Tasks across every Space the user can see. Each filter combines with the others; empty means any. */
export function WorkspaceTasks({ seed }: { seed: TasksSeed }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const spaces = useOverviewSummary().data?.spaces ?? [];
  const [preset, setPreset] = useState<TasksPreset>(seed.preset ?? "");
  const [spaceIds, setSpaceIds] = useState<string[]>(seed.spaceId ? [seed.spaceId] : []);
  const [assignees, setAssignees] = useState<PersonOption[]>([]);
  const [statuses, setStatuses] = useState<string[]>([]);
  const [dueFrom, setDueFrom] = useState("");
  const [dueTo, setDueTo] = useState("");
  const [q, setQ] = useState("");
  const filter: TasksFilter = { preset, spaces: spaceIds, assignees: assignees.map((person) => person.id), statuses, dueFrom, dueTo, q };
  const tasks = useOverviewTasks(filter);
  const toggle = (list: string[], value: string) => (list.includes(value) ? list.filter((x) => x !== value) : [...list, value]);
  const clear = () => { setPreset(""); setSpaceIds([]); setAssignees([]); setStatuses([]); setDueFrom(""); setDueTo(""); setQ(""); };
  const active = preset !== "" || spaceIds.length > 0 || assignees.length > 0 || statuses.length > 0 || dueFrom !== "" || dueTo !== "" || q.trim() !== "";
  const n = (value: number) => formatNumber(value, profile);

  return <div className="space-y-4">
    <p className="text-sm text-muted-foreground">{t("dashboards.overview.tasks.hint")}</p>
    <div className="space-y-4 rounded-xl border bg-card p-4">
      <div className="flex flex-wrap items-end gap-4">
        <div className="space-y-1">
          <span className="text-xs font-medium text-muted-foreground">{t("dashboards.overview.tasks.preset")}</span>
          <SegmentedControl aria-label={t("dashboards.overview.tasks.preset")} value={PRESETS.find((p) => p.preset === preset)?.value ?? "all"}
            onChange={(value) => setPreset(PRESETS.find((p) => p.value === value)?.preset ?? "")}
            items={PRESETS.map((p) => ({ value: p.value, label: t(`dashboards.overview.tasks.presets.${p.value}`) }))} />
        </div>
        <label className="space-y-1 text-sm">
          <span className="block text-xs font-medium text-muted-foreground">{t("dashboards.overview.tasks.dueFrom")}</span>
          <input type="date" value={dueFrom} onChange={(event) => setDueFrom(event.target.value)} className="block h-9 rounded-md border border-input bg-background px-2" />
        </label>
        <label className="space-y-1 text-sm">
          <span className="block text-xs font-medium text-muted-foreground">{t("dashboards.overview.tasks.dueTo")}</span>
          <input type="date" value={dueTo} onChange={(event) => setDueTo(event.target.value)} className="block h-9 rounded-md border border-input bg-background px-2" />
        </label>
        <label className="min-w-48 flex-1 space-y-1 text-sm">
          <span className="block text-xs font-medium text-muted-foreground">{t("dashboards.overview.tasks.search")}</span>
          <Input type="search" value={q} onChange={(event) => setQ(event.target.value)} placeholder={t("dashboards.overview.tasks.searchPlaceholder")} />
        </label>
        {active && <Button variant="outline" size="sm" onClick={clear}>{t("dashboards.overview.tasks.clear")}</Button>}
      </div>
      <div className="flex flex-wrap items-start gap-4">
        <details className="relative text-sm">
          <summary className="flex h-9 cursor-pointer items-center rounded-md border border-input bg-background px-3">
            {t("dashboards.overview.tasks.spaces")}{spaceIds.length > 0 && <span className="ml-2 tabular-nums text-muted-foreground">{spaceIds.length}</span>}
          </summary>
          <div className="absolute z-20 mt-1 max-h-72 w-64 space-y-1 overflow-y-auto rounded-md border bg-popover p-2 shadow-md">
            {spaces.length === 0 && <p className="px-1 text-xs text-muted-foreground">{t("dashboards.overview.tasks.noSpaces")}</p>}
            {spaces.map((space) => <label key={space.id} className="flex items-center gap-2 rounded px-1 py-1 hover:bg-accent">
              <input type="checkbox" checked={spaceIds.includes(space.id)} onChange={() => setSpaceIds((list) => toggle(list, space.id))} />
              <span className="min-w-0 truncate">{space.name}</span>
            </label>)}
          </div>
        </details>
        <div className="min-w-64 flex-1 text-sm">
          <ItemUserPicker label={t("dashboards.overview.tasks.assignee")} selected={assignees} onChange={setAssignees} multiple />
        </div>
        <fieldset className="flex flex-wrap items-center gap-3 text-sm">
          <legend className="sr-only">{t("dashboards.overview.tasks.statuses")}</legend>
          {STATUSES.map((status) => <label key={status} className="flex items-center gap-1.5">
            <input type="checkbox" checked={statuses.includes(status)} onChange={() => setStatuses((list) => toggle(list, status))} />
            {t(`dashboards.overview.status.${status}`)}
          </label>)}
        </fieldset>
      </div>
    </div>

    {tasks.isPending && <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.loading")}</p>}
    {tasks.isError && <div><p role="alert">{t("dashboards.overview.error")}</p>
      <Button variant="outline" className="mt-3" onClick={() => tasks.refetch()}>{t("dashboards.overview.retry")}</Button></div>}
    {tasks.data && <>
      <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
        <span className="tabular-nums text-muted-foreground" role="status">{t("dashboards.overview.tasks.count", { count: tasks.data.items.length })}</span>
        {tasks.data.truncated && <span className="text-muted-foreground">{t("dashboards.overview.tasks.truncated")}</span>}
      </div>
      {tasks.data.items.length === 0
        ? <p className="text-sm text-muted-foreground">{t("dashboards.overview.tasks.empty")}</p>
        : <div className="overflow-x-auto rounded-xl border bg-card">
          <table className="w-full min-w-[720px] text-left text-sm">
            <thead className="border-b text-xs text-muted-foreground">
              <tr>
                <th scope="col" className="px-4 py-2 font-medium">{t("dashboards.overview.tasks.columns.task")}</th>
                <th scope="col" className="px-4 py-2 font-medium">{t("dashboards.overview.tasks.columns.space")}</th>
                <th scope="col" className="px-4 py-2 font-medium">{t("dashboards.overview.tasks.columns.assignees")}</th>
                <th scope="col" className="px-4 py-2 font-medium">{t("dashboards.overview.tasks.columns.status")}</th>
                <th scope="col" className="px-4 py-2 font-medium">{t("dashboards.overview.tasks.columns.start")}</th>
                <th scope="col" className="px-4 py-2 font-medium">{t("dashboards.overview.tasks.columns.due")}</th>
              </tr>
            </thead>
            <tbody>
              {tasks.data.items.map((task) => <tr key={task.id} className="border-b last:border-b-0 hover:bg-muted/40">
                <th scope="row" className="px-4 py-2.5 text-left font-medium">
                  <Link to="/tasks/$itemId" params={{ itemId: task.id }} className="underline">{task.title}</Link>
                </th>
                <td className="px-4 py-2.5"><Link to="/spaces/$spaceId" params={{ spaceId: task.space_id }} className="underline-offset-2 hover:underline">{task.space_name}</Link></td>
                <td className="px-4 py-2.5 text-muted-foreground">{task.assignees.map((person) => person.name).join(", ") || t("dashboards.overview.tasks.unassigned")}</td>
                <td className="px-4 py-2.5">{task.status_name || t(`dashboards.overview.status.${task.status_group}`)}</td>
                <td className="px-4 py-2.5 tabular-nums text-muted-foreground">{task.start_date ? formatDate(task.start_date, profile) : "—"}</td>
                <td className={cn("px-4 py-2.5 tabular-nums", task.overdue ? "font-semibold text-destructive" : "text-muted-foreground")}>
                  {task.due_date ? formatDate(task.due_date, profile) : "—"}
                  {task.overdue && <span className="ml-2 text-xs font-normal">{t("dashboards.overview.tasks.overdue")}</span>}
                </td>
              </tr>)}
            </tbody>
          </table>
        </div>}
    </>}
  </div>;
}
