import { formatNumber, formatDate, useProfile } from "@/lib/profile";
import { useState } from "react";
import { CalendarDays, ListTree, Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { CustomFieldDisplay } from "@/components/custom-field-editor";
import { AvatarStack } from "@/components/avatar-stack";
import { StatusChip, tintOf } from "@/components/status-chip";
import { statusDotClass } from "@/lib/status-colors";
import { TypeBadge } from "@/components/type-badge";
import { fieldColumn } from "@/lib/view-columns";
import { peopleOf } from "@/components/item-table";
import type { Item, ItemUpdate } from "@/lib/items";
import type { ItemCollectionProps } from "./item-collection-types";

type Lane = { key: string; name?: string; items: Item[] };
type Move = { item: Item; from: string; to: string; update: ItemUpdate; fromName: string; toName: string };
const DAY = 86_400_000;
const noColor = "slate";

/** One lane per assignee; an Item with several assignees appears in each lane, Items without one go to "" (Unassigned). */
function lanesOf(items: Item[], swimlanes: boolean, unassigned: string): Lane[] {
  if (!swimlanes) return [{ key: "", items }];
  const people = new Map<string, string>();
  for (const item of items) for (const person of item.assignees) people.set(person.id, person.name);
  const lanes = [...people].sort((a, b) => a[1].localeCompare(b[1])).map(([key, name]): Lane => ({
    key, name, items: items.filter((item) => item.assignees.some((person) => person.id === key)),
  }));
  const rest = items.filter((item) => !item.assignees.length);
  return [...lanes, ...(rest.length || !lanes.length ? [{ key: "", name: unassigned, items: rest }] : [])];
}

export function ItemBoard({ items, statuses, customFields, columns: shown, swimlanes, canCreate, pending, onOpen, onAdd, onPatch }: ItemCollectionProps & { swimlanes: boolean }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const [dragging, setDragging] = useState<{ id: string; lane: string }>();
  const [move, setMove] = useState<Move>();
  const [now] = useState(() => Date.now());
  const cardFields = customFields.filter((field) => shown.includes(fieldColumn(field.id)));
  const known = new Set(statuses.map((status) => status.id));

  function drop(lane: Lane, columnId: string) {
    const item = items.find((entry) => entry.id === dragging?.id);
    const from = dragging?.lane ?? "";
    setDragging(undefined);
    if (!item || pending) return;
    const statusChanged = !!columnId && columnId !== item.status?.id && item.my_capabilities.includes("change_status");
    if (swimlanes && lane.key !== from) {
      if (!item.my_capabilities.includes("edit")) return;
      const ids = item.assignees.map((person) => person.id).filter((id) => id !== from);
      const update: ItemUpdate = { assignee_ids: lane.key ? [...new Set([...ids, lane.key])] : ids, ...(statusChanged ? { status_id: columnId } : {}) };
      const name = (key: string) => item.assignees.find((person) => person.id === key)?.name ?? lanes.find((entry) => entry.key === key)?.name ?? t("views.unassigned");
      setMove({ item, from, to: lane.key, update, fromName: name(from), toName: name(lane.key) });
    } else if (statusChanged) void onPatch(item, { status_id: columnId });
  }

  const lanes = lanesOf(items, swimlanes, t("views.unassigned"));
  return <div className="space-y-4">
    {lanes.map((lane) => {
      const unlisted = lane.items.filter((item) => !item.status || !known.has(item.status.id));
      const cols = [
        ...statuses.map((status) => ({ id: status.id, name: status.name, color: status.color, items: lane.items.filter((item) => item.status?.id === status.id) })),
        ...(unlisted.length ? [{ id: "", name: t("views.noStatus"), color: noColor, items: unlisted }] : []),
      ];
      const board = <div className="flex gap-3 overflow-x-auto pb-3">
        {cols.map((column) => <section key={column.id} aria-label={lane.name ? `${lane.name}: ${column.name}` : column.name}
          className="w-64 shrink-0 space-y-2 rounded border bg-muted/30 p-3"
          onDragOver={(event) => { if (dragging && !pending && (column.id || swimlanes)) event.preventDefault(); }}
          onDrop={(event) => { event.preventDefault(); drop(lane, column.id); }}>
          <h3 className={`-mx-1 flex items-center gap-2 rounded px-2 py-1 font-semibold ${tintOf(column.color)}`}>
            <span aria-hidden className={`size-2 rounded-full ${statusDotClass(column.color)}`} />{column.name} <span className="font-normal">{formatNumber(column.items.length, profile)}</span></h3>
          <ul className="space-y-2">
            {column.items.map((item) => <Card key={item.id} item={item} lane={lane.key} now={now} fields={cardFields} shown={shown}
              statuses={statuses} pending={pending} onOpen={onOpen} onPatch={onPatch} onDrag={setDragging} />)}
          </ul>
          {canCreate && column.id && <Button size="sm" variant="outline" onClick={() => onAdd(column.id)}><Plus />{t("views.add")}</Button>}
        </section>)}
      </div>;
      return swimlanes
        ? <section key={lane.key} aria-label={lane.name} className="space-y-2">
          <h2 className="font-semibold">{lane.name} <span className="text-muted-foreground">{formatNumber(lane.items.length, profile)}</span></h2>
          {board}
        </section>
        : <div key="all">{board}</div>;
    })}
    {move && <ConfirmDialog open onOpenChange={(open) => { if (!open) setMove(undefined); }} title={t("views.reassign.title")}
      description={t("views.reassign.body", { title: move.item.title, from: move.fromName, to: move.toName })}
      confirmLabel={t("views.reassign.confirm")} cancelLabel={t("items.cancel")}
      onConfirm={() => { void onPatch(move.item, move.update); setMove(undefined); }} />}
  </div>;
}

function Card({ item, lane, now, fields, shown, statuses, pending, onOpen, onPatch, onDrag }: {
  item: Item; lane: string; now: number; fields: ItemCollectionProps["customFields"]; shown: string[];
  statuses: ItemCollectionProps["statuses"]; pending: boolean;
  onOpen: (item: Item) => void; onPatch: ItemCollectionProps["onPatch"]; onDrag: (value?: { id: string; lane: string }) => void;
}) {
  const { t } = useTranslation();
  const profile = useProfile();
  const canMove = item.my_capabilities.includes("change_status");
  const draggable = (canMove || item.my_capabilities.includes("edit")) && !pending;
  const meta = [
    shown.includes("id") && `#${item.seq}`,
    shown.includes("days_in_status") && item.status_changed_at && t("views.card.daysInStatus", { count: Math.max(0, Math.floor((now - Date.parse(item.status_changed_at)) / DAY)) }),
    shown.includes("author") && item.created_by && t("views.card.author", { name: item.created_by.name }),
    shown.includes("locations") && item.locations.map((loc) => loc.space_name).filter(Boolean).join(", "),
  ].filter(Boolean);
  return <li className="space-y-2 rounded border bg-card p-2 shadow-xs"
    draggable={draggable} onDragStart={(event) => { event.dataTransfer.setData("text/plain", item.id); onDrag({ id: item.id, lane }); }}
    onDragEnd={() => onDrag(undefined)}>
    {shown.includes("cover") && item.cover_attachment_id && <img alt="" loading="lazy" className="h-24 w-full rounded object-cover"
      src={`/api/v1/attachments/${item.cover_attachment_id}/download`} />}
    <Button variant="ghost" size="sm" className="h-auto max-w-full justify-start whitespace-normal text-left" onClick={() => onOpen(item)}>{item.title}</Button>
    {(!!meta.length || !!item.item_type) && <p className="flex flex-wrap items-center gap-x-1 text-xs text-muted-foreground">
      {<TypeBadge item={item} />}{!!meta.length && <span>{meta.join(" · ")}</span>}</p>}
    {(item.start_date || item.due_date) && <p className="flex items-center gap-1 text-xs text-muted-foreground">
      <CalendarDays className="size-3" />{[item.start_date, item.due_date].filter(Boolean).map((date) => formatDate(date, profile)).join(" – ")}
      {item.start_date && item.due_date && <span>({t("views.card.duration", { count: Math.max(1, Math.round((Date.parse(item.due_date) - Date.parse(item.start_date)) / DAY) + 1) })})</span>}
    </p>}
    {profile && fields.map((field) => <p key={field.id} className="flex flex-wrap gap-1 text-xs">
      <span className="text-muted-foreground">{field.name}:</span>
      <CustomFieldDisplay field={field} value={item.fields?.[field.id]} people={peopleOf(item, field, item.fields?.[field.id])} profile={profile} />
    </p>)}
    <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
      <span className="flex flex-wrap items-center gap-2">
        {!!item.child_count && <span title={t("items.childCount", { count: item.child_count })} className="inline-flex items-center gap-1"><ListTree className="size-3" aria-hidden />{item.child_count}<span className="sr-only"> {t("items.childCount", { count: item.child_count })}</span></span>}
        {!!item.attachment_count && <span>{t("views.attachments", { count: item.attachment_count })}</span>}
        {!!item.comment_count && <span>{t("views.comments", { count: item.comment_count })}</span>}
      </span>
      <AvatarStack people={item.assignees} />
    </div>
    {canMove && <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="ghost" size="sm" className="h-auto w-full justify-start p-0" disabled={pending} aria-label={`${item.status?.name ?? t("views.noStatus")}. ${t("views.moveStatus")}: ${item.title}`} />}>
        {item.status ? <StatusChip status={item.status} /> : t("views.noStatus")}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">
        {statuses.map((status) => <DropdownMenuItem key={status.id} disabled={status.id === item.status?.id} onClick={() => void onPatch(item, { status_id: status.id })}>{status.name}</DropdownMenuItem>)}
      </DropdownMenuContent>
    </DropdownMenu>}
  </li>;
}
