import { useState, type FormEvent } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SearchResultList, SearchScopes } from "@/components/search-results";
import { effectiveScope, useSearchResults, type SearchContext, type SearchScope } from "@/lib/search";

const presets = ["all", "tasks", "projects", "comments", "users"] as const;
const statuses = ["active", "completed", "deferred", "cancelled"] as const;
const sorts = ["relevance", "date_modified"] as const;
type Preset = (typeof presets)[number];
const uuid = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i;

export const Route = createFileRoute("/_app/search")({
  validateSearch: (search: Record<string, unknown>): { q: string; scope: SearchScope; show?: Preset; assigned?: boolean; status?: string; order?: string } & SearchContext => ({
    show: presets.find((value) => value === search.show),
    assigned: search.assigned === true ? true : undefined,
    status: statuses.find((value) => value === search.status),
    order: sorts.find((value) => value === search.order),
    q: typeof search.q === "string" ? search.q.trim().slice(0, 200) : "",
    scope: search.scope === "space" || search.scope === "project" ? search.scope : "everywhere",
    spaceId: typeof search.spaceId === "string" && uuid.test(search.spaceId) ? search.spaceId : undefined,
    containerId: typeof search.containerId === "string" && uuid.test(search.containerId) ? search.containerId : undefined,
  }),
  component: SearchPage,
});

function SearchPage() {
  const { t } = useTranslation();
  const { show: preset = "all", order: sort = "relevance", ...search } = Route.useSearch();
  const navigate = useNavigate({ from: "/search" });
  const scope = effectiveScope(search.scope, search);
  const result = useSearchResults(search.q, scope, search, 50, true, { assigned: search.assigned, status: search.status, sort });
  const data = result.data && {
    items: result.data.items.filter((item) => preset === "all" || preset === (item.type === "task" ? "tasks" : "projects")),
    comments: preset === "all" || preset === "comments" ? result.data.comments : [],
    users: preset === "all" || preset === "users" ? result.data.users : [],
  };
  const set = (patch: Record<string, unknown>) => void navigate({ search: { ...search, show: preset, order: sort, ...patch } });
  return <main className="mx-auto w-full max-w-3xl space-y-4 p-6">
    <h1 className="text-xl font-semibold">{t("search.title")}</h1>
    <SearchForm key={search.q} initialQuery={search.q} onSubmit={(q) => void navigate({ search: { ...search, q } })} />
    <SearchScopes context={search} scope={scope} onChange={(scope) => void navigate({ search: { ...search, scope } })} />
    <div className="flex flex-wrap gap-1" role="group" aria-label={t("search.preset")}>
      {presets.map((value) => <Button key={value} type="button" size="sm" variant={preset === value ? "secondary" : "ghost"}
        aria-pressed={preset === value} onClick={() => set({ show: value })}>{t(`search.presets.${value}`)}</Button>)}
    </div>
    <div className="flex flex-wrap items-center gap-4 text-sm" role="group" aria-label={t("search.filters")}>
      <label className="flex items-center gap-2">
        <input type="checkbox" checked={!!search.assigned} onChange={(event) => set({ assigned: event.target.checked })} />{t("search.assignedToMe")}
      </label>
      <label className="flex items-center gap-2">{t("search.statusGroup")}
        <select className="h-8 rounded-md border border-input bg-background px-2" value={search.status ?? ""}
          onChange={(event) => set({ status: event.target.value || undefined })}>
          <option value="">{t("search.statusAny")}</option>
          {statuses.map((value) => <option key={value} value={value}>{t(`search.statuses.${value}`)}</option>)}
        </select>
      </label>
      <label className="flex items-center gap-2">{t("search.sort")}
        <select className="h-8 rounded-md border border-input bg-background px-2" value={sort} onChange={(event) => set({ order: event.target.value })}>
          {sorts.map((value) => <option key={value} value={value}>{t(`search.sorts.${value}`)}</option>)}
        </select>
      </label>
    </div>
    <SearchResultList data={data} query={search.q} pending={result.isPending} error={result.isError}
      retry={() => void result.refetch()} />
  </main>;
}

function SearchForm({ initialQuery, onSubmit }: { initialQuery: string; onSubmit: (q: string) => void }) {
  const { t } = useTranslation();
  const [query, setQuery] = useState(initialQuery);
  function submit(event: FormEvent) {
    event.preventDefault();
    onSubmit(query.trim());
  }
  return <form className="flex gap-2" onSubmit={submit}>
    <Input type="search" maxLength={200} value={query} onChange={(event) => setQuery(event.target.value)}
      aria-label={t("search.keyword")} placeholder={t("search.placeholder")} />
    <Button type="submit">{t("search.title")}</Button>
  </form>;
}
