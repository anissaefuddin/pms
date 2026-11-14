import { formatDate, useProfile } from "@/lib/profile";
import { createFileRoute, Link, useLocation, useRouter } from "@tanstack/react-router";
import { useState, type FormEvent } from "react";
import { useInfiniteQuery, useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ArrowDownUp, Filter, ListChecks, Plus } from "lucide-react";
import { toast } from "sonner";
import type { components } from "@/api/schema";
import { Button } from "@/components/ui/button";
import { ApiError, api } from "@/lib/api";
import { getItem, itemKeys, type Item, type ItemPage } from "@/lib/items";
import { Avatars, StatusPill } from "@/components/todo-chips";
import { bucketOrder, dueBucket, groupBy } from "@/lib/todo-groups";
import { useSpaces } from "@/lib/space-queries";

export const Route = createFileRoute("/_app/my-todo")({ component: MyTodo });

const filters = ["all", "active", "overdue"];
const sorts = ["date", "priority", "importance", "date_modified", "date_created", "status", "title"];
const quickTypes = ["task", "project", "folder"];
const selectClass = "rounded-md border-0 bg-transparent px-1 py-1 text-sm font-medium text-primary hover:bg-muted";

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

  const [twoLine, setTwoLine] = useState(false);
  const [byLocation, setByLocation] = useState(false);
  const [showDuration, setShowDuration] = useState(false);
  const [quickTitle, setQuickTitle] = useState("");
  const [quickType, setQuickType] = useState("task");
  const personal = spaces.data?.find((space) => space.kind === "personal");
  const quickAdd = useMutation({
    mutationFn: () => api("/api/v1/items", { method: "POST", body: JSON.stringify({
      type: quickType, title: quickTitle.trim(), locations: [{ space_id: personal!.id }],
      ...(quickType === "task" && { assignee_ids: [profile!.id] }),
    }) }),
    onSuccess: () => { setQuickTitle(""); void queryClient.invalidateQueries({ queryKey: itemKeys.all }); },
    onError: () => toast.error(t("nav.todo.quick.error")),
  });
  function submitQuick(event: FormEvent) {
    event.preventDefault();
    if (quickTitle.trim() && personal) quickAdd.mutate();
  }
  const today = new Date().toLocaleDateString("sv");
  const locationOf = (item: Item) => item.locations.flatMap((location) => {
    const name = (location.parent_item_id && parentTitles.get(location.parent_item_id))
      || spaces.data?.find((space) => space.id === location.space_id)?.name;
    return name ? [name] : [];
  })[0] ?? "";
  const groups = byLocation ? groupBy(items, (item) => locationOf(item) || t("nav.todo.group.none"))
    : sort === "status" ? groupBy(items, (item) => item.status?.name ?? t("nav.todo.group.noStatus"))
    : sort === "date" ? groupBy(items, (item) => dueBucket(item.due_date, today), bucketOrder).map((g) => ({ ...g, key: t(`nav.todo.group.${g.key}`) }))
    : [{ key: "", rows: items }];

  function updateSearch(key: "filter" | "sort", value: string) {
    const url = new URL(window.location.href);
    url.searchParams.set(key, value);
    router.history.push(url.pathname + url.search + url.hash);
  }

  return <main className="space-y-4 p-6">
    <h1 className="workspace-title">{t("nav.todo.label")}</h1>
    <div className="flex flex-wrap items-center gap-3 border-b pb-2">
      <label className="flex items-center gap-1 text-sm"><Filter aria-hidden="true" className="size-4 text-primary" /><span className="sr-only">{t("nav.todo.filter")}</span>
        <select className={selectClass} value={filter} onChange={(event) => updateSearch("filter", event.target.value)}>
          {filters.map((value) => <option key={value} value={value}>{t(`nav.todo.filters.${value}`)}</option>)}
        </select>
      </label>
      <label className="flex items-center gap-1 text-sm"><ArrowDownUp aria-hidden="true" className="size-4" /><span className="sr-only">{t("nav.todo.sort")}</span>
        <select className={selectClass} value={sort} onChange={(event) => updateSearch("sort", event.target.value)}>
          {sorts.map((value) => <option key={value} value={value}>{t(`nav.todo.sorts.${value}`)}</option>)}
        </select>
      </label>
      <details className="relative text-sm">
        <summary className="flex cursor-pointer items-center gap-1 rounded-md px-2 py-1 hover:bg-muted"><ListChecks aria-hidden="true" className="size-4" />{t("nav.todo.options")}</summary>
        <div className="absolute z-10 mt-1 space-y-1 rounded border bg-background p-3 shadow">
          <label className="flex items-center gap-2"><input type="checkbox" checked={twoLine} onChange={(e) => setTwoLine(e.target.checked)} />{t("nav.todo.opt.twoLine")}</label>
          <label className="flex items-center gap-2"><input type="checkbox" checked={byLocation} onChange={(e) => setByLocation(e.target.checked)} />{t("nav.todo.opt.hierarchy")}</label>
          <label className="flex items-center gap-2"><input type="checkbox" checked={showDuration} onChange={(e) => setShowDuration(e.target.checked)} />{t("nav.todo.opt.duration")}</label>
        </div>
      </details>
    </div>
    {personal && <form className="flex flex-wrap items-center gap-2 border-b pb-2" onSubmit={submitQuick}>
      <Plus aria-hidden="true" className="size-4 text-muted-foreground" />
      <input className="min-w-48 flex-1 bg-transparent px-1 py-1 text-sm outline-none focus-visible:ring-1 focus-visible:ring-ring" aria-label={t("nav.todo.quick.title")} placeholder={t("nav.todo.quick.title")}
        value={quickTitle} onChange={(e) => setQuickTitle(e.target.value)} />
      <select className={selectClass} aria-label={t("nav.todo.quick.type")} value={quickType} onChange={(e) => setQuickType(e.target.value)}>
        {quickTypes.map((type) => <option key={type} value={type}>{t(`nav.todo.quick.types.${type}`)}</option>)}
      </select>
      <Button type="submit" size="sm" variant="ghost" disabled={quickAdd.isPending || !quickTitle.trim()}>{t("nav.todo.quick.add")}</Button>
    </form>}
    {query.isPending && <p role="status">{t("items.loading")}</p>}
    {query.isError && <div>
      <p role="alert">{t("items.loadError")}</p>
      <Button variant="link" onClick={() => void query.refetch()}>{t("items.retry")}</Button>
    </div>}
    {query.isSuccess && items.length === 0 && <p>{t("nav.todo.empty")}</p>}
    {groups.map((group) => <section key={group.key} aria-label={group.key || undefined}>
      {group.key && <h2 className="mt-4 text-xs font-semibold uppercase tracking-wide">{group.key} ({group.rows.length})</h2>}
    <ul className="divide-y">
      {group.rows.map((item) => {
        const statuses = (workflows.data?.items.find((workflow) => workflow.statuses.some((status) => status.id === item.status?.id))
          ?? workflows.data?.items.find((workflow) => workflow.is_default))?.statuses ?? [];
        const locations = item.locations.flatMap((location) => {
          const name = (location.parent_item_id && parentTitles.get(location.parent_item_id))
            || spaces.data?.find((space) => space.id === location.space_id)?.name;
          return name ? [name] : [];
        });
        return <li key={item.id} className={`flex flex-wrap items-center gap-3 text-sm ${twoLine ? "py-4" : "py-2"}`}>
          <Avatars people={item.assignees ?? []} />
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
            : item.status && <StatusPill name={item.status.name} color={item.status.color} group={item.status.group} />}
          {showDuration && item.duration_days != null && <span className="text-muted-foreground">{t("nav.todo.duration", { count: item.duration_days })}</span>}
        </li>;
      })}
    </ul></section>)}
    {query.hasNextPage && <Button variant="outline" disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>
      {t("nav.todo.more")}
    </Button>}
  </main>;
}
