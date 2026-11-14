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
    <h1 className="workspace-title">{t("workload.title")}</h1>
    <div className="flex flex-wrap items-end gap-3">
      <label>{t("workload.from")}<Input type="date" value={from} onChange={e => setFrom(e.target.value)} /></label>
      <label>{t("workload.to")}<Input type="date" value={to} onChange={e => setTo(e.target.value)} /></label>
      <label>{t("workload.role")}<select className="block rounded border bg-background p-2" value={role} onChange={e => setRole(e.target.value)}><option value="">{t("workload.all")}</option>{directory.data?.roles.map(x => <option key={x.id} value={x.id}>{x.name}</option>)}</select></label>
      <label>{t("workload.skill")}<select className="block rounded border bg-background p-2" value={skill} onChange={e => setSkill(e.target.value)}><option value="">{t("workload.all")}</option>{directory.data?.skills.map(x => <option key={x.id} value={x.id}>{x.name}</option>)}</select></label>
      <label className="flex gap-2"><input type="checkbox" checked={weekly} onChange={e => setWeekly(e.target.checked)} /> {t("workload.weekly")}</label>
    </div>
    {data.isPending && <p role="status">{t("workload.loading")}</p>}
    {(data.error || save.error) && <p role="alert">{(save.error ?? data.error)?.message}</p>}
    {data.data && <>
      <p className="text-sm text-muted-foreground">{t("workload.hint")}</p>
      <div className="relative overflow-x-auto">
        <table className="w-full text-left text-sm">
          <caption className="sr-only">{t("workload.caption")}</caption>
          <thead><tr><th scope="col" className="p-2">{t("workload.resource")}</th>{columns.map(i => <th scope="col" key={i} className="whitespace-nowrap p-2">{iso(dayAt(from, i))}</th>)}</tr></thead>
          <tbody>{[...data.data.people, ...(data.data.role_demand ?? [])].map(p => <tr key={p.id} className="border-t"><th scope="row" className="p-2">{p.name}</th>
            {columns.map(i => {
              const range = (values: number[]) => values.slice(i, i + (weekly ? 7 : 1));
              const sum = (values: number[]) => range(values).reduce((total, n) => total + n, 0);
              const over = range(p.capacity).some((cap, k) => p.planned[i+k] + p.booked[i+k] > cap);
              return <td key={i} className={over ? "bg-destructive/10 p-2 font-medium text-foreground" : "p-2"}>
                {sum(p.planned)} + {sum(p.booked)} / {sum(p.capacity)}{over && <span className="block">{t("workload.over")}</span>}
              </td>;
            })}
          </tr>)}</tbody>
        </table>
      </div>
      {util.data && <section className="space-y-2"><h2 className="font-semibold">{t("workload.utilization")}</h2>
        <div className="overflow-x-auto"><table className="w-full text-left text-sm">
          <caption className="sr-only">{t("workload.utilCaption")}</caption>
          <thead><tr><th scope="col" className="p-2">{t("workload.resource")}</th><th scope="col" className="p-2">{t("workload.capacity")}</th><th scope="col" className="p-2">{t("workload.planned")}</th><th scope="col" className="p-2">{t("workload.actual")}</th></tr></thead>
          <tbody>{util.data.items.map(u => <tr key={u.id} className="border-t"><th scope="row" className="p-2">{u.name}</th><td className="p-2">{u.capacity}</td>
            <td className="p-2">{u.planned} ({u.planned_percent}%)</td><td className="p-2">{u.actual} ({u.actual_percent}%)</td></tr>)}</tbody>
        </table></div></section>}
      <TaskList title={t("workload.backlog")} tasks={data.data.tasks.filter(x => !x.assignee_ids.length)} onSave={(task, ids) => save.mutate({ path: `/api/v1/items/${task.id}`, method: "PATCH", version: task.version, body: { assignee_ids: ids } })} busy={save.isPending} />
      <TaskList title={t("workload.assigned")} tasks={data.data.tasks.filter(x => x.assignee_ids.length > 0)} onSave={(task, ids) => save.mutate({ path: `/api/v1/items/${task.id}`, method: "PATCH", version: task.version, body: { assignee_ids: ids } })} busy={save.isPending} />
      {profile?.user_type === "admin" && <section className="space-y-3"><h2 className="font-semibold">{t("workload.bookings")}</h2>
        <form className="flex flex-wrap items-end gap-3" onSubmit={event => {
          event.preventDefault();
          const fields = new FormData(event.currentTarget);
          const resource = String(fields.get("resource"));
          save.mutate({ path: "/api/v1/resource-bookings", method: "POST", body: {
            project_id: project, start_date: from, end_date: to, minutes_per_day: Number(fields.get("minutes")),
            ...(resource.startsWith("role:") ? { role_id: resource.slice(5) } : { user_id: resource }),
          } });
        }}>
          <label>{t("workload.findProject")}<Input type="search" value={projectQuery} onChange={e => { setProject(""); setProjectQuery(e.target.value); }} /></label>
          <label>{t("workload.project")}<select value={project} required className="block rounded border bg-background p-2" onChange={e => setProject(e.target.value)}>
            <option value="">{t("workload.select")}</option>{projects.data?.items.filter(x => x.type === "project").map(x => <option key={x.id} value={x.id}>{x.title}</option>)}
          </select></label>
          <label>{t("workload.resource")}<select name="resource" required className="block rounded border bg-background p-2"><option value="">{t("workload.select")}</option>
            {data.data.people.map(x => <option key={x.id} value={x.id}>{x.name}</option>)}
            {directory.data?.roles.map(x => <option key={x.id} value={`role:${x.id}`}>{t("workload.roleSuffix", { name: x.name })}</option>)}</select></label>
          <label>{t("workload.minutes")}<Input type="number" name="minutes" min={1} max={1440} required /></label><Button type="submit" disabled={save.isPending}>{t("workload.add")}</Button>
        </form>
        <ul>{data.data.bookings.map(x => <li key={x.id} className="flex flex-wrap items-center gap-2 py-1">
          <span>{t("workload.perDay", { name: data.data.people.find(p => p.id === x.user_id)?.name ?? directory.data?.roles.find(p => p.id === x.role_id)?.name, start: x.start_date, end: x.end_date, min: x.minutes_per_day })}</span>
          <Button size="sm" variant="outline" disabled={save.isPending} onClick={() => save.mutate({path:`/api/v1/resource-bookings/${x.id}`,method:"DELETE"})}>{t("workload.remove")}</Button>
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
    {tasks.length === 0 ? <p>{t("workload.noTasks")}</p> : <ul className="divide-y">{tasks.map(task => <li key={task.id} className="space-y-2 py-2">
      <span>{task.title}</span>
      {task.editable && <Button className="ml-2" size="sm" variant="outline" onClick={() => { setEditing(task.id); setPeople(task.assignee_ids.map((id, index) => ({ id, name: task.assignee_names[index] ?? id }))); }}>{t("workload.reassign")}</Button>}
      {editing === task.id && <div className="space-y-2">
        <p className="text-sm">{t("workload.pick")}</p>
        <ItemUserPicker multiple maxSelected={20} label={t("workload.replacement")} selected={people} onChange={setPeople} />
        <Button size="sm" disabled={busy} onClick={() => { onSave(task, people.map(p => p.id)); setEditing(undefined); }}>{t("workload.save")}</Button>
        <Button size="sm" variant="ghost" onClick={() => setEditing(undefined)}>{t("workload.cancel")}</Button>
      </div>}
    </li>)}</ul>}
  </section>;
}
