import { formatDate, useProfile } from "@/lib/profile";
import { createFileRoute, Link, useLocation, useRouter } from "@tanstack/react-router";
import { useInfiniteQuery, useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { components } from "@/api/schema";
import { Button } from "@/components/ui/button";
import { ApiError, api } from "@/lib/api";
import { getItem, itemKeys, type Item, type ItemPage } from "@/lib/items";
import { useSpaces } from "@/lib/space-queries";

export const Route = createFileRoute("/_app/my-todo")({ component: MyTodo });

const filters = ["all", "active", "overdue"];
const sorts = ["date", "date_modified", "date_created", "status", "title"];
const selectClass = "rounded border bg-background px-2 py-1 text-sm";

function MyTodo() {
  const { t } = useTranslation();
  const profile = useProfile();
  const router = useRouter();
  const search = new URLSearchParams(useLocation({ select: (location) => location.searchStr }));
  const requestedFilter = search.get("filter") ?? "";
  const requestedSort = search.get("sort") ?? "";
  const filter = filters.includes(requestedFilter) ? requestedFilter : "active";
  const sort = sorts.includes(requestedSort) ? requestedSort : "date";
  const queryClient = useQueryClient();
  const query = useInfiniteQuery({
    queryKey: [...itemKeys.all, "todo", filter, sort],
    initialPageParam: "",
    queryFn: ({ pageParam, signal }) => {
      const params = new URLSearchParams({ filter, sort });
      if (pageParam) params.set("cursor", pageParam);
      return api<ItemPage>(`/api/v1/me/todo?${params}`, { signal });
    },
    getNextPageParam: (lastPage) => lastPage.next_cursor,
  });
  const items = query.data?.pages.flatMap((page) => page.items) ?? [];
  const spaces = useSpaces();
  const parentIds = [...new Set(items.flatMap((item) =>
    item.locations.flatMap((location) => location.parent_item_id ? [location.parent_item_id] : [])))];
  const parents = useQueries({ queries: parentIds.map((id) => ({
    queryKey: itemKeys.detail(id), queryFn: () => getItem(id),
  })) });
  const parentTitles = new Map(parentIds.map((id, index) => [id, parents[index].data?.item.title]));
  const workflows = useQuery({
    queryKey: ["workflows"],
    enabled: items.some((item) => item.my_capabilities.includes("change_status")),
    queryFn: () => api<{ items: components["schemas"]["Workflow"][] }>("/api/v1/workflows"),
  });
  const changeStatus = useMutation({
    mutationFn: ({ item, status }: { item: Item; status: string }) => api(`/api/v1/items/${item.id}`, {
      method: "PATCH", headers: { "If-Match": `"${item.version}"` }, body: JSON.stringify({ status_id: status }),
    }),
    onError: (error) => toast.error(error instanceof ApiError && error.status === 409
      ? t("items.statusConflict") : error.message),
    onSettled: () => queryClient.invalidateQueries({ queryKey: itemKeys.all }),
  });

  function updateSearch(key: "filter" | "sort", value: string) {
    const url = new URL(window.location.href);
    url.searchParams.set(key, value);
    router.history.push(url.pathname + url.search + url.hash);
  }

  return <main className="space-y-4 p-6">
    <h1 className="text-xl font-semibold">{t("nav.todo.label")}</h1>
    <div className="flex flex-wrap gap-4">
      <label className="flex items-center gap-2 text-sm">{t("nav.todo.filter")}
        <select className={selectClass} value={filter} onChange={(event) => updateSearch("filter", event.target.value)}>
          {filters.map((value) => <option key={value} value={value}>{t(`nav.todo.filters.${value}`)}</option>)}
        </select>
      </label>
      <label className="flex items-center gap-2 text-sm">{t("nav.todo.sort")}
        <select className={selectClass} value={sort} onChange={(event) => updateSearch("sort", event.target.value)}>
          {sorts.map((value) => <option key={value} value={value}>{t(`nav.todo.sorts.${value}`)}</option>)}
        </select>
      </label>
    </div>
    {query.isPending && <p role="status">{t("items.loading")}</p>}
    {query.isError && <div>
      <p role="alert">{t("items.loadError")}</p>
      <Button variant="link" onClick={() => void query.refetch()}>{t("items.retry")}</Button>
    </div>}
    {query.isSuccess && items.length === 0 && <p>{t("nav.todo.empty")}</p>}
    <ul className="divide-y">
      {items.map((item) => {
        const statuses = (workflows.data?.items.find((workflow) => workflow.statuses.some((status) => status.id === item.status?.id))
          ?? workflows.data?.items.find((workflow) => workflow.is_default))?.statuses ?? [];
        const locations = item.locations.flatMap((location) => {
          const name = (location.parent_item_id && parentTitles.get(location.parent_item_id))
            || spaces.data?.find((space) => space.id === location.space_id)?.name;
          return name ? [name] : [];
        });
        return <li key={item.id} className="flex flex-wrap items-center gap-3 py-3 text-sm">
          <Link className="min-w-40 flex-1 font-medium hover:underline" to="/tasks/$itemId" params={{ itemId: item.id }}>{item.title}</Link>
          {locations.length > 0 && <span className="rounded bg-muted px-2 py-1 text-foreground">{[...new Set(locations)].join(", ")}</span>}
          {item.due_date && <time className="text-muted-foreground" dateTime={item.due_date}>{formatDate(item.due_date, profile)}</time>}
          {item.my_capabilities.includes("change_status") && statuses.length > 0
            ? <select className={selectClass} aria-label={t("nav.todo.status", { title: item.title })}
                value={item.status?.id ?? ""} disabled={changeStatus.isPending}
                onChange={(event) => changeStatus.mutate({ item, status: event.target.value })}>
                {!item.status && <option value="" disabled>{t("views.noStatus")}</option>}
                {statuses.map((status) => <option key={status.id} value={status.id}>{status.name}</option>)}
              </select>
            : item.status && <span>{item.status.name}</span>}
        </li>;
      })}
    </ul>
    {query.hasNextPage && <Button variant="outline" disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>
      {t("nav.todo.more")}
    </Button>}
  </main>;
}
