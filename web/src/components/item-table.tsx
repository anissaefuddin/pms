import { useMemo, useRef, useState, type KeyboardEvent } from "react";
import { useQueries } from "@tanstack/react-query";
import { canAddSubitem } from "@/lib/create-item-validate";
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { ChevronDown, ChevronRight, MoreHorizontal, Pencil, Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { formatDate, useProfile } from "@/lib/profile";
import { DatePicker, calendarDate, dateValue } from "@/components/ui/date-picker";
import { itemKeys, listItems, type Item } from "@/lib/items";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { ItemUserPicker, type PersonOption } from "@/components/item-user-picker";
import { ProjectProgress } from "@/components/project-progress";
import { CustomFieldDisplay, CustomFieldEditor } from "@/components/custom-field-editor";
import { AvatarStack } from "@/components/ui/avatar-stack";
import { tintOf } from "@/components/ui/status-chip";
import { TypeBadge, TypeGlyph } from "@/components/type-badge";
import { fieldColumn } from "@/lib/view-columns";
import { useSearchPages } from "@/lib/search";
import type { CustomField, CustomFieldValue } from "@/lib/custom-fields";
import type { ItemCollectionProps } from "./item-collection-types";

type PathStep = { id: string; title: string; type: string };
type TableRow = { item: Item; depth: number; parent?: string; groupLabel?: string; path?: PathStep[] };

export function ItemTable({ initialExpanded, onExpandedChange, items, statuses, spaceId, parentId, customFields, columns: shown, canCreate, pending, onOpen, onAdd, onAddChild, onPatch, onBulkPatch, group, manual, onReorder, showEmpty = true, filter = "", noResults }: ItemCollectionProps & {
  showEmpty?: boolean;
  /** B2.3: row menu → Add subitem. The caller opens the create dialog with this row as parent. */
  onAddChild?: (item: Item) => void;
  initialExpanded?: string[]; // WA-D05: rows expanded when the view opens
  onExpandedChange?: (rows: string[]) => void;
  filter?: string; // RT-37: title filter applied to the whole tree, so loaded nested rows match too
  noResults?: React.ReactNode;
  group?: "status" | "type";
  manual: boolean;
  onReorder: (item: Item, parent: string | undefined, after: string | null) => Promise<void>;
}) {
  const { t } = useTranslation();
  const profile = useProfile();
  // WA-D05: the caller restores expanded rows from the server profile and receives every change to save them.
  const [expanded, setExpandedState] = useState<string[]>(() => initialExpanded ?? []);
  const setExpanded = (update: (open: string[]) => string[]) => {
    const next = update(expanded);
    setExpandedState(next);
    onExpandedChange?.(next);
  };
  const [selected, setSelected] = useState(0);
  // B-02: rows picked for a bulk action. Only loaded rows the user may change can be picked; the server still checks every update.
  const [picked, setPicked] = useState<string[]>([]);
  const [bulkResult, setBulkResult] = useState<{ done: number; failed: { title: string; message: string }[] }>();
  const canBulk = (item: Item) => item.my_capabilities.includes("change_status") || item.my_capabilities.includes("edit");
  const [editing, setEditing] = useState<{ id: string; field: "title" | "assignees" | `field:${string}` }>();
  const dragging = useRef<string | undefined>(undefined);
  const viewport = useRef<HTMLDivElement>(null);
  const children = useQueries({
    queries: expanded.map((id) => ({
      queryKey: itemKeys.children(id),
      queryFn: () => listItems(`/api/v1/items/${id}/children`),
    })),
    combine: (results) => results.map((result) => result.data),
  });
  const needle = filter.trim();
  const remote = useSearchPages(needle, { spaceId: spaceId || undefined, containerId: parentId }, !!spaceId);
  const hits = useMemo(() => remote.data?.pages.flatMap((page) => page.items as (Item & { path?: PathStep[] })[]), [remote.data]);
  const rows = useMemo(() => {
    const branches = new Map(expanded.map((id, index) => [id, children[index] ?? []]));
    const roots = hits ? hits : group ? [...items].sort((a, b) => (group === "status" ? a.status?.name ?? "" : a.type)
      .localeCompare(group === "status" ? b.status?.name ?? "" : b.type)) : items;
    const output: TableRow[] = [];
    function visit(list: Item[], depth: number, parent?: string, ancestors = new Set<string>()) {
      let lastGroup: string | undefined;
      for (const item of list) {
        if (ancestors.has(item.id)) continue;
        const label = depth !== 0 ? undefined : group === "status" ? item.status?.name ?? t("views.noStatus")
          : group === "type" ? (item.item_type && item.item_type.key !== item.type ? item.item_type.name : t(`items.${item.type}`)) : undefined;
        output.push({ item, depth, parent, groupLabel: label !== lastGroup ? label : undefined, path: depth === 0 ? (item as Item & { path?: PathStep[] }).path : undefined });
        lastGroup = label;
        const branch = branches.get(item.id);
        if (branch) visit(branch.filter((child) => child.locations.some((loc) => loc.space_id === spaceId && loc.parent_item_id === item.id)),
          depth + 1, item.id, new Set([...ancestors, item.id]));
      }
    }
    visit(roots, 0, parentId);
    if (hits || !needle) return output; // server hits already match; until they arrive the loaded rows are filtered locally
    const lowered = needle.toLocaleLowerCase();
    // keep a row when it matches or when any row below it (deeper, until the next row at its level) matches
    return output.filter((row, index) => {
      if (row.item.title.toLocaleLowerCase().includes(lowered)) return true;
      for (let next = index + 1; next < output.length && output[next].depth > row.depth; next++) {
        if (output[next].item.title.toLocaleLowerCase().includes(lowered)) return true;
      }
      return false;
    });
  }, [items, expanded, children, group, spaceId, parentId, t, needle, hits]);

  const allColumns: ColumnDef<TableRow>[] = [
    { id: "name", header: t("items.name"), cell: ({ row }) => {
      const { item, depth, groupLabel, path } = row.original;
      const isExpanded = expanded.includes(item.id);
      const expandable = item.type !== "task" || !!item.child_count;
      const canEditTitle = item.my_capabilities.includes("edit") && editing?.id !== item.id;
      return <div className="flex min-w-0 items-center gap-1" style={{ paddingInlineStart: depth * 14 }}>
        {groupLabel && <span className="shrink-0 rounded bg-secondary px-1 text-xs">{groupLabel}</span>}
        {expandable && <Button size="icon-sm" variant="ghost" aria-expanded={isExpanded}
          aria-label={t(isExpanded ? "views.collapse" : "views.expand", { name: item.title })}
          onClick={() => setExpanded((open) => (open.includes(item.id) ? open.filter((id) => id !== item.id) : [...open, item.id]))}>
          {isExpanded ? <ChevronDown /> : <ChevronRight />}
        </Button>}
        <TypeGlyph item={item} className="size-4 text-muted-foreground" />
        {!!path?.length && <span className="max-w-48 shrink-0 truncate text-xs text-muted-foreground" title={path.map((step) => step.title).join(" › ")}>{path.map((step) => step.title).join(" › ")}</span>}
        {editing?.id === item.id && editing.field === "title"
          ? <form className="min-w-0 flex-1" onSubmit={(event) => {
            event.preventDefault();
            const title = String(new FormData(event.currentTarget).get("title") ?? "").trim();
            if (title && title !== item.title) void onPatch(item, { title });
            setEditing(undefined);
          }}>
            <Input name="title" aria-label={t("items.name")} defaultValue={item.title} maxLength={500} required autoFocus
              onKeyDown={(event) => { if (event.key === "Escape") setEditing(undefined); }} />
          </form>
          : <Button variant="ghost" size="sm" className="min-w-0 justify-start" title={item.title}
              onClick={() => onOpen(item)}><span className="truncate">{item.title}</span></Button>}
        {canEditTitle && <Button size="icon-sm" variant="ghost" className="opacity-0 transition-opacity group-hover/table-row:opacity-100 group-focus-within/table-row:opacity-100 [@media(hover:none)]:opacity-100" aria-label={t("views.editTitle", { name: item.title })}
          onClick={() => setEditing({ id: item.id, field: "title" })}><Pencil /></Button>}
        {!!item.child_count && <span className="shrink-0 text-xs text-muted-foreground">{t("items.childCount", { count: item.child_count })}</span>}
      </div>;
    } },
    { id: "progress", header: t("items.progress.column"), cell: ({ row }) => (
      <span className="text-sm"><ProjectProgress item={row.original.item} /></span>
    ) },
    { id: "type", header: t("views.columns.type"), cell: ({ row }) => <TypeBadge item={row.original.item} /> },
    { id: "status", header: t("items.status"), cell: ({ row }) => {
      const { item } = row.original;
      const change = item.my_capabilities.includes("change_status");
      return <select data-field="status" aria-label={`${t("items.status")}: ${item.title}`}
        className={`w-full truncate rounded-full border-0 px-2 py-0.5 text-xs font-medium ${tintOf(item.status?.color)}`} value={item.status?.id ?? ""} disabled={!change || pending}
        onChange={(event) => void onPatch(item, { status_id: event.target.value })}>
        {!item.status && <option value="" disabled>{t("views.noStatus")}</option>}
        {item.status && !statuses.some((status) => status.id === item.status?.id) && <option value={item.status.id}>{item.status.name}</option>}
        {statuses.map((status) => <option key={status.id} value={status.id}>{status.name}</option>)}
      </select>;
    } },
    { id: "assignee", header: t("items.assignees"), cell: ({ row }) => {
      const { item } = row.original;
      const canEdit = item.my_capabilities.includes("edit");
      return canEdit ? <Button variant="ghost" size="sm" className="max-w-full truncate"
        onClick={() => setEditing({ id: item.id, field: "assignees" })}>
        <span className="sr-only">{`${t("items.assignees")}: ${item.title}`}</span>
        {item.assignees.length ? <AvatarStack people={item.assignees} /> : t("views.unassigned")}
      </Button> : <AvatarStack people={item.assignees} />;
    } },
    ...(["start_date", "due_date"] as const).map((field): ColumnDef<TableRow> => ({
      id: field, header: t(`views.${field}`), cell: ({ row }) => {
        const { item } = row.original;
        const canEdit = item.my_capabilities.includes("edit");
        return canEdit ? <DatePicker value={calendarDate(item[field])} label={`${t(`views.${field}`)}: ${item.title}`}
          workingDays disabled={pending} onChange={(date) => {
            const value = dateValue(date) || null;
            if (value !== (item[field] ?? null)) void onPatch(item, { [field]: value });
          }} /> : <span>{formatDate(item[field], profile)}</span>;
      },
    })),
    ...customFields.filter((field) => shown.includes(fieldColumn(field.id))).map((field): ColumnDef<TableRow> => ({
      id: fieldColumn(field.id), header: field.name, cell: ({ row }) => {
        const { item } = row.original;
        const value = item.fields?.[field.id];
        const display = profile ? <CustomFieldDisplay field={field} value={value} people={peopleOf(item, field, value)} profile={profile} /> : null;
        return item.my_capabilities.includes("edit") && !field.archived_at && field.type !== "formula" && profile
          ? <Button variant="ghost" size="sm" className="max-w-full justify-start truncate" aria-label={`${field.name}: ${item.title}`}
            onClick={() => setEditing({ id: item.id, field: fieldColumn(field.id) as `field:${string}` })}>{display}</Button>
          : display;
      },
    })),
    { id: "actions", header: t("views.actions"), cell: ({ row }) => {
      const { item, parent } = row.original;
      // B2.3: "Add subitem" needs create permission on this row; ordering needs manual sort and edit permission.
      const movable = manual && item.my_capabilities.includes("edit") && inLocation(item, parent);
      // Same rule as the Item Detail panel (canAddSubitem); the dialog then offers only the child types the parent allows.
      const addable = !!onAddChild && canCreate && canAddSubitem(item.my_capabilities);
      if (!movable && !addable) return null;
      const siblings = movable ? rows.filter((entry) => entry.parent === parent && entry.item.id !== item.id && inLocation(entry.item, parent)) : [];
      return <DropdownMenu>
        <DropdownMenuTrigger render={<Button size="icon-sm" variant="ghost" aria-label={movable ? t("views.move", { name: item.title }) : t("views.actions")} />}><MoreHorizontal /></DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="max-h-64 overflow-y-auto">
          {addable && <DropdownMenuItem onClick={() => onAddChild?.(item)}>{t("items.addChild")}</DropdownMenuItem>}
          {movable && <DropdownMenuItem disabled={pending} onClick={() => void onReorder(item, parent, null)}>{t("views.moveTop")}</DropdownMenuItem>}
          {movable && siblings.map((sibling) => <DropdownMenuItem disabled={pending} key={sibling.item.id} onClick={() => void onReorder(item, parent, sibling.item.id)}>
            {t("views.moveAfter", { name: sibling.item.title })}
          </DropdownMenuItem>)}
        </DropdownMenuContent>
      </DropdownMenu>;
    } },
  ];
  // A column with nothing to show for any row (no Project progress, nobody assigned) is left out; the Columns picker can still add it back.
  const empty = new Set<string>([
    ...(rows.some((row) => row.item.type === "project" && (row.item.progress_percent ?? 0) > 0) ? [] : ["progress"]),
    ...(rows.some((row) => row.item.assignees.length > 0) ? [] : ["assignee"]),
  ]);
  const columns = allColumns.filter((column) => column.id === "name" || column.id === "actions" || (shown.includes(column.id ?? "") && !empty.has(column.id ?? "")));
  const widths: Record<string, [string, number]> = { name: ["minmax(15rem,2fr)", 15], progress: ["8.5rem", 8.5], start_date: ["8rem", 8], due_date: ["8rem", 8], actions: ["2.5rem", 2.5] };
  const sized = [["2.5rem", 2.5] as [string, number], ...columns.map((column) => widths[column.id ?? ""] ?? ["minmax(8rem,1fr)", 8])];
  const grid = "grid";
  // The selection column (2.5rem) and the Name column stay in view while the table scrolls sideways.
  const stickyName = "sticky left-10 shadow-[2px_0_0_hsl(var(--border))]";
  const minWidth = `${sized.reduce((total, [, rem]) => total + rem, 0)}rem`;
  const gridStyle = { gridTemplateColumns: sized.map(([width]) => width).join(" "), minWidth };
  const table = useReactTable({ data: rows, columns, getCoreRowModel: getCoreRowModel(), autoResetAll: false });
  const model = table.getRowModel().rows;
  const virtual = useVirtualizer({ count: model.length, getScrollElement: () => viewport.current, estimateSize: () => 44, overscan: 8 });
  const editedItem = editing?.field === "assignees" ? rows.find((row) => row.item.id === editing.id)?.item : undefined;
  const editedField = editing?.field.startsWith("field:") ? customFields.find((field) => fieldColumn(field.id) === editing.field) : undefined;
  const editedFieldItem = editedField ? rows.find((row) => row.item.id === editing?.id)?.item : undefined;
  function inLocation(item: Item, parent?: string) {
    return item.locations.some((loc) => loc.space_id === spaceId && (loc.parent_item_id ?? undefined) === parent);
  }

  function keyboard(event: KeyboardEvent<HTMLDivElement>) {
    if ((event.target as HTMLElement).closest("input, select, button, [contenteditable]")) return;
    const index = Math.min(selected, rows.length - 1);
    const current = rows[index]?.item;
    if (!current) return;
    const key = event.key.toLowerCase();
    if (key === "j" || key === "k" || key === "arrowdown" || key === "arrowup" || key === "home" || key === "end") {
      event.preventDefault();
      const next = key === "home" ? 0 : key === "end" ? rows.length - 1
        : Math.min(rows.length - 1, Math.max(0, index + (key === "j" || key === "arrowdown" ? 1 : -1)));
      setSelected(next);
      virtual.scrollToIndex(next);
    } else if (key === "enter") {
      event.preventDefault();
      onOpen(current);
    } else if (key === "s") {
      viewport.current?.querySelector<HTMLSelectElement>(`[data-index="${index}"] [data-field="status"]`)?.focus();
    } else if (key === " " && canBulk(current)) {
      event.preventDefault();
      togglePicked(current.id);
    } else if (key === "arrowright" && (current.type !== "task" || !!current.child_count) && !expanded.includes(current.id)) {
      event.preventDefault();
      setExpanded((open) => [...open, current.id]);
    } else if (key === "arrowleft" && expanded.includes(current.id)) {
      event.preventDefault();
      setExpanded((open) => open.filter((id) => id !== current.id));
    } else if ((key === "e" || key === "a") && current.my_capabilities.includes("edit")) {
      setEditing({ id: current.id, field: key === "e" ? "title" : "assignees" });
    }
  }

  const pickable = model.map((row) => row.original.item).filter(canBulk);
  const pickedItems = pickable.filter((item) => picked.includes(item.id));
  const allPicked = pickable.length > 0 && pickedItems.length === pickable.length;
  const somePicked = pickedItems.length > 0 && !allPicked;
  function togglePicked(id: string) {
    setPicked((current) => (current.includes(id) ? current.filter((entry) => entry !== id) : [...current, id]));
  }
  async function applyStatus(statusId: string) {
    if (!onBulkPatch || !pickedItems.length) return;
    setBulkResult(await onBulkPatch(pickedItems, { status_id: statusId }));
    setPicked([]);
  }

  return <div className="space-y-2">
    {(picked.length > 0 || bulkResult) && <div role="toolbar" aria-label={t("views.bulk.toolbar")} className="flex flex-wrap items-center gap-2 rounded border bg-muted/40 p-2 text-sm">
      {picked.length > 0 && <>
        <span className="font-medium">{t("views.bulk.selected", { count: pickedItems.length })}</span>
        {onBulkPatch && <label className="flex items-center gap-1">
          <span>{t("views.bulk.setStatus")}</span>
          <select aria-label={t("views.bulk.setStatus")} className="rounded border bg-background p-1" value="" disabled={pending} onChange={(event) => { if (event.target.value) void applyStatus(event.target.value); }}>
            <option value="" disabled>—</option>
            {statuses.map((status) => <option key={status.id} value={status.id}>{status.name}</option>)}
          </select>
        </label>}
        <Button size="sm" variant="ghost" onClick={() => setPicked([])}>{t("views.bulk.clear")}</Button>
      </>}
      {bulkResult && <p role="status" className="ml-auto">
        {t("views.bulk.result", { done: bulkResult.done, failed: bulkResult.failed.length })}
        {bulkResult.failed.length > 0 && <span className="block text-xs text-muted-foreground">{bulkResult.failed.map((entry) => `${entry.title}: ${entry.message}`).join(" · ")}</span>}
      </p>}
    </div>}
    <div ref={viewport} role="table" aria-label={t("items.table")} aria-rowcount={model.length + 1} tabIndex={0}
      onKeyDown={keyboard} className="max-h-[60vh] min-h-28 overflow-auto rounded border focus-visible:outline-ring">
      <div role="rowgroup" className="sticky top-0 z-10 bg-card">
        {table.getHeaderGroups().map((header) => <div role="row" aria-rowindex={1} key={header.id} className={grid} style={gridStyle}>
          <div role="columnheader" className="sticky left-0 z-30 flex items-center border-b bg-card px-2 py-2">
            <input type="checkbox" aria-label={t("views.bulk.selectAll")} disabled={!pickable.length} checked={allPicked}
              ref={(element) => { if (element) element.indeterminate = somePicked; }}
              onChange={() => setPicked(allPicked ? [] : pickable.map((item) => item.id))} />
          </div>
          {header.headers.map((cell) => <div role="columnheader" key={cell.id} className={`border-b bg-card px-2 py-2 font-medium ${cell.column.id === "name" ? `${stickyName} z-30` : cell.column.id === "actions" ? "sticky right-0 z-30 shadow-[-2px_0_0_hsl(var(--border))]" : ""}`}>
            {flexRender(cell.column.columnDef.header, cell.getContext())}
          </div>)}
        </div>)}
      </div>
      <div role="rowgroup" className="relative" style={{ height: virtual.getTotalSize(), minWidth }}>
        {virtual.getVirtualItems().map((entry) => {
          const row = model[entry.index];
          const { item, parent } = row.original;
          const canDrag = manual && !pending && inLocation(item, parent) && item.my_capabilities.includes("edit");
          const parentRow = item.type === "project" || item.type === "folder" || !!item.child_count;
          return <div key={row.id} role="row" aria-rowindex={entry.index + 2} aria-current={selected === entry.index ? "true" : undefined} data-index={entry.index}
            className={`${grid} group/table-row absolute inset-x-0 border-b ${parentRow ? "font-medium" : ""} ${selected === entry.index ? "bg-muted/60" : ""}`}
            style={{ ...gridStyle, height: entry.size, transform: `translateY(${entry.start}px)` }}
            draggable={canDrag} onMouseDown={() => setSelected(entry.index)}
            onDragStart={(event) => { event.dataTransfer.setData("text/plain", item.id); dragging.current = item.id; }}
            onDragEnd={() => { dragging.current = undefined; }}
            onDragOver={(event) => { if (dragging.current && manual && !pending) event.preventDefault(); }}
            onDrop={(event) => {
              event.preventDefault();
              const source = rows.find((entry) => entry.item.id === dragging.current && entry.parent === parent);
              if (source && source.item.id !== item.id && inLocation(item, parent)) void onReorder(source.item, parent, item.id);
              dragging.current = undefined;
            }}>
            <div role="cell" className="sticky left-0 z-20 flex items-center bg-card px-2">
              {canBulk(item) && <input type="checkbox" aria-label={t("views.bulk.selectRow", { title: item.title })} checked={picked.includes(item.id)} onChange={() => togglePicked(item.id)} onMouseDown={(event) => event.stopPropagation()} />}
            </div>
            {row.getVisibleCells().map((cell) => <div role="cell" key={cell.id} className={`flex min-w-0 items-center px-2 ${cell.column.id === "name" ? `${stickyName} z-20 bg-card` : cell.column.id === "actions" ? "sticky right-0 z-20 bg-card shadow-[-2px_0_0_hsl(var(--border))]" : ""}`}>
              {/* called, not rendered as a component: a new component type per render would remount the cell between mousedown and mouseup and swallow the click (QA-D-lane3-010) */}
              {(cell.column.columnDef.cell as (context: ReturnType<typeof cell.getContext>) => React.ReactNode)(cell.getContext())}
            </div>)}
          </div>;
        })}
      </div>
    </div>
    {showEmpty && model.length === 0 && (filter.trim() && noResults ? noResults : <p className="text-muted-foreground">{t("views.empty")}</p>)}
    {remote.hasNextPage && <Button variant="outline" size="sm" disabled={remote.isFetchingNextPage} onClick={() => void remote.fetchNextPage()}>{t("views.loadMore")}</Button>}
    {canCreate && <Button variant="ghost" size="sm" className="sticky bottom-0 z-10 w-full justify-start border-t bg-background text-primary" onClick={() => onAdd()}><Plus />{t("views.add")}</Button>}
    {editedField && editedFieldItem && profile && <FieldEdit key={editedFieldItem.id + editedField.id} field={editedField} item={editedFieldItem}
      locale={profile.locale} pending={pending} onCancel={() => setEditing(undefined)}
      onSave={async (value) => { await onPatch(editedFieldItem, { fields: { [editedField.id]: value } }); setEditing(undefined); }} />}
    {editedItem && <AssigneeEdit key={editedItem.id} item={editedItem} onCancel={() => setEditing(undefined)} onSave={(people) => {
      void onPatch(editedItem, { assignee_ids: people.map((person) => person.id) });
      setEditing(undefined);
    }} />}
  </div>;
}

export function peopleOf(item: Item, field: CustomField, value: unknown) {
  return field.type === "people" && Array.isArray(value) ? value.map((id: string) => item.field_people?.[id] ?? { id, name: id }) : [];
}

function FieldEdit({ field, item, locale, pending, onSave, onCancel }: {
  field: CustomField; item: Item; locale: string; pending: boolean; onSave: (value: CustomFieldValue) => Promise<void>; onCancel: () => void;
}) {
  const value = item.fields?.[field.id];
  return <Dialog open onOpenChange={(open) => { if (!open) onCancel(); }}>
    <DialogContent>
      <DialogTitle>{field.name}: {item.title}</DialogTitle>
      <CustomFieldEditor field={field} value={value} people={peopleOf(item, field, value)} locale={locale}
        disabled={pending} onCancel={onCancel} onSave={onSave} />
    </DialogContent>
  </Dialog>;
}

function AssigneeEdit({ item, onSave, onCancel }: { item: Item; onSave: (people: PersonOption[]) => void; onCancel: () => void }) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState(item.assignees);
  return <Dialog open onOpenChange={(open) => { if (!open) onCancel(); }}>
    <DialogContent>
      <DialogTitle>{t("items.assignees")}</DialogTitle>
      <ItemUserPicker label={t("items.assignees")} multiple selected={selected} onChange={setSelected} />
      <div className="flex gap-2">
        <Button size="sm" onClick={() => onSave(selected)}>{t("items.save")}</Button>
        <Button size="sm" variant="ghost" onClick={onCancel}>{t("items.cancel")}</Button>
      </div>
    </DialogContent>
  </Dialog>;
}
