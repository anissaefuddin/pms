import { useTranslation } from "react-i18next";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api";
import type { Item } from "@/lib/items";
import { Button } from "@/components/ui/button";
import { useSearchResults } from "@/lib/search";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/portfolio")({ component: Portfolio });

type Project = { id: string; title: string };
type Dependency = { id: string; predecessor_id: string; successor_id: string; type: string; lag: number };
type Timeline = { projects: Project[]; items: { project_id: string; item: Item }[]; dependencies: Dependency[] };

const day = 86_400_000;
const atDay = (value?: string | null) => value ? Date.parse(`${value}T00:00:00Z`) : undefined;

function Portfolio() {
  const { t } = useTranslation();
  const initial = new URLSearchParams(window.location.search).get("projects") ?? "";
  const [projectIDs, setProjectIDs] = useState(() => [...new Set(initial.split(",").map(id => id.trim()).filter(Boolean))].slice(0, 20));
  const [selected, setSelected] = useState<Project[]>(() => projectIDs.map(id => ({ id, title: id })));
  const searchInput = useRef<HTMLInputElement>(null);
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  useEffect(() => {
    const timer = setTimeout(() => setQuery(search.trim()), 250);
    return () => clearTimeout(timer);
  }, [search]);
  const results = useSearchResults(query, "everywhere", {}, 20);
  const matches = query === search.trim()
    ? (results.data?.items ?? []).filter(item => item.type === "project" && !selected.some(project => project.id === item.id))
    : [];
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
    <header><h1 className="workspace-title">{t("settings.portfolio.portfolioGantt")}</h1><p className="text-sm text-muted-foreground">{t("settings.portfolio.compareVisibleProjectSchedulesAndTheirCrossProjectDependencies")}</p></header>
    <form className="max-w-3xl space-y-3" onSubmit={event => {
      event.preventDefault();
      const ids = selected.map(project => project.id);
      setProjectIDs(ids);
      window.history.replaceState(null, "", ids.length ? `/portfolio?projects=${encodeURIComponent(ids.join(","))}` : "/portfolio");
    }}>
      <label className="block text-sm">{t("settings.portfolio.findProjectsByName")}
        <Input ref={searchInput} type="search" value={search} onChange={event => setSearch(event.target.value)} />
      </label>
      {query.length >= 2 && results.isFetching && <p role="status">{t("settings.portfolio.searchingProjects")}</p>}
      {results.error && query === search.trim() && <p role="alert">{results.error.message}</p>}
      {query.length >= 2 && query === search.trim() && results.isSuccess && matches.length === 0 && <p role="status">{t("settings.portfolio.noMatchingProjectsToAdd")}</p>}
      {matches.length > 0 && <ul aria-label={t("settings.portfolio.matchingProjects")} className="space-y-1">
        {matches.map(project => <li key={project.id}>
          <Button type="button" variant="outline" className="h-auto max-w-full whitespace-normal text-left wrap-anywhere"
            disabled={selected.length >= 20} onClick={() => {
              setSelected(previous => [...previous, project]);
              searchInput.current?.focus();
            }}>{project.title}</Button>
        </li>)}
      </ul>}
      {selected.length > 0 && <ul aria-label={t("settings.portfolio.selectedProjects")} className="flex flex-wrap gap-2">
        {selected.map(project => {
          const name = projectByID.get(project.id) ?? project.title;
          return <li key={project.id} className="min-w-0 max-w-full">
            <Button type="button" variant="secondary" className="h-auto max-w-full whitespace-normal text-left wrap-anywhere"
              aria-label={t("settings.portfolio.remove", { name })} onClick={() => {
                setSelected(previous => previous.filter(item => item.id !== project.id));
                searchInput.current?.focus();
              }}>{name} ×</Button>
          </li>;
        })}
      </ul>}
      {selected.length === 20 && <p role="status">{t("settings.portfolio.upTo20ProjectsCanBeSelected")}</p>}
      <Button type="submit" disabled={selected.length === 0 && projectIDs.length === 0}>{t("settings.portfolio.showTimeline")}</Button>
    </form>
    {timeline.isPending && projectIDs.length > 0 && <p role="status">{t("settings.portfolio.loadingPortfolio")}</p>}
    {timeline.error && <p role="alert">{timeline.error.message}</p>}
    {timeline.data && <>
      <section aria-label={t("settings.portfolio.portfolioTimeline")} tabIndex={0} className="overflow-x-auto rounded-lg border focus-visible:outline-2 focus-visible:outline-ring">
        <div className="min-w-[720px] divide-y">
          {timeline.data.items.map(({ project_id: projectID, item }) => {
            const start = atDay(item.start_date) ?? atDay(item.due_date);
            const due = atDay(item.due_date) ?? start;
            const span = range ? Math.max(1, (range.last - range.first) / day + 1) : 1;
            const left = start && range ? ((start - range.first) / day / span) * 100 : 0;
            const width = start && due ? Math.max(1.5, ((due - start) / day + 1) / span * 100) : 0;
            return <div key={item.id} className="grid grid-cols-[16rem_1fr] items-center gap-3 p-2 text-sm">
              <div className="truncate"><span className="text-muted-foreground">{projectByID.get(projectID)} · </span>{item.title}</div>
              <div role="img" className="relative h-6 rounded bg-muted" aria-label={t("settings.portfolio.itemRange", { title: item.title, start: item.start_date ?? t("settings.portfolio.unscheduled"), due: item.due_date ?? t("settings.portfolio.unscheduled") })}>
                {width > 0 && <div className="absolute top-1 h-4 rounded bg-primary" style={{ left: `${left}%`, width: `${width}%` }} />}
              </div>
            </div>;
          })}
        </div>
      </section>
      <section><h2 className="font-semibold">{t("settings.portfolio.crossProjectDependencies")}</h2>
        {timeline.data.dependencies.length === 0 ? <p className="text-sm text-muted-foreground">{t("settings.portfolio.noDependencyConnectsTheSelectedProjects")}</p> :
          <ul className="divide-y">{timeline.data.dependencies.map(dep => <li className="py-2 text-sm" key={dep.id}>
            {itemByID.get(dep.predecessor_id)} → {itemByID.get(dep.successor_id)} ({dep.type}{dep.lag ? t("settings.portfolio.lag", { days: dep.lag }) : ""})
          </li>)}</ul>}
      </section>
    </>}
  </main>;
}
