import { formatNumber, formatDate, useProfile } from "@/lib/profile";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { AlertTriangle, Diamond, CalendarDays, ChevronDown, ChevronRight, EyeOff, ListTree, MoreHorizontal, PanelLeftClose, PanelLeftOpen, Plus, Rows3 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { canAssignOthers } from "@/components/item-user-picker";
import { toast } from "sonner";
import { CustomFieldDisplay } from "@/components/custom-field-editor";
import { AvatarStack } from "@/components/ui/avatar-stack";
import { StatusChip, tintOf } from "@/components/ui/status-chip";
import { statusDotClass } from "@/lib/status-colors";
import { TypeBadge, TypeGlyph } from "@/components/type-badge";
import { fieldColumn } from "@/lib/view-columns";
import { peopleOf } from "@/components/item-table";
import { itemKeys, listItems, type Item, type ItemUpdate } from "@/lib/items";
import { newItemId } from "@/lib/create-item-validate";
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

/** Board density and empty-column preferences, kept per view in this browser. The workspace owns them so the toggles can sit in the shared toolbar. */
export type BoardPrefs = { compact: boolean; hideEmpty: boolean; collapsed: string[]; toggleCompact: () => void; toggleEmpty: () => void; toggleColumn: (id: string) => void };
export function useBoardPrefs(preferenceKey: string): BoardPrefs {
  const emptyKey = `pms:board-hide-empty:${preferenceKey}`;
  // Empty columns are hidden unless the user turned that off before; "0" is the only value that shows them.
  const [hideEmpty, setHideEmpty] = useState(() => { try { return localStorage.getItem(emptyKey) !== "0"; } catch { return true; } });
  const densityKey = `pms:board-compact:${preferenceKey}`;
  const [compact, setCompact] = useState(() => { try { return localStorage.getItem(densityKey) !== "0"; } catch { return true; } });
  const collapsedKey = `pms:board-collapsed:${preferenceKey}`;
  const [collapsed, setCollapsed] = useState<string[]>(() => { try { const value = JSON.parse(localStorage.getItem(collapsedKey) ?? "[]"); return Array.isArray(value) ? value.filter((id): id is string => typeof id === "string") : []; } catch { return []; } });
  useEffect(() => {
    if (!preferenceKey) return;
    try {
      setHideEmpty(localStorage.getItem(emptyKey) !== "0");
      setCompact(localStorage.getItem(densityKey) !== "0");
      const value = JSON.parse(localStorage.getItem(collapsedKey) ?? "[]");
      setCollapsed(Array.isArray(value) ? value.filter((id): id is string => typeof id === "string") : []);
    } catch { /* storage can be blocked */ }
  }, [preferenceKey, emptyKey, densityKey, collapsedKey]);
  const toggleEmpty = () => setHideEmpty((value) => {
    const next = !value;
    try { localStorage.setItem(emptyKey, next ? "1" : "0"); } catch { /* storage can be blocked */ }
    return next;
  });
  const toggleCompact = () => setCompact((value) => {
    const next = !value;
    try { localStorage.setItem(densityKey, next ? "1" : "0"); } catch { /* storage can be blocked */ }
    return next;
  });
  const toggleColumn = (id: string) => setCollapsed((value) => { const next = value.includes(id) ? value.filter((entry) => entry !== id) : [...value, id]; try { localStorage.setItem(collapsedKey, JSON.stringify(next)); } catch { /* storage can be blocked */ } return next; });
  return { compact, hideEmpty, collapsed, toggleCompact, toggleEmpty, toggleColumn };
}

/** The two board toggles, rendered in the view toolbar row. */
export function BoardToggles({ prefs }: { prefs: BoardPrefs }) {
  const { t } = useTranslation();
  return <>
    <Button size="sm" variant={prefs.compact ? "secondary" : "outline"} aria-pressed={prefs.compact} onClick={prefs.toggleCompact}>
      <Rows3 />{t("views.boardCompact")}
    </Button>
    <Button size="sm" variant={prefs.hideEmpty ? "secondary" : "outline"} aria-pressed={prefs.hideEmpty} onClick={prefs.toggleEmpty}>
      <EyeOff />{t("views.boardHideEmpty")}
    </Button>
  </>;
}

export function ItemBoard({ items, statuses, customFields, columns: shown, swimlanes, prefs, canCreate, pending, onOpen, onAdd, onPatch, onQuickCreate }: ItemCollectionProps & { swimlanes: boolean; prefs: BoardPrefs; onQuickCreate?: (columnId: string, title: string, id: string) => Promise<void> }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const [dragging, setDragging] = useState<{ id: string; lane: string }>();
  // B2.2: the column whose inline name field is open.
  const [quick, setQuick] = useState<string>();
  const addButton = (columnId: string) => onQuickCreate
    ? (quick === columnId
      ? <QuickCreate onSubmit={(title, id) => onQuickCreate(columnId, title, id)} onCancel={() => setQuick(undefined)} onDetail={() => { setQuick(undefined); onAdd(columnId); }} />
      : <Button size="sm" variant="outline" onClick={() => setQuick(columnId)}><Plus />{t("views.add")}</Button>)
    : <Button size="sm" variant="outline" onClick={() => onAdd(columnId)}><Plus />{t("views.add")}</Button>;
  const [move, setMove] = useState<Move>();
  const [now] = useState(() => Date.now());
  const { hideEmpty, compact, collapsed, toggleColumn } = prefs;
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
      if (lane.key && lane.key !== profile?.id && !canAssignOthers(profile, item.my_capabilities)) return void toast.error(t("items.assignSelfOnly"));
      const ids = item.assignees.map((person) => person.id).filter((id) => id !== from);
      const update: ItemUpdate = { assignee_ids: lane.key ? [...new Set([...ids, lane.key])] : ids, ...(statusChanged ? { status_id: columnId } : {}) };
      const name = (key: string) => item.assignees.find((person) => person.id === key)?.name ?? lanes.find((entry) => entry.key === key)?.name ?? t("views.unassigned");
      setMove({ item, from, to: lane.key, update, fromName: name(from), toName: name(lane.key) });
    } else if (statusChanged) void onPatch(item, { status_id: columnId });
  }

  const lanes = lanesOf(items, swimlanes, t("views.unassigned"));
  return <div className="h-[calc(100dvh-19rem)] min-h-72 space-y-2 overflow-y-auto">
    {lanes.map((lane) => {
      const unlisted = lane.items.filter((item) => !item.status || !known.has(item.status.id));
      const cols = [
        ...statuses.map((status) => ({ id: status.id, name: status.name, color: status.color, items: lane.items.filter((item) => item.status?.id === status.id) })),
        ...(unlisted.length ? [{ id: "", name: t("views.noStatus"), color: noColor, items: unlisted }] : []),
      ];
      const visibleCols = cols.filter((column) => !hideEmpty || column.items.length > 0);
      const board = <div className="flex h-full gap-2 overflow-x-auto pb-2">
        {visibleCols.map((column) => { const columnKey = `${lane.key}:${column.id}`; const folded = collapsed.includes(columnKey); return <section key={column.id} aria-label={lane.name ? `${lane.name}: ${column.name}` : column.name}
          className={`${folded ? "w-12" : "w-72"} flex h-full shrink-0 flex-col gap-1.5 transition-[width]`}
          onDragOver={(event) => { if (dragging && !pending && (column.id || swimlanes)) event.preventDefault(); }}
          onDrop={(event) => { event.preventDefault(); drop(lane, column.id); }}>
          <h3 className={`sticky top-0 z-10 flex items-center gap-1.5 rounded-md px-2 py-1.5 text-[13px] font-semibold ${tintOf(column.color)}`}>
            {!folded && <><span aria-hidden className={`size-2 rounded-full ${statusDotClass(column.color)}`} />{column.name} <span className="font-normal">{formatNumber(column.items.length, profile)}</span>{column.items.some((item) => item.is_milestone) && <span className="text-xs font-normal text-muted-foreground">{t("views.boardBreakdown", { tasks: column.items.filter((item) => !item.is_milestone).length, milestones: column.items.filter((item) => item.is_milestone).length })}</span>}</>}
            <Button size="icon-sm" variant="ghost" className={folded ? "mx-auto" : "ml-auto"} aria-label={t(folded ? "views.boardExpandColumn" : "views.boardCollapseColumn", { name: column.name })} onClick={() => toggleColumn(columnKey)}>{folded ? <PanelLeftOpen /> : <PanelLeftClose />}</Button></h3>
          {!folded && <><ul className="min-h-0 flex-1 space-y-1.5 overflow-y-auto pr-1">
            {column.items.map((item) => <Card key={item.id} item={item} lane={lane.key} now={now} fields={cardFields} shown={shown} compact={compact} showStatus={swimlanes}
              statuses={statuses} pending={pending} onOpen={onOpen} onPatch={onPatch} onDrag={setDragging} />)}
          </ul>
          {canCreate && column.id && addButton(column.id)}</>}
        </section>; })}
      </div>;
      return swimlanes
        ? <section key={lane.key} aria-label={lane.name} className="space-y-2">
          <h2 className="flex items-center gap-2 font-semibold">{lane.name} <span className="text-muted-foreground">{formatNumber(lane.items.length, profile)}</span>
            {canCreate && lane.key && <Button size="icon-sm" variant="ghost" className="text-primary" aria-label={t("views.addToGroup", { name: lane.name ?? "" })} title={t("views.addToGroup", { name: lane.name ?? "" })}
              onClick={() => onAdd(undefined, undefined, { id: lane.key, name: lane.name ?? "" })}><Plus /></Button>}</h2>
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

/** Due before today and not finished. Uses the viewer's local day, so the badge matches what the date shows. */
function overdue(item: Item): boolean {
  if (!item.due_date || item.status?.group === "completed" || item.status?.group === "cancelled") return false;
  const today = new Date();
  const local = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, "0")}-${String(today.getDate()).padStart(2, "0")}`;
  return item.due_date < local;
}

function Card({ item, lane, now, fields, shown, compact, showStatus, statuses, pending, onOpen, onPatch, onDrag }: {
  item: Item; lane: string; now: number; fields: ItemCollectionProps["customFields"]; shown: string[];
  compact: boolean; showStatus: boolean;
  statuses: ItemCollectionProps["statuses"]; pending: boolean;
  onOpen: (item: Item) => void; onPatch: ItemCollectionProps["onPatch"]; onDrag: (value?: { id: string; lane: string }) => void;
}) {
  const { t } = useTranslation();
  const profile = useProfile();
  const canMove = item.my_capabilities.includes("change_status");
  const draggable = (canMove || item.my_capabilities.includes("edit")) && !pending;
  const [childrenOpen, setChildrenOpen] = useState(false);
  const children = useQuery({ queryKey: itemKeys.children(item.id), queryFn: () => listItems(`/api/v1/items/${item.id}/children`), enabled: childrenOpen && !!item.child_count });
  const meta = [
    shown.includes("id") && `#${item.seq}`,
    shown.includes("importance") && t(`items.importance.${item.importance ?? "normal"}`),
    shown.includes("days_in_status") && item.status_changed_at && t("views.card.daysInStatus", { count: Math.max(0, Math.floor((now - Date.parse(item.status_changed_at)) / DAY)) }),
    shown.includes("author") && item.created_by && t("views.card.author", { name: item.created_by.name }),
    shown.includes("locations") && item.locations.map((loc) => loc.space_name).filter(Boolean).join(", "),
  ].filter(Boolean);
  const milestone = item.is_milestone ? <span className="inline-flex items-center gap-1 rounded bg-muted px-1.5 py-0.5 text-xs font-medium text-foreground"><Diamond aria-hidden className="size-3" />{t("views.milestone")}</span> : null;
  return <li className="rounded-lg border bg-card shadow-xs" style={{ contentVisibility: "auto", containIntrinsicSize: "120px" }}
    draggable={draggable} onDragStart={(event) => { event.dataTransfer.setData("text/plain", item.id); onDrag({ id: item.id, lane }); }}
    onDragEnd={() => onDrag(undefined)}>
    <div className={compact ? "space-y-1 p-2" : "space-y-2 p-3"}>
      {shown.includes("cover") && item.cover_attachment_id && <img alt="" loading="lazy" className={`${compact ? "h-16" : "h-24"} w-full rounded object-cover`}
        src={`/api/v1/attachments/${item.cover_attachment_id}/download`} />}
      <div className="flex items-start gap-2">
        <TypeGlyph item={item} className="mt-1.5 size-4 text-muted-foreground" />
        <Button variant="ghost" size="sm" title={item.title} className="line-clamp-2 h-auto min-w-0 flex-1 justify-start whitespace-normal p-0 text-left text-[13px] font-medium hover:bg-transparent hover:underline" onClick={() => onOpen(item)}>{item.title}</Button>
        {canMove && !showStatus && <DropdownMenu>
          <DropdownMenuTrigger render={<Button size="icon-sm" variant="ghost" disabled={pending} aria-label={`${t("views.moveStatus")}: ${item.title}`} />}><MoreHorizontal /></DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {statuses.map((status) => <DropdownMenuItem key={status.id} disabled={status.id === item.status?.id} onClick={() => void onPatch(item, { status_id: status.id })}>{status.name}</DropdownMenuItem>)}
          </DropdownMenuContent>
        </DropdownMenu>}
      </div>
      {/* Agent B: the type icon already shows the type; the name stays for screen readers and tests (sr-only). */}
      {(!!meta.length || !!item.item_type || !!item.is_milestone) && <p className="flex flex-wrap items-center gap-x-1 text-xs text-muted-foreground">
        {item.item_type && <span className="sr-only"><TypeBadge item={item} /></span>}{milestone}{!!meta.length && <span>{meta.join(" · ")}</span>}</p>}
      {((shown.includes("start_date") && item.start_date) || (shown.includes("due_date") && item.due_date) || (shown.includes("assignee") && item.assignees.length > 0)) && <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
        {shown.includes("assignee") && <AvatarStack people={item.assignees} />}
        {((shown.includes("start_date") && item.start_date) || (shown.includes("due_date") && item.due_date)) && <p className="ml-auto flex items-center gap-1"><CalendarDays className="size-3" aria-hidden />{[
          shown.includes("start_date") && item.start_date,
          shown.includes("due_date") && item.due_date,
        ].filter((date): date is string => !!date).map((date) => formatDate(date, profile)).join(" – ")}
          {item.start_date && item.due_date && <span>({t("views.card.duration", { count: Math.max(1, Math.round((Date.parse(item.due_date) - Date.parse(item.start_date)) / DAY) + 1) })})</span>}
        </p>}
      </div>}
      {overdue(item) && <p className="flex items-center gap-1 text-xs font-medium text-destructive"><AlertTriangle className="size-3" aria-hidden />{t("views.card.overdue")}</p>}
      {item.type === "project" && item.progress_percent === 0 && <p className="text-xs text-muted-foreground">{t("items.progress.notStarted")}</p>}
      {item.type === "project" && item.progress_percent != null && item.progress_percent > 0 && <p className="flex items-center gap-2 text-xs tabular-nums text-muted-foreground">
        <span aria-hidden className="relative block h-1 flex-1 overflow-hidden rounded-full bg-muted"><span className="absolute inset-y-0 start-0 rounded-full bg-primary" style={{ width: `${item.progress_percent}%` }} /></span>
        {t("items.progress.percent", { value: item.progress_percent })}</p>}
      {profile && fields.map((field) => <p key={field.id} className="flex flex-wrap gap-1 text-xs">
        <span className="text-muted-foreground">{field.name}:</span>
        <CustomFieldDisplay field={field} value={item.fields?.[field.id]} people={peopleOf(item, field, item.fields?.[field.id])} profile={profile} />
      </p>)}
    </div>
    {(!!item.child_count || !!item.attachment_count || !!item.comment_count) && <div className={`flex flex-wrap items-center gap-3 border-t px-3 ${compact ? "py-1" : "py-2"} text-xs text-muted-foreground`}>
      {!!item.child_count && <button type="button" title={t("items.childCount", { count: item.child_count })} aria-expanded={childrenOpen} className="inline-flex items-center gap-1 hover:underline" onClick={() => setChildrenOpen((value) => !value)}>{childrenOpen ? <ChevronDown className="size-3.5" aria-hidden /> : <ChevronRight className="size-3.5" aria-hidden />}<ListTree className="size-3.5" aria-hidden />{item.child_count}<span className="sr-only"> {t("items.childCount", { count: item.child_count })}</span></button>}
      {!!item.attachment_count && <span>{t("views.attachments", { count: item.attachment_count })}</span>}
      {!!item.comment_count && <span>{t("views.comments", { count: item.comment_count })}</span>}
    </div>}
    {childrenOpen && <div className="border-t bg-muted/20 px-3 py-2"><ul className="space-y-1">
      {children.isPending && <li className="text-xs text-muted-foreground">{t("items.loading")}</li>}
      {children.data?.map((child) => <li key={child.id}><button type="button" className="flex w-full items-center gap-2 rounded px-1 py-1 text-left text-xs hover:bg-muted" onClick={() => onOpen(child)}><TypeGlyph item={child} className="size-3" /><span className="min-w-0 flex-1 truncate">{child.title}</span>{child.status && <StatusChip status={child.status} />}</button></li>)}
    </ul></div>}
    {canMove && showStatus && <div className={`border-t px-3 ${compact ? "py-1" : "py-2"}`}><DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="ghost" size="sm" className="h-auto w-full justify-start p-0" disabled={pending} aria-label={`${item.status?.name ?? t("views.noStatus")}. ${t("views.moveStatus")}: ${item.title}`} />}>
        {item.status ? <StatusChip status={item.status} /> : t("views.noStatus")}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">
        {statuses.map((status) => <DropdownMenuItem key={status.id} disabled={status.id === item.status?.id} onClick={() => void onPatch(item, { status_id: status.id })}>{status.name}</DropdownMenuItem>)}
      </DropdownMenuContent>
    </DropdownMenu></div>}
  </li>;
}

/** B2.2: a Task name typed inside a column. Enter or leaving the field creates it, Escape cancels, and a failed request keeps the name. */
function QuickCreate({ onSubmit, onCancel, onDetail }: { onSubmit: (title: string, id: string) => Promise<void>; onCancel: () => void; onDetail: () => void }) {
  const { t } = useTranslation();
  const [title, setTitle] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  // One request at a time: Enter followed by blur must not create the item twice.
  const inFlight = useRef(false);
  // The id is chosen once per name: a retry after a lost response reuses it, so the server cannot store a second item.
  const attempt = useRef<{ name: string; id: string } | undefined>(undefined);
  async function submit() {
    if (inFlight.current) return;
    const name = title.trim();
    if (!name) { onCancel(); return; }
    if (attempt.current?.name !== name) attempt.current = { name, id: newItemId() };
    inFlight.current = true;
    setBusy(true);
    setError("");
    try {
      await onSubmit(name, attempt.current.id);
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : t("views.createFailed"));
      inFlight.current = false;
      setBusy(false);
    }
  }
  return <div className="space-y-1 rounded-md border bg-card p-2">
    <Input autoFocus aria-label={t("views.quickCreateName")} placeholder={t("views.quickCreateName")} value={title} disabled={busy} maxLength={500}
      onChange={(event) => setTitle(event.target.value)} onBlur={() => void submit()}
      onKeyDown={(event) => {
        if (event.key === "Enter") { event.preventDefault(); void submit(); }
        if (event.key === "Escape") { event.preventDefault(); onCancel(); }
      }} />
    {error && <p role="alert" className="text-xs text-destructive">{error}</p>}
    <button type="button" className="text-xs text-primary underline" onMouseDown={(event) => event.preventDefault()} onClick={onDetail}>{t("views.quickCreateDetail")}</button>
  </div>;
}
