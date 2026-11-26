import { useTranslation } from "react-i18next";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { Suspense, lazy, useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api";
import type { Item } from "@/lib/items";
import { Button } from "@/components/ui/button";
import { useSearchResults } from "@/lib/search";
import { Input } from "@/components/ui/input";
import { TopbarTitle } from "@/components/topbar-title";

export const Route = createFileRoute("/_app/portfolio")({ component: Portfolio });

type Project = { id: string; title: string };
type Dependency = { id: string; predecessor_id: string; successor_id: string; type: string; lag: number };
type Timeline = { projects: Project[]; items: { project_id: string; item: Item }[]; dependencies: Dependency[] };

const ItemGantt = lazy(() => import("@/components/item-gantt").then((module) => ({ default: module.ItemGantt })));

function Portfolio() {
  const { t } = useTranslation();
  const navigate = useNavigate();
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
  // The Gantt reads each level below a project itself, so every row passes the same access check as the project page.
  const roots = useMemo(() => {
    const ids = new Set(timeline.data?.projects.map((project) => project.id));
    return timeline.data?.items.map(({ item }) => item).filter((item) => ids.has(item.id)) ?? [];
  }, [timeline.data]);
  const projectByID = new Map(timeline.data?.projects.map(x => [x.id, x.title]));
  const itemByID = new Map(timeline.data?.items.map(x => [x.item.id, x.item.title]));

  return <main className="min-w-0 space-y-5 p-4 sm:p-8">
    <TopbarTitle>{t("settings.portfolio.portfolioGantt")}</TopbarTitle>
    <header><p className="text-sm text-muted-foreground">{t("settings.portfolio.compareVisibleProjectSchedulesAndTheirCrossProjectDependencies")}</p></header>
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
      <section aria-label={t("settings.portfolio.portfolioTimeline")}>
        <Suspense fallback={<p role="status">{t("settings.portfolio.loadingPortfolio")}</p>}>
          <ItemGantt key={projectIDs.join()} items={roots} statuses={[]} customFields={[]} columns={["status", "start_date", "due_date"]} spaceId="" readonly
            onOpen={(item) => void navigate({ to: "/tasks/$itemId", params: { itemId: item.id } })} onDelete={() => {}} onPatch={async () => {}} />
        </Suspense>
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
