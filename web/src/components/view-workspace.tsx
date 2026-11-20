import { Suspense, lazy, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter, useSearch } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { components } from "@/api/schema";
import { ApiError, api, type CurrentUser } from "@/lib/api";
import { isProjectDetail, itemKeys, listItems, listSubtree, subtreeChildren, type Item, type ItemUpdate, type SavedView } from "@/lib/items";
import { Button } from "@/components/ui/button";
import { CreateItemDialog } from "@/components/create-item-dialog";
import { ItemTable } from "@/components/item-table";
import { ItemBoard } from "@/components/item-board";
import { createItemOnce, quickTaskInput } from "@/lib/create-item-service";
const ItemCalendar = lazy(() => import("@/components/item-calendar").then((module) => ({ default: module.ItemCalendar })));
const ItemGantt = lazy(() => import("@/components/item-gantt").then((module) => ({ default: module.ItemGantt })));
import { Timelog } from "@/components/timelog";
import { ProjectChart, ProjectFiles } from "@/components/project-files";
import { ViewTabs } from "@/components/view-tabs";
import { useItemTypes } from "@/lib/item-types";
import { ColumnPicker } from "@/components/view-columns";
import { ProjectExcelExport } from "@/components/project-excel-export";
import { useSpaces } from "@/lib/space-queries";
import { can } from "@/lib/spaces";
import { ViewToolbar } from "@/components/view-toolbar";
import { EmptyState } from "@/components/ui/empty-state";
import { ProjectIllustration } from "@/components/ui/empty-illustrations";
import type { CustomField } from "@/lib/custom-fields";
import { visibleColumns } from "@/lib/view-columns";
import { useViewExpansion } from "@/lib/use-view-expansion";

export { viewSorts } from "@/components/view-toolbar";
const tableGroups = ["none", "status", "type"] as const;
const boardGroups = ["none", "assignee"] as const;

export function ViewWorkspace({ spaceId, parent, me, canCreate, canEdit, onOpen, spaceEmpty = false }: {
  spaceId: string;
  spaceEmpty?: boolean;
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
  const fieldFilter = search.field ?? "";
  const preset = search.preset ?? "";
  const typeFilter = typeof search.type === "string" ? search.type : "";
  const viewKey = ["views", base];
  const listKey = parent ? itemKeys.children(parent.id) : itemKeys.root(spaceId);
  const sortedKey = sort === "manual" && !fieldFilter && !preset && !typeFilter ? listKey : [...listKey, sort, fieldFilter, preset, typeFilter];
  const views = useQuery({ queryKey: viewKey, queryFn: () => api<{ items: SavedView[] }>(base + "/views") });
  const available = views.data?.items ?? [];
  const active = available.find((view) => view.id === search.view) ?? available.find((view) => view.is_default) ?? available[0];

  // WA-D05: expansion is saved on the server per view. Each view is keyed by its id, so state never carries over between views.
  const expansion = useViewExpansion(active?.id);
  const isBoard = active?.type === "board";
  const group = !isBoard && (search.group === "status" || search.group === "type") ? search.group : undefined;
  const swimlanes = isBoard && search.group === "assignee";
  const isGantt = active?.type === "gantt";
  const isCalendar = active?.type === "calendar";
  const flat = isCalendar || isBoard;
  const isCollection = active?.type === "table" || active?.type === "board";
  const items = useQuery({
    queryKey: flat ? [...sortedKey, "subtree"] : sortedKey, enabled: !active || isCollection || isGantt || isCalendar || active?.type === "chart",
    queryFn: async () => {
      const query = (path: string) => listItems(path, sort === "manual" ? undefined : sort, typeFilter || undefined, fieldFilter || undefined, preset || undefined);
      const top = await query(base + (parent ? "/children" : "/items"));
      // Space-level Boards and Calendars keep their top-level cards (projects); the walk applies inside a project or folder.
      return flat && parent ? listSubtree(top, (id) => subtreeChildren(qc, id)) : top;
    },
  });
  // The Space's applicable fields: its own and the Workspace fields assigned to it (DR-08), without those the viewer cannot read.
  const fieldQueries = useQuery({
    queryKey: ["space-applicable-fields", spaceId], enabled: (isCollection || isCalendar || isGantt) && !!spaceId,
    queryFn: () => api<{ items: CustomField[] }>(`/api/v1/spaces/${spaceId}/fields`),
  });
  const spaceList = useSpaces(!!spaceId);
  const spaceInfo = spaceList.data?.find((space) => space.id === spaceId);
  const canManageFields = !!spaceInfo && can(spaceInfo, "share");
  const customFields = (fieldQueries.data?.items ?? []).filter((field) => !field.archived_at);
  const [savingColumns, setSavingColumns] = useState(false);
  const [exportingTable, setExportingTable] = useState(false);
  const workflows = useQuery({
    queryKey: ["workflows"], enabled: isCollection,
    queryFn: () => api<{ items: components["schemas"]["Workflow"][] }>("/api/v1/workflows"),
  });
  const workflow = workflows.data?.items.find((wf) => wf.id === parent?.default_workflow_id)
    ?? workflows.data?.items.find((wf) => wf.statuses.some((status) => status.id === parent?.status?.id))
    ?? workflows.data?.items.find((wf) => wf.is_default);
  const itemTypes = useItemTypes(isCollection);
  const [blocked, setBlocked] = useState<{ title: string; problems: string[] }>();
  const [pending, setPending] = useState(false);
  const [creating, setCreating] = useState<{ status?: string; parent?: Item; day?: string; finishDay?: string; locationParent?: string | null; placement?: { parentId: string | null; after: string | null } }>();
  // Batch 3B: a Task created above or below a Gantt row is placed next to that row with the reorder endpoint.
  async function placeNextTo(item: Item, placement: { parentId: string | null; after: string | null }) {
    if (!spaceId) return;
    await api(`/api/v1/items/${item.id}/reorder`, { method: "POST", body: JSON.stringify({ space_id: spaceId, parent_item_id: placement.parentId, after_item_id: placement.after }) });
    await qc.invalidateQueries({ queryKey: itemKeys.all, predicate: ({ queryKey }) => queryKey[1] !== "detail" });
  }
  // B2.2: a Board column creates its Task through the shared service. B2.4: if an active filter hides the new item,
  // say so instead of implying the create failed. The filter is left as the user set it.
  async function quickCreate(columnId: string, title: string, id: string) {
    if (!me || !spaceId) throw new Error(t("views.createFailed"));
    const item = await createItemOnce(qc, { ...quickTaskInput(me, spaceId, parent?.id, title, columnId), id }, me);
    if (!noticeIfHidden(item)) toast.success(t("items.created", { name: item.title }));
  }
  // B2.4 / R4: a create that the active filter hides must say so, for every entry point (Board, Table "+ Add", the dialog).
  // Only root items are judged here: a subitem sits under a row that may be collapsed, so the list cannot show it yet.
  // Returns true when it reported that the item is hidden. Filters are never changed.
  function noticeIfHidden(item: Item): boolean {
    if (item.locations.some((loc) => loc.parent_item_id)) return false;
    const listKey = flat ? [...sortedKey, "subtree"] : sortedKey;
    const listed = (qc.getQueryData<Item[]>(listKey) ?? []).some((entry) => entry.id === item.id);
    const matchesTitle = item.title.toLocaleLowerCase().includes(filter.trim().toLocaleLowerCase());
    if (listed && matchesTitle) return false;
    toast.info(t("views.createdHidden"), { action: { label: t("views.openCreated"), onClick: () => onOpen(item) } });
    return true;
  }
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
  // B-02: sequential, one PATCH per item with its own version. Successful items are never sent again.
  async function bulkPatch(targets: Item[], update: ItemUpdate) {
    const failed: { title: string; message: string }[] = [];
    let done = 0;
    setPending(true);
    try {
      for (const item of targets) {
        try {
          await api<Item>(`/api/v1/items/${item.id}`, { method: "PATCH", headers: { "If-Match": `"${item.version}"` }, body: JSON.stringify(update) });
          done += 1;
        } catch (err) {
          failed.push({ title: item.title, message: (err as Error).message });
        }
      }
    } finally {
      setPending(false);
      await qc.invalidateQueries(isList);
    }
    return { done, failed };
  }

  async function patch(item: Item, update: ItemUpdate) {
    if (locked.current) return;
    locked.current = true;
    setPending(true);
    setBlocked(undefined);
    const snapshots = qc.getQueriesData<Item[]>(isList);
    const status = workflow?.statuses.find((candidate) => candidate.id === update.status_id);
    let conflict = false;
    if (status) {
      qc.setQueriesData<Item[]>(isList, (list) => list?.map((entry) => entry.id === item.id ? { ...entry, status } : entry));
    }
    try {
      const updated = await api<Item>(`/api/v1/items/${item.id}`, {
        method: "PATCH", headers: { "If-Match": `"${item.version}"` }, body: JSON.stringify(update),
      });
      qc.setQueriesData<Item[]>(isList, (list) => list?.map((entry) => entry.id === item.id ? updated : entry));
      await qc.invalidateQueries({ queryKey: ["items", "detail"],
        predicate: (query) => query.queryKey[2] === item.id || isProjectDetail(query),
      });
    } catch (err) {
      for (const [key, snapshot] of snapshots) qc.setQueryData(key, snapshot);
      conflict = err instanceof ApiError && (err.status === 409 || err.status === 428);
      if (conflict) await qc.invalidateQueries({ queryKey: itemKeys.detail(item.id), exact: true });
      else if (err instanceof ApiError && err.code === "transition_blocked") {
        setBlocked({ title: item.title, problems: err.fields.length ? err.fields.map((problem) => problem.message) : [err.message] });
      } else toast.error((err as Error).message);
    } finally {
      await qc.invalidateQueries(isList);
      if (conflict) toast.error(t("items.statusConflict"));
      locked.current = false;
      setPending(false);
    }
  }
  async function saveColumns(columns: string[]) {
    if (!active) return;
    setSavingColumns(true);
    try {
      await api(`/api/v1/views/${active.id}`, { method: "PATCH", body: JSON.stringify({ config: { ...active.config, columns } }) });
      await qc.invalidateQueries({ queryKey: viewKey });
    } catch { toast.error(t("views.columns.saveError")); }
    finally { setSavingColumns(false); }
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

  const hasFilters = !!(filter.trim() || fieldFilter || preset || typeFilter);
  function clearFilters() {
    const url = new URL(window.location.href);
    for (const key of ["filter", "field", "preset", "type"]) url.searchParams.delete(key);
    router.history.replace(url.pathname + url.search + url.hash);
  }
  // The existing Space empty state stays available while the View list loads.
  if (!active) return views.isError && !!items.data?.length
    ? <ul className="mt-4 divide-y rounded border">{items.data.map((item) => <li key={item.id}>
      <button type="button" className="block w-full p-3 text-left" onClick={() => onOpen(item)}>{item.title}</button>
    </li>)}</ul> : null;
  const filtered = (items.data ?? []).filter((item) => (!flat || item.type !== "folder") && item.title.toLocaleLowerCase().includes(filter.trim().toLocaleLowerCase()));
  const noMatches = items.isSuccess && filtered.length === 0 && hasFilters;
  const emptyResults = <div className="space-y-2"><p role="status">{t("views.empty")}</p>
    <Button variant="outline" onClick={clearFilters}>{t("views.clearFilters")}</Button></div>;
  const columns = visibleColumns(active.config, (items.data ?? []).some((item) => item.type === "project"));
  const collection = {
    items: filtered, statuses: workflow?.statuses ?? [], spaceId, parentId: parent?.id, customFields, columns,
    canCreate: !!spaceId && canCreate, pending, onOpen, onAdd: (status?: string) => setCreating({ status }), onPatch: patch, onBulkPatch: bulkPatch,
  };
  const toolbar = {
    filter, preset, typeFilter, sort, fieldFilter, customFields, itemTypes: itemTypes.data?.filter((def) => !def.space_id || def.space_id === spaceId), setSearch: (key: string, value: string) => setSearch(key, value),
    group: group ?? (swimlanes ? "assignee" : "none"),
  };
  return <section className="min-w-0 space-y-3">
    <ViewTabs views={available} selected={active} canEdit={canEdit}
      project={parent?.type === "project"} scope={parent ? { scope_item_id: parent.id } : { scope_space_id: spaceId }}
      onSelect={(id) => setSearch("view", id, true)} onRefresh={async () => { await qc.invalidateQueries({ queryKey: viewKey }); }} />
    {parent && (parent.type === "project" || parent.type === "folder") && active.type === "table" && <>
      <button type="button" className="mt-2 text-sm underline" onClick={() => setExportingTable(true)}>{t("excel.exportTable")}</button>
      {exportingTable && <ProjectExcelExport item={parent} open={exportingTable} onOpenChange={setExportingTable}
        table={{ search: filter, preset, sort, columns: visibleColumns(active.config, false), typeFilter, fieldFilter, viewId: active.id }} />}
    </>}
    {active.type === "timelog" && parent ? <Timelog project={parent.id} /> : active.type === "files" && parent ? <ProjectFiles project={parent} onOpen={onOpen} /> : active.type === "chart" && parent ? <ProjectChart project={parent} /> : isGantt ? <>
      <ViewToolbar filterOnly {...toolbar} columns={!!spaceId && <ColumnPicker fieldsOnly key={active.id + visibleColumns(active.config, false).join()} selected={visibleColumns(active.config, false)}
        fields={customFields} canSave={canEdit} saving={savingColumns} onSave={(next) => void saveColumns(next)}
          spaceId={spaceId} spaceName={spaceInfo?.name} canManage={canManageFields} />} />
      {items.isPending && <p role="status">{t("items.loading")}</p>}
      {items.isError && <p role="alert">{t("items.loadError")}</p>}
      {items.isSuccess && <Suspense fallback={<p role="status">{t("views.gantt.loading")}</p>}>
        <ItemGantt key={active?.id} onTimelineCreate={collection.canCreate ? (start, finish) => setCreating({ day: start, finishDay: finish }) : undefined} onRowAdd={(kind, item, previousId) => {
          if (kind === "child") { setCreating({ parent: item }); return; }
          const parentId = item.locations.find((loc) => loc.space_id === spaceId)?.parent_item_id ?? null;
          const parentItem = parentId ? (items.data ?? []).find((entry) => entry.id === parentId) : undefined;
          setCreating({ parent: parentItem, day: item.start_date ?? item.due_date ?? undefined, locationParent: parentId, placement: { parentId, after: kind === "below" ? item.id : previousId } });
        }} initialOpen={!expansion.initial.gantt_collapsed} onOpenChange={(open) => expansion.save({ rows: [], gantt_collapsed: !open })} items={filtered} spaceId={spaceId} parentId={parent?.id} readonly={!canEdit} onOpen={onOpen} onAdd={!!spaceId && canCreate ? () => setCreating({}) : undefined}
          customFields={customFields} columns={visibleColumns(active.config, false)}
          onPatch={(item, update) => patch(item, update)} />
      </Suspense>}
    </> : isCalendar ? <>
      <ViewToolbar filterOnly {...toolbar} />
      {items.isPending && <p role="status">{t("items.loading")}</p>}
      {items.isError && <p role="alert">{t("items.loadError")}</p>}
      {noMatches && !spaceEmpty ? emptyResults : items.isSuccess && <Suspense fallback={<p role="status">{t("items.loading")}</p>}>
        <ItemCalendar items={filtered} readonly={!canEdit} onOpen={onOpen} onPatch={(item, update) => patch(item, update)} onCreateOn={collection.canCreate ? (day) => setCreating({ day }) : undefined} />
      </Suspense>}
    </> : !isCollection ? <div className="space-y-1 rounded border p-4">
      <h2 className="font-semibold">{t(`views.types.${active.type}`)}</h2>
      <p className="text-sm text-muted-foreground">{t("views.placeholder")}</p>
    </div> : <>
      <ViewToolbar {...toolbar} groups={isBoard ? boardGroups : tableGroups}
        columns={!!spaceId && <ColumnPicker board={isBoard} key={active.id + columns.join()} selected={columns} fields={customFields} canSave={canEdit} saving={savingColumns} onSave={(next) => void saveColumns(next)}
          spaceId={spaceId} spaceName={spaceInfo?.name} canManage={canManageFields} />} />
      {blocked && <div role="alert" className="space-y-1 rounded border border-destructive p-3 text-sm">
        <p className="font-medium">{t("views.blocked.title", { title: blocked.title })}</p>
        <ul className="list-disc pl-5">{blocked.problems.map((problem) => <li key={problem}>{problem}</li>)}</ul>
        <Button size="sm" variant="outline" onClick={() => setBlocked(undefined)}>{t("views.blocked.dismiss")}</Button>
      </div>}
      {items.isPending && <p role="status">{t("items.loading")}</p>}
      {items.isError && <p role="alert">{t("items.loadError")}</p>}
      {items.isSuccess && (!spaceEmpty && !noMatches && items.data.length === 0 ? <EmptyState icon={<ProjectIllustration />} title={t("views.noItems.title")}
          description={t("views.noItems.description")} canCreate={canCreate}
          primaryAction={{ label: t("items.new"), onClick: () => setCreating({}) }} />
        : active.type === "board" ? (!spaceEmpty && noMatches ? emptyResults : <ItemBoard {...collection} swimlanes={swimlanes} onQuickCreate={quickCreate} />)
        : <ItemTable key={active?.id} {...collection} onAddChild={(item) => setCreating({ parent: item })}initialExpanded={expansion.initial.rows} onExpandedChange={(rows) => expansion.save({ rows, gantt_collapsed: expansion.initial.gantt_collapsed })} items={items.data ?? []} filter={filter} noResults={emptyResults} showEmpty={!spaceEmpty} group={group} manual={!!spaceId && sort === "manual" && !group && !filter && !fieldFilter && !preset} onReorder={reorder} />)}
    </>}
    {me && creating && <CreateItemDialog open onOpenChange={(open) => { if (!open) setCreating(undefined); }}
      initialKind="task" taskOnly={creating.parent?.type === "task"} initialStatus={creating.status} initialDate={creating.day} initialFinish={creating.finishDay} location={{ space_id: spaceId, parent_item_id: creating.locationParent !== undefined ? (creating.locationParent ?? undefined) : (creating.parent?.id ?? parent?.id) }} me={me} onCreated={(item) => { noticeIfHidden(item); if (creating?.placement) void placeNextTo(item, creating.placement); onOpen(item); }} />}
  </section>;
}
