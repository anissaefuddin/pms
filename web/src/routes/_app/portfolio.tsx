import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { api } from "@/lib/api";
import type { Item } from "@/lib/items";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/portfolio")({ component: Portfolio });

type Project = { id: string; title: string };
type Dependency = { id: string; predecessor_id: string; successor_id: string; type: string; lag: number };
type Timeline = { projects: Project[]; items: { project_id: string; item: Item }[]; dependencies: Dependency[] };

const day = 86_400_000;
const atDay = (value?: string | null) => value ? Date.parse(`${value}T00:00:00Z`) : undefined;

function Portfolio() {
  const initial = new URLSearchParams(window.location.search).get("projects") ?? "";
  const [draft, setDraft] = useState(initial);
  const [projectIDs, setProjectIDs] = useState(() => initial.split(",").map(x => x.trim()).filter(Boolean));
  const timeline = useQuery({
    queryKey: ["portfolio", projectIDs], enabled: projectIDs.length > 0, retry: false,
    queryFn: () => api<Timeline>("/api/v1/portfolio/timeline", { method: "POST", body: JSON.stringify({ project_ids: projectIDs }) }),
  });
  const range = useMemo(() => {
    const dates = (timeline.data?.items ?? []).flatMap(({ item }) => [atDay(item.start_date), atDay(item.due_date)]).filter((x): x is number => x !== undefined);
    return dates.length ? { first: Math.min(...dates), last: Math.max(...dates) } : undefined;
  }, [timeline.data]);
  const projectByID = new Map(timeline.data?.projects.map(x => [x.id, x.title]));
  const itemByID = new Map(timeline.data?.items.map(x => [x.item.id, x.item.title]));

  return <main className="min-w-0 space-y-5 p-4 sm:p-8">
    <header><h1 className="workspace-title">Portfolio Gantt</h1><p className="text-sm text-muted-foreground">Compare visible project schedules and their cross-project dependencies.</p></header>
    <form className="flex max-w-3xl items-end gap-2" onSubmit={event => {
      event.preventDefault();
      const ids = draft.split(",").map(x => x.trim()).filter(Boolean);
      setProjectIDs(ids);
      window.history.replaceState(null, "", ids.length ? `/portfolio?projects=${encodeURIComponent(ids.join(","))}` : "/portfolio");
    }}>
      <label className="grow text-sm">Project IDs, separated by commas<Input value={draft} onChange={event => setDraft(event.target.value)} placeholder="UUID, UUID" /></label>
      <Button type="submit">Show timeline</Button>
    </form>
    {timeline.isPending && projectIDs.length > 0 && <p role="status">Loading portfolio…</p>}
    {timeline.error && <p role="alert">{timeline.error.message}</p>}
    {timeline.data && <>
      <section aria-label="Portfolio timeline" className="overflow-x-auto rounded-lg border">
        <div className="min-w-[720px] divide-y">
          {timeline.data.items.map(({ project_id: projectID, item }) => {
            const start = atDay(item.start_date) ?? atDay(item.due_date);
            const due = atDay(item.due_date) ?? start;
            const span = range ? Math.max(1, (range.last - range.first) / day + 1) : 1;
            const left = start && range ? ((start - range.first) / day / span) * 100 : 0;
            const width = start && due ? Math.max(1.5, ((due - start) / day + 1) / span * 100) : 0;
            return <div key={item.id} className="grid grid-cols-[16rem_1fr] items-center gap-3 p-2 text-sm">
              <div className="truncate"><span className="text-muted-foreground">{projectByID.get(projectID)} · </span>{item.title}</div>
              <div role="img" className="relative h-6 rounded bg-muted" aria-label={`${item.title}: ${item.start_date ?? "unscheduled"} to ${item.due_date ?? "unscheduled"}`}>
                {width > 0 && <div className="absolute top-1 h-4 rounded bg-primary" style={{ left: `${left}%`, width: `${width}%` }} />}
              </div>
            </div>;
          })}
        </div>
      </section>
      <section><h2 className="font-semibold">Cross-project dependencies</h2>
        {timeline.data.dependencies.length === 0 ? <p className="text-sm text-muted-foreground">No dependency connects the selected projects.</p> :
          <ul className="divide-y">{timeline.data.dependencies.map(dep => <li className="py-2 text-sm" key={dep.id}>
            {itemByID.get(dep.predecessor_id)} → {itemByID.get(dep.successor_id)} ({dep.type}{dep.lag ? `, ${dep.lag}d` : ""})
          </li>)}</ul>}
      </section>
    </>}
  </main>;
}
