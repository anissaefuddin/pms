import { useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter, useSearch } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { components } from "@/api/schema";
import { api, type CurrentUser } from "@/lib/api";
import { itemKeys, listItems, type Item, type ItemUpdate, type SavedView } from "@/lib/items";
import { Input } from "@/components/ui/input";
import { CreateItemDialog } from "@/components/create-item-dialog";
import { ItemTable } from "@/components/item-table";
import { ItemBoard } from "@/components/item-board";
import { ViewTabs } from "@/components/view-tabs";

export const viewSorts = ["manual", "title", "status", "start_date", "due_date"] as const;
const viewGroups = ["none", "status", "type"] as const;

export function ViewWorkspace({ spaceId, parent, me, canCreate, canEdit, onOpen }: {
  spaceId: string;
  parent?: Item;
  me?: CurrentUser;
  canCreate: boolean;
  canEdit: boolean;
  onOpen: (item: Item) => void;
}) {
  const { t } = useTranslation();
  const router = useRouter();
  const search = useSearch({ strict: false });
  const qc = useQueryClient();
  const base = parent ? `/api/v1/items/${parent.id}` : `/api/v1/spaces/${spaceId}`;
  const sort = search.sort ?? "manual";
  const filter = search.filter ?? "";
  const group = search.group === "status" || search.group === "type" ? search.group : undefined;
  const viewKey = ["views", base];
  const listKey = parent ? itemKeys.children(parent.id) : itemKeys.root(spaceId);
  const sortedKey = sort === "manual" ? listKey : [...listKey, sort];
  const views = useQuery({ queryKey: viewKey, queryFn: () => api<{ items: SavedView[] }>(base + "/views") });
  const available = views.data?.items ?? [];
  const active = available.find((view) => view.id === search.view) ?? available.find((view) => view.is_default) ?? available[0];
  const items = useQuery({
    queryKey: sortedKey,
    queryFn: () => listItems(base + (parent ? "/children" : "/items"), sort === "manual" ? undefined : sort),
  });
  const workflows = useQuery({
    queryKey: ["workflows"], enabled: !!active,
    queryFn: () => api<{ items: components["schemas"]["Workflow"][] }>("/api/v1/workflows"),
  });
  const workflow = workflows.data?.items.find((wf) => wf.id === parent?.default_workflow_id)
    ?? workflows.data?.items.find((wf) => wf.statuses.some((status) => status.id === parent?.status?.id))
    ?? workflows.data?.items.find((wf) => wf.is_default);
  const [pending, setPending] = useState(false);
  const [creating, setCreating] = useState<{ status?: string }>();
  const locked = useRef(false);
  const isList = { queryKey: itemKeys.all, predicate: ({ queryKey }: { queryKey: readonly unknown[] }) => queryKey[1] !== "detail" };

  function setSearch(key: string, value: string, push = false) {
    const url = new URL(window.location.href);
    if (value) url.searchParams.set(key, value);
    else url.searchParams.delete(key);
    const href = url.pathname + url.search + url.hash;
    if (push) router.history.push(href);
    else router.history.replace(href);
  }
  async function patch(item: Item, update: ItemUpdate) {
    if (locked.current) return;
    locked.current = true;
    setPending(true);
    const snapshots = qc.getQueriesData<Item[]>(isList);
    const status = workflow?.statuses.find((candidate) => candidate.id === update.status_id);
    if (status) {
      qc.setQueriesData<Item[]>(isList, (list) => list?.map((entry) => entry.id === item.id ? { ...entry, status } : entry));
    }
    try {
      const updated = await api<Item>(`/api/v1/items/${item.id}`, {
        method: "PATCH", headers: { "If-Match": `"${item.version}"` }, body: JSON.stringify(update),
      });
      qc.setQueriesData<Item[]>(isList, (list) => list?.map((entry) => entry.id === item.id ? updated : entry));
      await qc.invalidateQueries({ queryKey: itemKeys.detail(item.id), exact: true });
    } catch (err) {
      for (const [key, snapshot] of snapshots) qc.setQueryData(key, snapshot);
      toast.error((err as Error).message);
    } finally {
      await qc.invalidateQueries(isList);
      locked.current = false;
      setPending(false);
    }
  }
  async function reorder(item: Item, parentId: string | undefined, after: string | null) {
    if (locked.current) return;
    locked.current = true;
    setPending(true);
    try {
      await api(`/api/v1/items/${item.id}/reorder`, { method: "POST", body: JSON.stringify({
        space_id: spaceId, parent_item_id: parentId ?? null, after_item_id: after,
      }) });
      await qc.invalidateQueries({ queryKey: itemKeys.detail(item.id), exact: true });
    } catch (err) { toast.error((err as Error).message); }
    finally {
      await qc.invalidateQueries(isList);
      locked.current = false;
      setPending(false);
    }
  }

  // The existing Space empty state stays available while the View list loads.
  if (!active) return views.isError && !!items.data?.length
    ? <ul className="mt-4 divide-y rounded border">{items.data.map((item) => <li key={item.id}>
      <button type="button" className="block w-full p-3 text-left" onClick={() => onOpen(item)}>{item.title}</button>
    </li>)}</ul> : null;
  const filtered = (items.data ?? []).filter((item) => item.title.toLocaleLowerCase().includes(filter.trim().toLocaleLowerCase()));
  const collection = {
    items: filtered, statuses: workflow?.statuses ?? [], spaceId, parentId: parent?.id,
    canCreate: !!spaceId && canCreate, pending, onOpen, onAdd: (status?: string) => setCreating({ status }), onPatch: patch,
  };
  return <section className="min-w-0 space-y-3">
    <ViewTabs views={available} selected={active} canEdit={canEdit}
      onSelect={(id) => setSearch("view", id, true)} onRefresh={async () => { await qc.invalidateQueries({ queryKey: viewKey }); }} />
    <div className="flex flex-wrap items-end gap-3">
      <label className="space-y-1"><span>{t("views.filter")}</span>
        <Input value={filter} maxLength={120} onChange={(event) => setSearch("filter", event.target.value)} /></label>
      <label className="space-y-1"><span>{t("views.sort")}</span>
        <select aria-label={t("views.sort")} value={sort} onChange={(event) => setSearch("sort", event.target.value)}
          className="block rounded border bg-background p-2">
          {viewSorts.map((value) => <option value={value} key={value}>{t(`views.${value}`)}</option>)}
        </select></label>
      <label className="space-y-1"><span>{t("views.group")}</span>
        <select aria-label={t("views.group")} value={group ?? "none"} onChange={(event) => setSearch("group", event.target.value === "none" ? "" : event.target.value)}
          className="block rounded border bg-background p-2">
          {viewGroups.map((value) => <option value={value} key={value}>{t(`views.${value}`)}</option>)}
        </select></label>
    </div>
    {items.isPending && <p role="status">{t("items.loading")}</p>}
    {items.isError && <p role="alert">{t("items.loadError")}</p>}
    {items.isSuccess && (active.type === "board"
      ? <ItemBoard {...collection} />
      : <ItemTable {...collection} group={group} manual={!!spaceId && sort === "manual" && !group && !filter} onReorder={reorder} />)}
    {me && creating && <CreateItemDialog open onOpenChange={(open) => { if (!open) setCreating(undefined); }}
      taskOnly initialStatus={creating.status} location={{ space_id: spaceId, parent_item_id: parent?.id }} me={me} onCreated={onOpen} />}
  </section>;
}
