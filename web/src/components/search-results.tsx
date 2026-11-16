import { TypeBadge } from "@/components/type-badge";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { SearchIllustration } from "@/components/ui/empty-illustrations";
import type { SearchContext, SearchResults, SearchScope } from "@/lib/search";

export function SearchScopes({ context, scope, onChange }: {
  context: SearchContext;
  scope: SearchScope;
  onChange: (scope: SearchScope) => void;
}) {
  const { t } = useTranslation();
  const choices: SearchScope[] = ["everywhere"];
  if (context.spaceId) choices.push("space");
  if (context.containerId) choices.push("project");
  if (choices.length === 1) return null;
  return <div className="flex flex-wrap gap-1" role="group" aria-label={t("search.scope")}>
    {choices.map((choice) => <Button key={choice} type="button" size="sm" variant={scope === choice ? "secondary" : "ghost"}
      aria-pressed={scope === choice} onClick={() => onChange(choice)}>{t(`search.${choice}`)}</Button>)}
  </div>;
}

const resultClass = "block rounded-md px-3 py-2 hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring";
const empty: SearchResults = { items: [], comments: [], users: [] };
const itemGroups = ["tasks", "projects"] as const;

export function SearchResultList({ data = empty, query, pending, error, retry, close }: {
  data?: SearchResults;
  query: string;
  pending: boolean;
  error: boolean;
  retry: () => void;
  close?: () => void;
}) {
  const { t } = useTranslation();
  if (query.length < 2) return <p className="px-3 py-2 text-muted-foreground">{t("search.minimum")}</p>;
  if (pending) return <div role="status" className="space-y-2 px-3 py-2">
    <span className="sr-only">{t("search.loading")}</span>
    <div className="h-5 w-3/4 animate-pulse rounded bg-muted" />
    <div className="h-5 w-1/2 animate-pulse rounded bg-muted" />
  </div>;
  if (error) return <div className="px-3 py-2">
    <p role="alert">{t("search.error")}</p><Button variant="link" onClick={retry}>{t("search.retry")}</Button>
  </div>;
  if (!data.items.length && !data.comments.length && !data.users.length) {
    return <EmptyState className="border-0 px-3 py-6" icon={<SearchIllustration />} title={t("search.emptyTitle")} description={t("search.empty", { query })} />;
  }

  return <div className="space-y-3">
    {itemGroups.map((group) => {
      const items = data.items.filter((item) => group === "tasks" ? item.type === "task" : item.type !== "task");
      if (!items.length) return null;
      return <section key={group}>
        <h2 className="px-3 text-xs font-semibold text-muted-foreground">{t(`search.${group}`)}</h2>
        <ul>{items.map((item) => <li key={item.id}>
          <Link to={item.type === "task" ? "/tasks/$itemId" : "/items/$itemId"} params={{ itemId: item.id }}
            className={resultClass} onClick={close} data-search-result>
            <Highlight value={item.title} query={query} />
            {item.item_type && <span className="block text-xs text-muted-foreground"><TypeBadge item={item} /></span>}
          </Link>
        </li>)}</ul>
      </section>;
    })}
    {!!data.comments.length && <section>
      <h2 className="px-3 text-xs font-semibold text-muted-foreground">{t("search.comments")}</h2>
      <ul>{data.comments.map((comment) => <li key={comment.comment_id}>
        <Link to="/tasks/$itemId" params={{ itemId: comment.item_id }} className={resultClass} onClick={close} data-search-result>
          <Highlight value={comment.item_title} query={query} />
          <p className="text-xs text-muted-foreground"><Highlight value={comment.snippet} query={query} /></p>
        </Link>
      </li>)}</ul>
    </section>}
    {!!data.users.length && <section>
      <h2 className="px-3 text-xs font-semibold text-muted-foreground">{t("search.users")}</h2>
      <ul>{data.users.map((user) => <li key={user.id}>
        {close
          ? <Link to="/search" search={{ q: user.name, scope: "everywhere" }} className={resultClass} onClick={close} data-search-result>
            <Highlight value={user.name} query={query} />
          </Link>
          : <p className="px-3 py-2"><Highlight value={user.name} query={query} /></p>}
      </li>)}</ul>
    </section>}
  </div>;
}

function Highlight({ value, query }: { value: string; query: string }) {
  const terms = query.toLocaleLowerCase().match(/[\p{L}\p{M}\p{N}]+/gu) ?? [];
  return value.split(/([\p{L}\p{M}\p{N}]+)/gu).map((part, index) =>
    terms.some((term) => part.toLocaleLowerCase().startsWith(term))
      ? <mark key={index} className="rounded bg-accent text-accent-foreground underline">{part}</mark>
      : part);
}
