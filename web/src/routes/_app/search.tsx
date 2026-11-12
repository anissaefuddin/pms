import { useState, type FormEvent } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SearchResultList, SearchScopes } from "@/components/search-results";
import { effectiveScope, useSearchResults, type SearchContext, type SearchScope } from "@/lib/search";

const uuid = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i;

export const Route = createFileRoute("/_app/search")({
  validateSearch: (search: Record<string, unknown>): { q: string; scope: SearchScope } & SearchContext => ({
    q: typeof search.q === "string" ? search.q.trim().slice(0, 200) : "",
    scope: search.scope === "space" || search.scope === "project" ? search.scope : "everywhere",
    spaceId: typeof search.spaceId === "string" && uuid.test(search.spaceId) ? search.spaceId : undefined,
    containerId: typeof search.containerId === "string" && uuid.test(search.containerId) ? search.containerId : undefined,
  }),
  component: SearchPage,
});

function SearchPage() {
  const { t } = useTranslation();
  const search = Route.useSearch();
  const navigate = useNavigate({ from: "/search" });
  const scope = effectiveScope(search.scope, search);
  const result = useSearchResults(search.q, scope, search, 50);
  return <main className="mx-auto w-full max-w-3xl space-y-4 p-6">
    <h1 className="text-xl font-semibold">{t("search.title")}</h1>
    <SearchForm key={search.q} initialQuery={search.q} onSubmit={(q) => void navigate({ search: { ...search, q } })} />
    <SearchScopes context={search} scope={scope} onChange={(scope) => void navigate({ search: { ...search, scope } })} />
    <SearchResultList data={result.data} query={search.q} pending={result.isPending} error={result.isError}
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
