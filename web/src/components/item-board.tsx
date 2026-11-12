import { formatNumber, formatDate, useProfile } from "@/lib/profile";
import { useState } from "react";
import { CalendarDays, Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { CustomFieldDisplay } from "@/components/custom-field-editor";
import { fieldColumn } from "@/lib/view-columns";
import { peopleOf } from "@/components/item-table";
import type { ItemCollectionProps } from "./item-collection-types";

export function ItemBoard({ items, statuses, customFields, columns: shown, canCreate, pending, onOpen, onAdd, onPatch }: ItemCollectionProps) {
  const { t } = useTranslation();
  const profile = useProfile();
  const [dragging, setDragging] = useState<string>();
  const cardFields = customFields.filter((field) => shown.includes(fieldColumn(field.id)));
  const known = new Set(statuses.map((status) => status.id));
  const unlisted = items.filter((item) => !item.status || !known.has(item.status.id));
  const columns = [
    ...statuses.map((status) => ({ id: status.id, name: status.name, items: items.filter((item) => item.status?.id === status.id) })),
    ...(unlisted.length ? [{ id: "", name: t("views.noStatus"), items: unlisted }] : []),
  ];
  return <div className="flex gap-3 overflow-x-auto pb-3">
    {columns.map((column) => <section key={column.id} aria-label={column.name} className="w-64 shrink-0 space-y-2 rounded border bg-muted/30 p-3"
      onDragOver={(event) => { if (dragging && column.id && !pending) event.preventDefault(); }}
      onDrop={(event) => {
        event.preventDefault();
        const item = items.find((entry) => entry.id === dragging);
        if (item && column.id && column.id !== item.status?.id && item.my_capabilities.includes("change_status") && !pending) {
          void onPatch(item, { status_id: column.id });
        }
        setDragging(undefined);
      }}>
      <h3 className="font-semibold">{column.name} <span className="text-muted-foreground">{formatNumber(column.items.length, profile)}</span></h3>
      <ul className="space-y-2">
        {column.items.map((item) => {
          const canMove = item.my_capabilities.includes("change_status");
          return <li key={item.id} className="space-y-2 rounded border bg-card p-2 shadow-xs"
            draggable={canMove && !pending} onDragStart={(event) => { event.dataTransfer.setData("text/plain", item.id); setDragging(item.id); }}
            onDragEnd={() => setDragging(undefined)}>
            <Button variant="ghost" size="sm" className="h-auto max-w-full justify-start whitespace-normal text-left" onClick={() => onOpen(item)}>{item.title}</Button>
            {!!item.assignees.length && <p className="text-xs text-muted-foreground">{item.assignees.map((person) => person.name).join(", ")}</p>}
            {(item.start_date || item.due_date) && <p className="flex items-center gap-1 text-xs text-muted-foreground">
              <CalendarDays className="size-3" />{[item.start_date, item.due_date].filter(Boolean).map((date) => formatDate(date, profile)).join(" – ")}
            </p>}
            {profile && cardFields.map((field) => <p key={field.id} className="flex flex-wrap gap-1 text-xs">
              <span className="text-muted-foreground">{field.name}:</span>
              <CustomFieldDisplay field={field} value={item.fields?.[field.id]} people={peopleOf(item, field, item.fields?.[field.id])} profile={profile} />
            </p>)}
            <p className="flex flex-wrap gap-2 text-xs text-muted-foreground">
              {item.child_count !== undefined && <span>{t("items.childCount", { count: item.child_count })}</span>}
              <span>{t("views.attachments", { count: item.attachment_count ?? 0 })}</span>
              <span>{t("views.comments", { count: item.comment_count ?? 0 })}</span>
            </p>
            {canMove && <select aria-label={`${t("views.moveStatus")}: ${item.title}`}
              className="w-full rounded border bg-background px-1 py-1 text-xs" disabled={pending} value={item.status?.id ?? ""}
              onChange={(event) => void onPatch(item, { status_id: event.target.value })}>
              {(!item.status || !known.has(item.status.id)) && <option value={item.status?.id ?? ""} disabled>{item.status?.name ?? t("views.noStatus")}</option>}
              {statuses.map((status) => <option key={status.id} value={status.id}>{status.name}</option>)}
            </select>}
          </li>;
        })}
      </ul>
      {canCreate && column.id && <Button size="sm" variant="outline" onClick={() => onAdd(column.id)}><Plus />{t("views.add")}</Button>}
    </section>)}
  </div>;
}
