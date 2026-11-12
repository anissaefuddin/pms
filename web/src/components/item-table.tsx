import { useMemo, useRef, useState, type KeyboardEvent } from "react";
import { useQueries } from "@tanstack/react-query";
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
import { fieldColumn } from "@/lib/view-columns";
import type { CustomField, CustomFieldValue } from "@/lib/custom-fields";
import type { ItemCollectionProps } from "./item-collection-types";

type TableRow = { item: Item; depth: number; parent?: string; groupLabel?: string };

export function ItemTable({ items, statuses, spaceId, parentId, customFields, columns: shown, canCreate, pending, onOpen, onAdd, onPatch, group, manual, onReorder }: ItemCollectionProps & {
  group?: "status" | "type";
  manual: boolean;
  onReorder: (item: Item, parent: string | undefined, after: string | null) => Promise<void>;
}) {
  const { t } = useTranslation();
  const profile = useProfile();
  const [expanded, setExpanded] = useState<string[]>([]);
  const [selected, setSelected] = useState(0);
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
  const rows = useMemo(() => {
    const branches = new Map(expanded.map((id, index) => [id, children[index] ?? []]));
    const roots = group ? [...items].sort((a, b) => (group === "status" ? a.status?.name ?? "" : a.type)
      .localeCompare(group === "status" ? b.status?.name ?? "" : b.type)) : items;
    const output: TableRow[] = [];
    function visit(list: Item[], depth: number, parent?: string, ancestors = new Set<string>()) {
      let lastGroup: string | undefined;
      for (const item of list) {
        if (ancestors.has(item.id)) continue;
        const label = depth !== 0 ? undefined : group === "status" ? item.status?.name ?? t("views.noStatus")
          : group === "type" ? t(`items.${item.type}`) : undefined;
        output.push({ item, depth, parent, groupLabel: label !== lastGroup ? label : undefined });
        lastGroup = label;
        const branch = branches.get(item.id);
        if (branch) visit(branch.filter((child) => child.locations.some((loc) => loc.space_id === spaceId && loc.parent_item_id === item.id)),
          depth + 1, item.id, new Set([...ancestors, item.id]));
      }
    }
    visit(roots, 0, parentId);
    return output;
  }, [items, expanded, children, group, spaceId, parentId, t]);

  const allColumns: ColumnDef<TableRow>[] = [
    { id: "name", header: t("items.name"), cell: ({ row }) => {
      const { item, depth, groupLabel } = row.original;
      const isExpanded = expanded.includes(item.id);
      const expandable = item.type !== "task" || !!item.child_count;
      const canEditTitle = item.my_capabilities.includes("edit") && editing?.id !== item.id;
      return <div className="flex min-w-0 items-center gap-1" style={{ paddingInlineStart: depth * 14 }}>
        {groupLabel && <span className="shrink-0 rounded bg-secondary px-1 text-xs">{groupLabel}</span>}
        {expandable && <Button size="icon-sm" variant="ghost" aria-expanded={isExpanded}
          aria-label={t(isExpanded ? "views.collapse" : "views.expand", { name: item.title })}
          onClick={() => setExpanded(isExpanded ? expanded.filter((id) => id !== item.id) : [...expanded, item.id])}>
          {isExpanded ? <ChevronDown /> : <ChevronRight />}
        </Button>}
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
          : <Button variant="ghost" size="sm" className="min-w-0 justify-start"
              onClick={() => onOpen(item)}><span className="truncate">{item.title}</span></Button>}
        {canEditTitle && <Button size="icon-sm" variant="ghost" aria-label={t("views.editTitle", { name: item.title })}
          onClick={() => setEditing({ id: item.id, field: "title" })}><Pencil /></Button>}
        {!!item.child_count && <span className="shrink-0 text-xs text-muted-foreground">{t("items.childCount", { count: item.child_count })}</span>}
      </div>;
    } },
    { id: "progress", header: t("items.progress.column"), cell: ({ row }) => (
      <span className="text-sm"><ProjectProgress item={row.original.item} /></span>
    ) },
    { id: "status", header: t("items.status"), cell: ({ row }) => {
      const { item } = row.original;
      const change = item.my_capabilities.includes("change_status");
      return <select data-field="status" aria-label={`${t("items.status")}: ${item.title}`}
        className="w-full truncate rounded border bg-background px-1 py-1" value={item.status?.id ?? ""} disabled={!change || pending}
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
        aria-label={`${t("items.assignees")}: ${item.title}`}
        onClick={() => setEditing({ id: item.id, field: "assignees" })}>
        {item.assignees.map((person) => person.name).join(", ") || t("items.assignees")}
      </Button> : <span className="truncate">{item.assignees.map((person) => person.name).join(", ")}</span>;
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
        return item.my_capabilities.includes("edit") && !field.archived_at && profile
          ? <Button variant="ghost" size="sm" className="max-w-full justify-start truncate" aria-label={`${field.name}: ${item.title}`}
            onClick={() => setEditing({ id: item.id, field: fieldColumn(field.id) as `field:${string}` })}>{display}</Button>
          : display;
      },
    })),
    { id: "actions", header: t("views.actions"), cell: ({ row }) => {
      const { item, parent } = row.original;
      if (!manual || !item.my_capabilities.includes("edit") || !inLocation(item, parent)) return null;
      const siblings = rows.filter((entry) => entry.parent === parent && entry.item.id !== item.id && inLocation(entry.item, parent));
      return <DropdownMenu>
        <DropdownMenuTrigger render={<Button size="icon-sm" variant="ghost" aria-label={t("views.move", { name: item.title })} />}><MoreHorizontal /></DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="max-h-64 overflow-y-auto">
          <DropdownMenuItem disabled={pending} onClick={() => void onReorder(item, parent, null)}>{t("views.moveTop")}</DropdownMenuItem>
          {siblings.map((sibling) => <DropdownMenuItem disabled={pending} key={sibling.item.id} onClick={() => void onReorder(item, parent, sibling.item.id)}>
            {t("views.moveAfter", { name: sibling.item.title })}
          </DropdownMenuItem>)}
        </DropdownMenuContent>
      </DropdownMenu>;
    } },
  ];
  const columns = allColumns.filter((column) => column.id === "name" || column.id === "actions" || shown.includes(column.id ?? ""));
  const widths: Record<string, [string, number]> = { name: ["minmax(15rem,2fr)", 15], progress: ["8.5rem", 8.5], start_date: ["8rem", 8], due_date: ["8rem", 8], actions: ["2.5rem", 2.5] };
  const sized = columns.map((column) => widths[column.id ?? ""] ?? ["minmax(8rem,1fr)", 8]);
  const grid = "grid";
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
    if (key === "j" || key === "k") {
      event.preventDefault();
      const next = Math.min(rows.length - 1, Math.max(0, index + (key === "j" ? 1 : -1)));
      setSelected(next);
      virtual.scrollToIndex(next);
    } else if (key === "enter") {
      event.preventDefault();
      onOpen(current);
    } else if (key === "s") {
      viewport.current?.querySelector<HTMLSelectElement>(`[data-index="${index}"] [data-field="status"]`)?.focus();
    } else if ((key === "e" || key === "a") && current.my_capabilities.includes("edit")) {
      setEditing({ id: current.id, field: key === "e" ? "title" : "assignees" });
    }
  }

  return <div className="space-y-2">
    <div ref={viewport} role="table" aria-label={t("items.table")} aria-rowcount={model.length + 1} tabIndex={0}
      onKeyDown={keyboard} className="max-h-[60vh] min-h-28 overflow-auto rounded border focus-visible:outline-ring">
      <div role="rowgroup" className="sticky top-0 z-10 bg-card">
        {table.getHeaderGroups().map((header) => <div role="row" aria-rowindex={1} key={header.id} className={grid} style={gridStyle}>
          {header.headers.map((cell) => <div role="columnheader" key={cell.id} className="border-b px-2 py-2 font-medium">
            {flexRender(cell.column.columnDef.header, cell.getContext())}
          </div>)}
        </div>)}
      </div>
      <div role="rowgroup" className="relative" style={{ height: virtual.getTotalSize(), minWidth }}>
        {virtual.getVirtualItems().map((entry) => {
          const row = model[entry.index];
          const { item, parent } = row.original;
          const canDrag = manual && !pending && inLocation(item, parent) && item.my_capabilities.includes("edit");
          return <div key={row.id} role="row" aria-rowindex={entry.index + 2} data-index={entry.index}
            className={`${grid} absolute inset-x-0 border-b ${selected === entry.index ? "bg-muted/60" : ""}`}
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
            {row.getVisibleCells().map((cell) => <div role="cell" key={cell.id} className="flex min-w-0 items-center px-2">
              {flexRender(cell.column.columnDef.cell, cell.getContext())}
            </div>)}
          </div>;
        })}
      </div>
    </div>
    {model.length === 0 && <p className="text-muted-foreground">{t("views.empty")}</p>}
    {canCreate && <Button variant="outline" size="sm" onClick={() => onAdd()}><Plus />{t("views.add")}</Button>}
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
