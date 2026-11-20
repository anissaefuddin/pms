import { useTranslation } from "react-i18next";
import { createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ItemUserPicker } from "@/components/item-user-picker";
import { useSearchResults } from "@/lib/search";
import { useProfile } from "@/lib/profile";
import { TopbarTitle } from "@/components/topbar-title";

export const Route = createFileRoute("/_app/workload")({ component: Workload });
type Choice = { id: string; name: string };
type Person = Choice & { capacity: number[]; planned: number[]; booked: number[] };
type Task = { id: string; title: string; version: number; effort: number; assignee_ids: string[]; assignee_names: string[]; editable: boolean };
type Booking = { id: string; user_id: string | null; role_id: string | null; start_date: string; end_date: string; minutes_per_day: number };
const iso = (date: Date) => date.toISOString().slice(0, 10);
const dayAt = (from: string, offset: number) => new Date(Date.parse(from) + offset * 86400000);

function Workload() {
  const { t } = useTranslation();
  const profile = useProfile();
  const cache = useQueryClient();
  const today = iso(new Date());
  const [from, setFrom] = useState(today);
  const [to, setTo] = useState(iso(dayAt(today, 13)));
  const [projectQuery, setProjectQuery] = useState("");
  const [project, setProject] = useState("");
  const projects = useSearchResults(projectQuery.trim(), "everywhere", {}, 20, profile?.user_type === "admin");
  const [weekly, setWeekly] = useState(false);
  const [role, setRole] = useState("");
  const [skill, setSkill] = useState("");
  const params = new URLSearchParams({ from, to, role, skill });
  const data = useQuery({ queryKey: ["workload", from, to, role, skill], enabled: !!from && !!to,
    queryFn: () => api<{ people: Person[]; tasks: Task[]; bookings: Booking[]; role_demand: Person[] }>(`/api/v1/workload?${params}`) });
  const util = useQuery({ queryKey: ["utilization", from, to, role, skill], enabled: !!from && !!to,
    queryFn: () => api<{ items: { id: string; name: string; capacity: number; planned: number; actual: number; planned_percent: number; actual_percent: number }[] }>(`/api/v1/utilization?${params}`) });
  const directory = useQuery({ queryKey: ["resource-directory"], queryFn: () => api<{ roles: Choice[]; skills: Choice[] }>("/api/v1/resource-directory") });
  const save = useMutation({ mutationFn: ({ path, method, body, version }: { path: string; method: string; body?: object; version?: number }) =>
    api(path, { method, body: body && JSON.stringify(body), headers: version ? { "If-Match": `"${version}"` } : undefined }),
    onSuccess: () => cache.invalidateQueries({ queryKey: ["workload"] }) });
  const days = from && to ? Math.max(0, Math.min(93, Math.round((Date.parse(to) - Date.parse(from)) / 86400000) + 1)) : 0;
  const columns = Array.from({ length: days }, (_, i) => i).filter((i) => !weekly || i % 7 === 0);

  return <main className="min-w-0 space-y-5 p-4 sm:p-8">
    <TopbarTitle>{t("settings.workload.workload")}</TopbarTitle>
    <div className="flex flex-wrap items-end gap-3">
      <label>{t("settings.workload.from")}<Input type="date" value={from} onChange={e => setFrom(e.target.value)} /></label>
      <label>{t("settings.workload.to")}<Input type="date" value={to} onChange={e => setTo(e.target.value)} /></label>
      <label className="min-w-0 max-w-full">{t("settings.workload.jobRole")}<select className="block max-w-full rounded border bg-background p-2" value={role} onChange={e => setRole(e.target.value)}><option value="">{t("settings.workload.all")}</option>{directory.data?.roles.map(x => <option key={x.id} value={x.id}>{x.name}</option>)}</select></label>
      <label className="min-w-0 max-w-full">{t("settings.workload.skill")}<select className="block max-w-full rounded border bg-background p-2" value={skill} onChange={e => setSkill(e.target.value)}><option value="">{t("settings.workload.all")}</option>{directory.data?.skills.map(x => <option key={x.id} value={x.id}>{x.name}</option>)}</select></label>
      <label className="flex gap-2"><input type="checkbox" checked={weekly} onChange={e => setWeekly(e.target.checked)} /> {t("settings.workload.groupByWeek")}</label>
    </div>
    {data.isPending && <p role="status">{t("settings.workload.loadingWorkload")}</p>}
    {(data.error || save.error) && <p role="alert">{(save.error ?? data.error)?.message}</p>}
    {data.data && <>
      <p className="text-sm text-muted-foreground">{t("settings.workload.minutesPlannedBookedCapacityWeeklyColumnsGroupSevenDaysFromTheSelectedStart")}</p>
      <div role="region" aria-label={t("settings.workload.dailyWorkloadColumns")} tabIndex={0} className="relative overflow-x-auto rounded focus-visible:outline-2 focus-visible:outline-ring">
        <table aria-label={t("settings.workload.dailyWorkload")} className="w-full text-left text-sm">
          <caption className="sr-only">{t("settings.workload.workloadAndCapacity")}</caption>
          <thead><tr><th scope="col" className="p-2">{t("settings.workload.resource")}</th>{columns.map(i => <th scope="col" key={i} className="whitespace-nowrap p-2">{iso(dayAt(from, i))}</th>)}</tr></thead>
          <tbody>{[...data.data.people, ...(data.data.role_demand ?? [])].map(p => <tr key={p.id} className="border-t"><th scope="row" className="p-2">{p.name}</th>
            {columns.map(i => {
              const range = (values: number[]) => values.slice(i, i + (weekly ? 7 : 1));
              const sum = (values: number[]) => range(values).reduce((total, n) => total + n, 0);
              const over = range(p.capacity).some((cap, k) => p.planned[i+k] + p.booked[i+k] > cap);
              return <td key={i} className={over ? "bg-destructive/10 p-2 font-medium text-foreground" : "p-2"}>
                {sum(p.planned)} + {sum(p.booked)} / {sum(p.capacity)}{over && <span className="block">{t("settings.workload.overCapacity")}</span>}
              </td>;
            })}
          </tr>)}</tbody>
        </table>
      </div>
      {util.data && <section className="space-y-2"><h2 id="workload-utilization" className="font-semibold">{t("settings.workload.utilizationPlannedVsActual")}</h2>
        <div role="region" aria-labelledby="workload-utilization" tabIndex={0} className="overflow-x-auto rounded focus-visible:outline-2 focus-visible:outline-ring"><table aria-labelledby="workload-utilization" className="w-full text-left text-sm">
          <caption className="sr-only">{t("settings.workload.plannedAndLoggedMinutesAgainstCapacity")}</caption>
          <thead><tr><th scope="col" className="p-2">{t("settings.workload.resource")}</th><th scope="col" className="p-2">{t("settings.workload.capacityMin")}</th><th scope="col" className="p-2">{t("settings.workload.planned")}</th><th scope="col" className="p-2">{t("settings.workload.actual")}</th></tr></thead>
          <tbody>{util.data.items.map(u => <tr key={u.id} className="border-t"><th scope="row" className="p-2">{u.name}</th><td className="p-2">{u.capacity}</td>
            <td className="p-2">{u.planned} ({u.planned_percent}%)</td><td className="p-2">{u.actual} ({u.actual_percent}%)</td></tr>)}</tbody>
        </table></div></section>}
      <TaskList title={t("settings.workload.unassignedBacklog")} tasks={data.data.tasks.filter(t => !t.assignee_ids.length)} onSave={(task, ids) => save.mutate({ path: `/api/v1/items/${task.id}`, method: "PATCH", version: task.version, body: { assignee_ids: ids } })} busy={save.isPending} />
      <TaskList title={t("settings.workload.assignedTasks")} tasks={data.data.tasks.filter(t => t.assignee_ids.length > 0)} onSave={(task, ids) => save.mutate({ path: `/api/v1/items/${task.id}`, method: "PATCH", version: task.version, body: { assignee_ids: ids } })} busy={save.isPending} />
      {profile?.user_type === "admin" && <section className="space-y-3"><h2 className="font-semibold">{t("settings.workload.bookings")}</h2>
        <form className="flex flex-wrap items-end gap-3" onSubmit={event => {
          event.preventDefault();
          const fields = new FormData(event.currentTarget);
          const resource = String(fields.get("resource"));
          save.mutate({ path: "/api/v1/resource-bookings", method: "POST", body: {
            project_id: project, start_date: from, end_date: to, minutes_per_day: Number(fields.get("minutes")),
            ...(resource.startsWith("role:") ? { role_id: resource.slice(5) } : { user_id: resource }),
          } });
        }}>
          <label>{t("settings.workload.findAProject")}<Input type="search" value={projectQuery} onChange={e => { setProject(""); setProjectQuery(e.target.value); }} /></label>
          <label className="min-w-0 max-w-full">{t("settings.workload.project")}<select value={project} required className="block max-w-full rounded border bg-background p-2" onChange={e => setProject(e.target.value)}>
            <option value="">{t("settings.workload.select")}</option>{projects.data?.items.filter(x => x.type === "project").map(x => <option key={x.id} value={x.id}>{x.title}</option>)}
          </select></label>
          <label className="min-w-0 max-w-full">{t("settings.workload.resource")}<select name="resource" required className="block max-w-full rounded border bg-background p-2"><option value="">{t("settings.workload.select")}</option>
            {data.data.people.map(x => <option key={x.id} value={x.id}>{x.name}</option>)}
            {directory.data?.roles.map(x => <option key={x.id} value={`role:${x.id}`}>{t("settings.workload.roleOption", { name: x.name })}</option>)}</select></label>
          <label>{t("settings.workload.minutesPerDay")}<Input type="number" name="minutes" min={1} max={1440} required /></label><Button type="submit" disabled={save.isPending}>{t("settings.workload.addBooking")}</Button>
        </form>
        <ul>{data.data.bookings.map(x => <li key={x.id} className="flex flex-wrap items-center gap-2 py-1">
          <span>{t("settings.workload.bookingSummary", { name: data.data.people.find(p => p.id === x.user_id)?.name ?? directory.data?.roles.find(p => p.id === x.role_id)?.name ?? "", start: x.start_date, end: x.end_date, minutes: x.minutes_per_day })}</span>
          <Button size="sm" variant="outline" disabled={save.isPending} onClick={() => save.mutate({path:`/api/v1/resource-bookings/${x.id}`,method:"DELETE"})}>{t("settings.workload.removeBooking")}</Button>
        </li>)}</ul>
      </section>}
    </>}
  </main>;
}

function TaskList({ title, tasks, onSave, busy }: { title: string; tasks: Task[]; onSave: (task: Task, ids: string[]) => void; busy: boolean }) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState<string>();
  const [people, setPeople] = useState<Choice[]>([]);
  return <section><h2 className="font-semibold">{title}</h2>
    {tasks.length === 0 ? <p>{t("settings.workload.noTasksInThisPeriod")}</p> : <ul className="divide-y">{tasks.map(task => <li key={task.id} className="space-y-2 py-2">
      <span>{task.title}</span>
      {task.editable && <Button className="ml-2" size="sm" variant="outline" onClick={() => { setEditing(task.id); setPeople(task.assignee_ids.map((id, index) => ({ id, name: task.assignee_names[index] ?? id }))); }}>{t("settings.workload.reassign")}</Button>}
      {editing === task.id && <div className="space-y-2">
        <p className="text-sm">{t("settings.workload.selectTheCompleteReplacementAssigneeList")}</p>
        <ItemUserPicker multiple maxSelected={20} label={t("settings.workload.replacementAssignees")} selected={people} onChange={setPeople} />
        <Button size="sm" disabled={busy} onClick={() => { onSave(task, people.map(p => p.id)); setEditing(undefined); }}>{t("settings.workload.saveAssignment")}</Button>
        <Button size="sm" variant="ghost" onClick={() => setEditing(undefined)}>{t("settings.workload.cancel")}</Button>
      </div>}
    </li>)}</ul>}
  </section>;
}
