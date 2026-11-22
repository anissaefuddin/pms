import { createContext, useContext, useEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { Gantt, Willow, WillowDark, type IApi, type ITask } from "@svar-ui/react-gantt";
import "@svar-ui/react-gantt/style.css";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ChevronDown, ChevronLeft, ChevronRight, ChevronsDownUp, ChevronsUpDown, Minus, MoreHorizontal, Plus } from "lucide-react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { canAddSubitem } from "@/lib/create-item-validate";
import { GanttSnapshotDialog } from "@/components/gantt-snapshot-dialog";
import { ApiError, api } from "@/lib/api";
import type { components } from "@/api/schema";
import { formatDate, useProfile } from "@/lib/profile";
import { itemKeys, listItems, type Item } from "@/lib/items";
import { addDays, fromDay, toDay } from "@/lib/calendar-days";
import { dayCellWidth, dayIndexAt, dayRange, isDragStep } from "@/lib/gantt-timeline";
import { loadWorkSchedule, workScheduleKey } from "@/lib/work-schedule";
import { customFieldText, type CustomField } from "@/lib/custom-fields";
import { fieldColumn } from "@/lib/view-columns";
import { statusDotClass } from "@/lib/status-colors";

type Baseline = components["schemas"]["Baseline"];
type Variance = components["schemas"]["BaselineVariance"];
type Dependency = components["schemas"]["ItemDependency"];
type DependencyType = Dependency["type"];
const linkTypes = { FS: "e2s", SS: "s2s", FF: "e2e", SF: "s2e" } as const;
const dependencyTypes: DependencyType[] = ["FS", "SS", "FF", "SF"];
const fromLink = (type: string) => (Object.keys(linkTypes) as DependencyType[]).find((key) => linkTypes[key] === type);
type Scale = "day" | "week" | "month" | "quarter" | "year";
const scaleOptions: Scale[] = ["year", "quarter", "month", "week", "day"];
/** One row per scale: the px a cell takes, and how many px one day takes (for the off-view count). */
const scaleGeometry: Record<Scale, { cell: number; perDay: number }> = {
  year: { cell: 24, perDay: 24 / 30 }, quarter: { cell: 90, perDay: 90 / 90 }, month: { cell: 90, perDay: 90 / 30 },
  week: { cell: 80, perDay: 80 / 7 }, day: { cell: 36, perDay: 36 },
};
/** Bar colours by workflow colour token: 700/900 shades keep white labels above 4.5:1. */
const barColors: Record<string, [string, string]> = {
  slate: ["#334155", "#0f172a"], blue: ["#1d4ed8", "#1e3a8a"], sky: ["#0369a1", "#0c4a6e"], teal: ["#0f766e", "#134e4a"], // allow-hex
  green: ["#15803d", "#14532d"], lime: ["#4d7c0f", "#365314"], amber: ["#b45309", "#78350f"], orange: ["#c2410c", "#7c2d12"], // allow-hex
  red: ["#b91c1c", "#7f1d1d"], pink: ["#be185d", "#831843"], purple: ["#7e22ce", "#581c87"], indigo: ["#4338ca", "#312e81"], // allow-hex
};
/** Bar content: fill coloured by status (darker part = progress) plus "name • assignees". */
function BarLabel({ data }: { data: ITask }) {
  const [base, dark] = barColors[String(data.status_color)] ?? barColors.blue;
  const done = Math.min(100, Math.max(0, Number(data.progress ?? 0)));
  return <div data-bar-status={String(data.status_color ?? "")} data-bar-kind={String(data.type ?? "task")} data-bar-state={String(data.status_group ?? "")} data-overdue={String(!!data.overdue)} className="gantt-bar-fill"
    style={{ background: `linear-gradient(to right, ${dark} ${done}%, ${base} ${done}%)` }}>
    <span className="truncate">{[data.text, data.assignee_names].filter(Boolean).join(" • ")}</span>
  </div>;
}

/** Predecessors in one cell: the task numbers. Click to add, remove, or change the link type; this is the old panel, now in the column. */
function PredecessorCell({ row, open, onOpenChange, options, value, readonly, busy, onAdd, onRemove, onType }: {
  row: ITask; open: boolean; onOpenChange: (open: boolean) => void; options: ITask[]; value: { id: string; predecessorId: string; number: number; type: DependencyType }[]; readonly: boolean; busy: boolean;
  onAdd: (predecessor: string, successor: string, type: DependencyType) => void; onRemove: (id: string) => void; onType: (id: string, type: DependencyType) => void;
}) {
  const { t } = useTranslation();
  const successor = String(row.id);
  const name = String(row.text ?? "");
  const shown = value.map((entry) => entry.number).sort((x, y) => x - y).join(", ");
  const titleOf = (predecessorId: string) => options.find((task) => String(task.id) === predecessorId)?.text ?? predecessorId;
  return <Popover open={open} onOpenChange={onOpenChange}>
    <PopoverTrigger render={<button type="button" disabled={readonly} aria-label={shown ? `${t("views.gantt.predecessorsOf", { name })}: ${shown}` : t("views.gantt.predecessorsOf", { name })}
      className="min-w-0 truncate rounded px-1 text-left tabular-nums hover:bg-muted disabled:hover:bg-transparent" />}>
      {shown || (readonly ? null : <Plus aria-hidden className="size-3.5 text-muted-foreground" />)}
    </PopoverTrigger>
    <PopoverContent align="start" className="w-80 space-y-3">
      <p className="font-medium">{t("views.gantt.dependencies", { name })}</p>
      <ul className="space-y-1">
        {value.map((entry) => <li key={entry.id} className="flex flex-wrap items-center gap-2 text-sm">
          <span className="min-w-0 truncate">{entry.number}. {titleOf(entry.predecessorId)}</span>
          <select aria-label={t("views.gantt.depType", { name: titleOf(entry.predecessorId) })} value={entry.type} disabled={readonly || busy}
            className="rounded border bg-background p-1" onChange={(event) => onType(entry.id, event.target.value as DependencyType)}>
            {dependencyTypes.map((type) => <option key={type} value={type}>{type}</option>)}
          </select>
          {!readonly && <Button size="sm" variant="outline" disabled={busy} onClick={() => onRemove(entry.id)}
            aria-label={t("views.gantt.removeDep", { name: titleOf(entry.predecessorId) })}>{t("views.gantt.remove")}</Button>}
        </li>)}
        {!value.length && <li className="text-sm text-muted-foreground">{t("views.gantt.noDeps")}</li>}
      </ul>
      {!readonly && <form className="flex flex-wrap items-end gap-2" onSubmit={(event) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        const predecessor = String(data.get("predecessor") ?? "");
        if (predecessor) onAdd(predecessor, successor, String(data.get("type")) as DependencyType);
        event.currentTarget.reset();
      }}>
        <label className="space-y-1"><span className="block text-sm">{t("views.gantt.predecessor")}</span>
          <select name="predecessor" required defaultValue="" className="rounded border bg-background p-1">
            <option value="" disabled />
            {options.filter((task) => String(task.id) !== successor).map((task) => <option key={String(task.id)} value={String(task.id)}>{task.text}</option>)}
          </select></label>
        <label className="space-y-1"><span className="block text-sm">{t("views.gantt.type")}</span>
          <select name="type" defaultValue="FS" className="rounded border bg-background p-1">
            {dependencyTypes.map((type) => <option key={type} value={type}>{type}</option>)}
          </select></label>
        <Button type="submit" size="sm" disabled={busy}>{t("views.gantt.addDep")}</Button>
      </form>}
    </PopoverContent>
  </Popover>;
}

/** Task name in the grid: double-click or F2 to edit. Enter saves, Escape cancels, a click outside saves. The draft lives in ItemGantt, so a grid re-render cannot drop what was typed. */
function NameCell({ row, editing, draft, canEdit, onStart, onDraft, onEnd, onSave }: { row: ITask; editing: boolean; draft: string; canEdit: boolean; onStart: (id: string) => void; onDraft: (value: string) => void; onEnd: () => void; onSave: (id: string, title: string) => void }) {
  const { t } = useTranslation();
  const title = String(row.text ?? "");
  const input = useRef<HTMLInputElement>(null);
  const finished = useRef(false);
  const latest = useRef({ draft, finish: (_value: string | null) => {} });
  function finish(value: string | null) {
    if (finished.current) return;
    finished.current = true;
    onEnd();
    const next = value?.trim() ?? "";
    if (next && next !== title) onSave(String(row.id), next);
  }
  latest.current = { draft, finish };
  useEffect(() => { if (editing) finished.current = false; }, [editing]);
  useEffect(() => {
    if (!editing) return;
    // Focus moves inside the grid while it renders, so blur is not a reliable end; only a click outside the field saves.
    const outside = (event: MouseEvent) => { if (input.current && !input.current.contains(event.target as Node)) latest.current.finish(latest.current.draft); };
    document.addEventListener("mousedown", outside);
    return () => document.removeEventListener("mousedown", outside);
  }, [editing]);
  if (!editing) return <span className="min-w-0 truncate" title={title} onDoubleClick={canEdit ? () => onStart(String(row.id)) : undefined}>{title}</span>;
  return <form className="min-w-0 flex-1" onSubmit={(event) => { event.preventDefault(); finish(draft); }}>
    <input ref={input} name="title" value={draft} autoFocus maxLength={500} aria-label={t("views.gantt.name")} className="w-full rounded border bg-background px-1 text-sm"
      onChange={(event) => onDraft(event.target.value)}
      onKeyDown={(event) => { if (event.key === "Escape") { finished.current = true; onEnd(); } }} />
  </form>;
}

/** Status column: workflow colour square + name (the name stays text, colour is only a cue). */
function StatusCell({ row }: { row: ITask }) {
  return <span className="inline-flex items-center gap-1.5"><span aria-hidden className={`size-2.5 shrink-0 rounded-sm ${statusDotClass(String(row.status_color ?? ""))}`} />{String(row.status_name ?? "")}</span>;
}

export function ganttTask(item: Item, parent?: string): ITask {
  const start = toDay(item.start_date) ?? toDay(item.due_date);
  const finish = toDay(item.due_date) ?? start;
  const today = toDay(new Date().toISOString().slice(0, 10));
  return {
    status_name: item.status?.name ?? "", status_color: item.status?.color ?? "", status_group: item.status?.group ?? "",
    overdue: !!finish && !!today && finish < today && item.status?.group !== "completed",
    assignee_names: item.assignees.map((person) => person.name).join(", "),
    id: item.id, text: item.title, parent: parent ?? 0, open: true, progress: item.progress_percent ?? 0,
    type: item.is_milestone ? "milestone" : item.child_count ? "summary" : "task",
    ...(start && finish ? { start, end: addDays(finish, 1) } : { unscheduled: true }), // SVAR's end is exclusive
  };
}

export type GanttRowAdd = "child" | "above" | "below";

/**
 * Batch 3B: the row menu. "Add subitem" uses the same rule as the Table and the Item Detail panel. "Add above" and
 * "Add below" need a date on the row (the new Task takes that day) and create permission on the row.
 */
function GanttRowMenu({ item, previousId, spaceId, onAdd }: { item: Item; previousId: string | null; spaceId: string; onAdd: (kind: GanttRowAdd, item: Item, previousId: string | null) => void }) {
  const { t } = useTranslation();
  const dated = !!(item.start_date || item.due_date);
  const allowed = item.my_capabilities.includes("create");
  if (!canAddSubitem(item.my_capabilities) && !(allowed && dated)) return null;
  void spaceId;
  return <DropdownMenu>
    <DropdownMenuTrigger render={<Button size="icon-sm" variant="ghost" aria-label={t("views.gantt.rowActions", { name: item.title })} />}><MoreHorizontal /></DropdownMenuTrigger>
    <DropdownMenuContent align="end">
      {canAddSubitem(item.my_capabilities) && <DropdownMenuItem onClick={() => onAdd("child", item, null)}>{t("items.addChild")}</DropdownMenuItem>}
      {allowed && dated && <DropdownMenuItem onClick={() => onAdd("above", item, previousId)}>{t("views.gantt.addAbove")}</DropdownMenuItem>}
      {allowed && dated && <DropdownMenuItem onClick={() => onAdd("below", item, previousId)}>{t("views.gantt.addBelow")}</DropdownMenuItem>}
    </DropdownMenuContent>
  </DropdownMenu>;
}

type RowActionsContextValue = { tasks: ITask[]; spaceId: string; byId: Map<string, Item>; onRowAdd: (kind: GanttRowAdd, item: Item, previousId: string | null) => void };
const RowActionsContext = createContext<RowActionsContextValue | null>(null);

/** Stable module-level cell: SVAR keeps its menu state only while this component identity is unchanged. */
function RowActionsCell({ row }: { row: ITask }) {
  const ctx = useContext(RowActionsContext);
  const item = ctx?.byId.get(String(row.id));
  if (!ctx || !item) return null;
  // Siblings come from the tree the chart shows (same parent in display order), so "above" follows what the user sees.
  const siblings = ctx.tasks.filter((task) => String(task.parent ?? 0) === String(row.parent ?? 0));
  const index = siblings.findIndex((task) => String(task.id) === String(row.id));
  const previousId = index > 0 ? String(siblings[index - 1].id) : null;
  return <GanttRowMenu item={item} previousId={previousId} spaceId={ctx.spaceId} onAdd={ctx.onRowAdd} />;
}

export function ItemGantt({ initialOpen, onOpenChange, items, spaceId, parentId, readonly, customFields, columns, onOpen, onPatch, onAdd, onRowAdd, onTimelineCreate, onQuickCreate, renderToolbar }: {
  initialOpen?: boolean; // WA-D05: whether the hierarchy opens expanded
  onOpenChange?: (open: boolean) => void;
  items: Item[];
  customFields: CustomField[];
  columns: string[];
  spaceId: string;
  parentId?: string;
  readonly: boolean;
  onOpen: (item: Item) => void;
  onAdd?: () => void;
  /** The filter row. The workspace passes the shared toolbar; the Gantt's own controls go into its columns slot. */
  renderToolbar?: (controls: ReactNode) => ReactNode;
  /** Creates a Task with this name at the end of the list, immediately. Absent when the viewer cannot create. */
  onQuickCreate?: (title: string, id: string) => Promise<void>;
  /** Batch 3B: add a subitem, or a Task above/below a row (previousId is the row above it among its siblings, or null). */
  onRowAdd?: (kind: GanttRowAdd, item: Item, previousId: string | null) => void;
  /** Batch 3C: a double-click or a drag on empty timeline days opens Quick Create with that range (YYYY-MM-DD, inclusive). */
  onTimelineCreate?: (start: string, finish: string) => void;
  onPatch: (item: Item, update: { start_date?: string | null; due_date?: string | null; title?: string }) => Promise<void>;
}) {
  const { t, i18n } = useTranslation();
  const profile = useProfile();
  const [scale, setScale] = useState<Scale>("day");
  const [selected, setSelected] = useState<string>();
  const [baselineId, setBaselineId] = useState("");
  const ganttPrefKey = `pms:gantt-grid:${spaceId}:${parentId ?? "root"}`;
  const [gridWidth, setGridWidth] = useState(() => { try { return Math.min(720, Math.max(280, Number(localStorage.getItem(`${ganttPrefKey}:width`)) || 480)); } catch { return 480; } });
  const [naming, setNaming] = useState(false);
  // WA-D05: the collapse state comes from the server profile for this view and every change is sent back.
  const [allOpen, setAllOpenState] = useState(() => initialOpen ?? true);
  const setAllOpen = (open: boolean) => { setAllOpenState(open); onOpenChange?.(open); };
  const [sharing, setSharing] = useState(false);
  const chart = useRef<IApi>(undefined);
  const chartRegion = useRef<HTMLDivElement>(null);
  // Every level is fetched with the same cache keys as Table, so SSE updates and PATCH responses reach the chart.
  const [parents, setParents] = useState<string[]>([]);
  const branches = useQueries({
    queries: parents.map((id) => ({ queryKey: itemKeys.children(id), queryFn: () => listItems(`/api/v1/items/${id}/children`) })),
    combine: (results) => results.map((result) => result.data),
  });
  const hasColumn = (id: string) => columns.includes(id);
  const shownFields = useMemo(() => customFields.filter((field) => columns.includes(fieldColumn(field.id))), [customFields, columns]);
  const { tasks, byId } = useMemo(() => {
    const children = new Map(parents.map((id, index) => [id, branches[index] ?? []]));
    const out: ITask[] = [];
    const lookup = new Map<string, Item>();
    function visit(list: Item[], parent: string | undefined, ancestors: Set<string>) {
      for (const item of list) {
        if (ancestors.has(item.id) || lookup.has(item.id)) continue;
        lookup.set(item.id, item);
        out.push({ ...ganttTask(item, parent), start_text: formatDate(item.start_date, profile), due_text: formatDate(item.due_date, profile), ...Object.fromEntries(shownFields.map((field) => [`cf_${field.id}`, customFieldText(field, item.fields?.[field.id], i18n.language, item.field_people)])) });
        visit((children.get(item.id) ?? []).filter((child) => child.locations.some((loc) => loc.space_id === spaceId && loc.parent_item_id === item.id)),
          item.id, new Set([...ancestors, item.id]));
      }
    }
    visit(items, undefined, new Set());
    return { tasks: out, byId: lookup };
  }, [items, parents, branches, spaceId, parentId, shownFields, i18n.language, profile]);
  useEffect(() => {
    const wanted = [...byId.values()].filter((item) => item.child_count).map((item) => item.id).filter((id) => !parents.includes(id));
    if (wanted.length) setParents((list) => [...list, ...wanted]);
  }, [byId, parents]);

  const qc = useQueryClient();
  const baselines = useQuery({ queryKey: ["baselines", parentId], enabled: !!parentId, queryFn: () => api<{ items: Baseline[] }>(`/api/v1/items/${parentId}/baselines`) });
  const variance = useQuery({ queryKey: ["baseline-variance", baselineId], enabled: !!baselineId,
    queryFn: () => api<{ items: Variance[] }>(`/api/v1/baselines/${baselineId}/variance`) });
  const varianceById = useMemo(() => new Map((baselineId ? variance.data?.items ?? [] : []).map((v) => [v.item_id, v])), [baselineId, variance.data]);
  const shownTasks = useMemo(() => tasks.map((open) => {
    const task = { ...open, open: allOpen };
    const v = varianceById.get(String(task.id));
    if (!v) return task;
    // shortcut: the free SVAR build has no baseline bars, so the frozen dates are shown as grid columns; upgrade with SVAR PRO.
    const range = (a?: string | null, b?: string | null) => [a ?? b, b ?? a].filter(Boolean).map((d) => formatDate(d, profile)).join(" – ");
    return { ...task, baseline: range(v.baseline_start, v.baseline_due),
      variance: v.due_variance_days == null ? "" : `${v.due_variance_days > 0 ? "+" : ""}${t("views.gantt.varianceValue", { days: v.due_variance_days })}` };
  }), [tasks, varianceById, t, profile, allOpen]);
  const itemIds = useMemo(() => [...byId.keys()].sort().slice(0, 1000), [byId]);
  const dependencies = useQuery({
    queryKey: ["gantt-dependencies", itemIds], enabled: itemIds.length > 1,
    queryFn: () => api<{ items: Dependency[] }>("/api/v1/dependencies/query", { method: "POST", body: JSON.stringify({ item_ids: itemIds }) }),
  });
  const deps = dependencies.data?.items ?? [];
  const links = useMemo(() => deps.map((dep) => ({ id: dep.id, source: dep.predecessor_id, target: dep.successor_id, type: linkTypes[dep.type] })), [deps]);
  const [busy, setBusy] = useState(false);


  /** The server moves the successors (D-37); afterwards every list and the links are read again. */
  async function mutate(work: () => Promise<unknown>) {
    if (busy) return;
    setBusy(true);
    try { await work(); }
    catch (err) {
      toast.error(err instanceof ApiError && (err.status === 409 && err.message === "" || err.status === 428) ? ref.current.t("items.statusConflict") : (err as Error).message);
    } finally {
      await Promise.all([qc.invalidateQueries({ queryKey: ["gantt-dependencies"] }), qc.invalidateQueries({ queryKey: itemKeys.all })]);
      setBusy(false);
    }
  }
  const addDependency = (predecessor: string, successor: string, type: DependencyType) => mutate(() =>
    api(`/api/v1/items/${successor}/dependencies`, { method: "POST", body: JSON.stringify({ predecessor_id: predecessor, type, lag: 0 }) }));
  const changeDependency = (id: string, type: DependencyType) => mutate(() =>
    api(`/api/v1/dependencies/${id}`, { method: "PATCH", body: JSON.stringify({ type }) }));
  const removeDependency = (id: string) => mutate(() => api(`/api/v1/dependencies/${id}`, { method: "DELETE" }));

  // SVAR only auto-sizes to a default window, so a plan months away from today would be off screen.
  // Items without a start or finish date cannot be drawn; say how many, so they are not mistaken for missing work.
  const undated = useMemo(() => tasks.filter((task) => !task.start || !task.end).length, [tasks]);
  const range = useMemo(() => {
    const dated = tasks.filter((task) => task.start && task.end);
    if (!dated.length) return {};
    const first = Math.min(...dated.map((task) => task.start!.getTime()));
    const last = Math.max(...dated.map((task) => task.end!.getTime()));
    return { start: addDays(new Date(first), -3), end: addDays(new Date(last), 21) };
  }, [tasks]);
  // Bars that lie left or right of the visible part of the chart. Counted on scroll, so the user sees that work exists beyond the view.
  const [outside, setOutside] = useState<{ left: string[]; right: string[] }>({ left: [], right: [] });
  const pxPerDay = scaleGeometry[scale].perDay;
  const dayOf = (date: Date) => Math.round((date.getTime() - range.start!.getTime()) / 86_400_000);
  function measureOutside() {
    const chart = chartRegion.current?.querySelector<HTMLElement>(".wx-chart");
    if (!chart || !range.start) { setOutside({ left: [], right: [] }); return; }
    const first = chart.scrollLeft / pxPerDay;
    const last = (chart.scrollLeft + chart.clientWidth) / pxPerDay;
    const dated = tasks.filter((task) => task.start && task.end);
    setOutside({
      left: dated.filter((task) => dayOf(task.end!) < first).sort((a, b) => dayOf(b.end!) - dayOf(a.end!)).map((task) => String(task.id)),
      right: dated.filter((task) => dayOf(task.start!) > last).sort((a, b) => dayOf(a.start!) - dayOf(b.start!)).map((task) => String(task.id)),
    });
  }
  // Scroll events from the chart do not bubble, so the listener sits on the region in the capture phase.
  useEffect(() => {
    const region = chartRegion.current;
    region?.addEventListener("scroll", measureOutside, true);
    measureOutside();
    return () => region?.removeEventListener("scroll", measureOutside, true);
  }, [tasks, scale, range.start?.getTime()]);
  function jumpOutside(side: "left" | "right") {
    const chart = chartRegion.current?.querySelector<HTMLElement>(".wx-chart");
    const id = side === "left" ? outside.left[0] : outside.right[0];
    const task = tasks.find((entry) => String(entry.id) === id);
    if (!chart || !task || !range.start) return;
    if (side === "right") chart.scrollLeft = Math.max(0, dayOf(task.start!) * pxPerDay - 2 * pxPerDay);
    else chart.scrollLeft = Math.max(0, dayOf(task.end!) * pxPerDay - chart.clientWidth + 3 * pxPerDay);
  }
  // Batch 3C: the drag in progress on the timeline. Nothing is created until the pointer is released after a real move.
  const [drag, setDrag] = useState<{ from: number; to: number; fromX: number; moved: boolean }>();
  // Timeline create runs on the day scale only: a day is the unit a drag can select exactly.
  const timelineCreate = !!onTimelineCreate && !readonly && scale === "day" && !!range.start;
  const dayAt = (clientX: number) => {
    const chart = chartRegion.current?.querySelector(".wx-chart") as HTMLElement | null;
    return chart ? dayIndexAt(clientX, chart.getBoundingClientRect().left, chart.scrollLeft, dayCellWidth) : null;
  };
  // Empty timeline only: not a bar, a link, a milestone, a resize handle, or the grid to the left.
  const onEmptyTimeline = (target: EventTarget | null) => {
    const el = target as Element | null;
    if (!el?.closest || !el.closest(".wx-chart")) return false;
    return !el.closest(".wx-bar, .wx-link, .wx-milestone, .wx-summary, .wx-progress-marker, [class*='resize']");
  };
  const dayDate = (index: number) => fromDay(addDays(range.start!, index));
  const dragPreview = (() => {
        const chart = chartRegion.current?.querySelector(".wx-chart") as HTMLElement | null;
        const wrap = chartRegion.current;
        if (!drag?.moved || !chart || !wrap) return null;
        const { first, last } = dayRange(drag.from, drag.to);
        const offset = chart.getBoundingClientRect().left - wrap.getBoundingClientRect().left - chart.scrollLeft;
        const startDay = dayDate(first);
        const endDay = dayDate(last);
        return <div aria-hidden className="pointer-events-none absolute top-0 bottom-0 z-30 border-x border-primary bg-primary/15"
          style={{ left: offset + first * dayCellWidth, width: (last - first + 1) * dayCellWidth }}>
          <span className="absolute left-1 top-1 rounded bg-background px-1 text-xs shadow">{formatDate(startDay, profile)} – {formatDate(endDay, profile)}</span>
        </div>;
  })();

  const ref = useRef({ byId, readonly, onPatch, t, addDependency, removeDependency });
  ref.current = { byId, readonly, onPatch, t, addDependency, removeDependency };
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  // C-04: weekend and holiday columns follow the work schedule, the same rule the calendar uses.
  const schedule = useQuery({ queryKey: workScheduleKey, queryFn: loadWorkSchedule });
  const workingDays = new Set(schedule.data?.data.working_days ?? [1, 2, 3, 4, 5]);
  const holidayDays = new Set((schedule.data?.data.exceptions ?? []).map((entry) => entry.date));
  const offDay = (date: Date) => !workingDays.has(date.getDay()) || holidayDays.has(fromDay(date));
  const locale = i18n.language;
  const scales = useMemo(() => {
    const label = (options: Intl.DateTimeFormatOptions) => (date: Date) => new Intl.DateTimeFormat(locale, options).format(date);
    if (scale === "day") return [{ unit: "month", step: 1, format: label({ month: "long", year: "numeric" }) }, { unit: "day", step: 1, format: label({ day: "numeric" }) }];
    if (scale === "week") return [{ unit: "month", step: 1, format: label({ month: "long", year: "numeric" }) }, { unit: "week", step: 1, format: label({ day: "numeric", month: "short" }) }];
    if (scale === "quarter") return [{ unit: "year", step: 1, format: label({ year: "numeric" }) }, { unit: "month", step: 3, format: (date: Date) => `Q${Math.floor(date.getMonth() / 3) + 1}` }];
    if (scale === "year") return [{ unit: "year", step: 1, format: label({ year: "numeric" }) }, { unit: "month", step: 1, format: label({ month: "narrow" }) }];
    return [{ unit: "year", step: 1, format: label({ year: "numeric" }) }, { unit: "month", step: 1, format: label({ month: "short" }) }];
  }, [scale, locale]);

  /** Both drag/resize and the keyboard end here: nothing changes on screen until the server answers. */
  function change(id: string, start: Date, finish: Date) {
    const { byId, readonly, onPatch, t } = ref.current;
    const item = byId.get(id);
    if (!item) return;
    if (readonly || !item.my_capabilities.includes("edit")) {
      toast.error(t("views.gantt.noEdit", { name: item.title }));
      return;
    }
    const next = { start_date: fromDay(start), due_date: fromDay(finish) };
    if (next.start_date !== item.start_date || next.due_date !== item.due_date) void onPatch(item, next);
  }
  function init(instance: IApi) {
    chart.current = instance;
    instance.on("resize-column", (event: { id: string | number; width: number }) => {
      setColWidths((current) => ({ ...current, [String(event.id)]: Math.max(40, Math.round(event.width)) }));
    });
    instance.on("resize-grid", (event: { width: number; inProgress?: boolean }) => {
      const width = Math.min(720, Math.max(280, event.width));
      setGridWidth(width);
      if (!event.inProgress) try { localStorage.setItem(`${ganttPrefKey}:width`, String(Math.round(width))); } catch { /* storage can be blocked */ }
    });
    // SVAR reports the edge it moved (`start` and/or `end`) and a whole-day `diff`; the dates themselves come from our item.
    instance.intercept("update-task", (event: { id: string | number; diff?: number; task?: ITask; inProgress?: boolean }) => {
      if (event.inProgress) return true; // live preview while dragging
      const item = ref.current.byId.get(String(event.id));
      const start = toDay(item?.start_date) ?? toDay(item?.due_date);
      const finish = toDay(item?.due_date) ?? start;
      const { diff = 0, task } = event;
      if (item && start && finish && diff && task) {
        const nextStart = task.start ? addDays(start, diff) : start;
        const nextFinish = item.is_milestone ? nextStart : task.end ? addDays(finish, diff) : finish;
        if (nextFinish >= nextStart) change(item.id, nextStart, nextFinish);
      }
      return false;
    });
    instance.intercept("add-link", (event: { link: { source?: string | number; target?: string | number; type?: string } }) => {
      const { link } = event;
      const type = link.type ? fromLink(link.type) : undefined;
      if (link.source !== undefined && link.target !== undefined && type && !ref.current.readonly) void ref.current.addDependency(String(link.source), String(link.target), type);
      return false;
    });
    instance.intercept("delete-link", (event: { id: string | number }) => {
      if (!ref.current.readonly) void ref.current.removeDependency(String(event.id));
      return false;
    });
    instance.on("select-task", (event: { id: string | number }) => setSelected(String(event.id)));
  }
  function keyboard(event: KeyboardEvent<HTMLDivElement>) {
    // F2 edits the selected task's name, as in a spreadsheet; double-click does the same for the mouse.
    if (event.key === "F2" && selected && !readonly) { event.preventDefault(); startEditing(selected); return; }
    const ids = tasks.map((task) => String(task.id));
    const at = selected ? ids.indexOf(selected) : -1;
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const next = ids[Math.min(ids.length - 1, Math.max(0, at + (event.key === "ArrowDown" ? 1 : -1)))];
      if (next) { setSelected(next); chart.current?.exec("select-task", { id: next }); }
    } else if ((event.key === "ArrowLeft" || event.key === "ArrowRight") && selected) {
      event.preventDefault();
      const item = byId.get(selected);
      const start = toDay(item?.start_date) ?? toDay(item?.due_date);
      const finish = toDay(item?.due_date) ?? start;
      if (!item || !start || !finish) return;
      const delta = event.key === "ArrowRight" ? 1 : -1;
      if (event.shiftKey) {
        if (!item.is_milestone && addDays(finish, delta) >= start) change(item.id, start, addDays(finish, delta));
      } else change(item.id, addDays(start, delta), addDays(finish, delta));
    } else if (event.key === "Enter" && selected && byId.get(selected)) onOpen(byId.get(selected)!);
  }

  const current = selected ? byId.get(selected) : undefined;
  const Theme = document.documentElement.classList.contains("dark") ? WillowDark : Willow;
  useEffect(() => {
    const region = chartRegion.current;
    if (!region) return;
    const taskParents = new Map(tasks.map((task) => [String(task.id), String(task.parent ?? 0)]));
    const parentIds = new Set(taskParents.values());
    const syncGridSemantics = () => {
      // SVAR reports viewport-relative row indices starting at zero, including after virtualization.
      for (const grid of region.querySelectorAll(".wx-table-box")) {
        if (grid.getAttribute("role") !== "treegrid") grid.setAttribute("role", "treegrid");
        grid.removeAttribute("aria-rowcount");
        for (const row of grid.querySelectorAll("[role=row][data-id]")) {
          row.removeAttribute("aria-rowindex");
          const id = row.getAttribute("data-id")?.replace(/^:/, "") ?? "";
          if (!parentIds.has(id) && !byId.get(id)?.child_count) row.removeAttribute("aria-expanded");
          let level = 1;
          for (let parent = taskParents.get(id); parent && taskParents.has(parent); parent = taskParents.get(parent)) level++;
          if (row.getAttribute("aria-level") !== String(level)) row.setAttribute("aria-level", String(level));
          const label = row.querySelector<HTMLElement>(".wx-text, .gantt-row-cell > span");
          if (label?.textContent?.trim()) label.title = label.textContent.trim();
        }
      }
    };
    syncGridSemantics();
    const observer = new MutationObserver(syncGridSemantics);
    observer.observe(region, { subtree: true, childList: true, attributes: true,
      attributeFilter: ["data-id", "aria-rowindex", "aria-rowcount", "aria-expanded"] });
    return () => observer.disconnect();
  }, [tasks, byId]);

  // Task numbers follow the order the grid shows the rows; predecessors and duration come from the same data.
  const numbers = useMemo(() => new Map(tasks.map((task, index) => [String(task.id), index + 1])), [tasks]);
  const dayCount = (task: ITask) => (task.start && task.end ? Math.round((task.end.getTime() - task.start.getTime()) / 86_400_000) : null);
  const predecessorList = (id: string) => deps.filter((dep) => String(dep.successor_id) === id)
    .map((dep) => ({ id: String(dep.id), predecessorId: String(dep.predecessor_id), number: numbers.get(String(dep.predecessor_id)) ?? 0, type: dep.type as DependencyType }));
  const [editingId, setEditingId] = useState<string>();
  const [predOpenId, setPredOpenId] = useState<string>();
  // Column widths the user dragged to, kept per view in this browser (SVAR reports each drag through resize-column).
  const colWidthsKey = `${ganttPrefKey}:columns`;
  const [colWidths, setColWidths] = useState<Record<string, number>>(() => { try { return JSON.parse(localStorage.getItem(colWidthsKey) ?? "{}") as Record<string, number>; } catch { return {}; } });
  const widthOf = (id: string, fallback: number) => colWidths[id] ?? fallback;
  useEffect(() => { try { localStorage.setItem(colWidthsKey, JSON.stringify(colWidths)); } catch { /* storage can be blocked */ } }, [colWidths, colWidthsKey]);
  const [nameDraft, setNameDraft] = useState("");
  // Starts a name edit with the current title in the draft.
  function startEditing(id: string) {
    setNameDraft(byId.get(id)?.title ?? "");
    setEditingId(id);
  }
  function saveTitle(id: string, title: string) {
    const item = byId.get(id);
    if (!item || readonly || !item.my_capabilities.includes("edit")) { toast.error(t("views.gantt.noEdit", { name: item?.title ?? "" })); return; }
    void onPatch(item, { title });
  }
  const saveRef = useRef(saveTitle);
  saveRef.current = saveTitle;
  // Memoised so the grid keeps its cells (and an open name editor) between renders.
  const gridColumns = useMemo(() => [
    { id: "index", header: t("views.gantt.index"), width: widthOf("index", 48), resize: true, cell: ({ row }: { row: ITask }) => <span className="tabular-nums text-muted-foreground">{numbers.get(String(row.id)) ?? ""}</span> },
    { id: "text", header: t("views.gantt.name"), flexgrow: 1, width: widthOf("text", 240), resize: true, cell: ({ row }: { row: ITask }) => <NameCell row={row} editing={editingId === String(row.id)} draft={nameDraft} canEdit={!readonly} onStart={startEditing} onDraft={setNameDraft} onEnd={() => setEditingId(undefined)} onSave={(id, title) => saveRef.current(id, title)} /> },
    ...(hasColumn("predecessors") ? [{ id: "predecessors", header: t("views.gantt.predecessors"), width: widthOf("predecessors", 120), resize: true,
      cell: ({ row }: { row: ITask }) => <PredecessorCell row={row} open={predOpenId === String(row.id)} onOpenChange={(next) => setPredOpenId(next ? String(row.id) : undefined)} options={tasks} value={predecessorList(String(row.id))} readonly={readonly} busy={busy}
        onAdd={(predecessor, successor, type) => void addDependency(predecessor, successor, type)} onRemove={(id) => void removeDependency(id)} onType={(id, type) => void changeDependency(id, type)} /> }] : []),
    ...(hasColumn("duration") ? [{ id: "duration", header: t("views.gantt.duration"), width: widthOf("duration", 72), resize: true, cell: ({ row }: { row: ITask }) => { const n = dayCount(row); return <span className="tabular-nums">{n ? t("views.gantt.days", { count: n }) : ""}</span>; } }] : []),
    ...(hasColumn("status") ? [{ id: "status_name", header: t("views.columns.status"), width: widthOf("status_name", 120), resize: true, cell: StatusCell }] : []),
    ...(hasColumn("assignee") ? [{ id: "assignee_names", header: t("items.assignees"), width: widthOf("assignee_names", 140), resize: true }] : []),
    ...(hasColumn("start_date") ? [{ id: "start_text", header: t("views.start_date"), width: widthOf("start_text", 112), resize: true }] : []),
    ...(hasColumn("due_date") ? [{ id: "due_text", header: t("views.due_date"), width: widthOf("due_text", 112), resize: true }] : []),
    ...(baselineId ? [{ id: "baseline", header: t("views.gantt.baselineDates"), width: 150 }, { id: "variance", header: t("views.gantt.variance"), width: 90 }] : []),
    ...shownFields.map((field) => ({ id: `cf_${field.id}`, header: field.name, width: 120 })),
    ...(onRowAdd && !readonly ? [{ id: "row_actions", header: t("views.actions"), width: 44, cell: RowActionsCell }] : []),
  ], [numbers, deps, tasks, columns, baselineId, shownFields, readonly, busy, editingId, predOpenId, nameDraft, colWidths, onRowAdd, t]);

  const controls = <>
      <Button size="sm" variant="outline" onClick={() => setAllOpen(!allOpen)}>{allOpen ? <ChevronsDownUp /> : <ChevronsUpDown />}{t(allOpen ? "views.gantt.collapseAll" : "views.gantt.expandAll")}</Button>
      <Button size="sm" variant="outline" onClick={() => chartRegion.current?.querySelector(".gantt-today")?.scrollIntoView({ inline: "center", block: "nearest" })}>{t("views.gantt.today")}</Button>
      {/* Fit project: brings the earliest scheduled bar to the left edge (no zoom; SVAR free has no fit-to-range). */}
      {/* C-04: brings the selected bar into view; nothing changes on screen until a bar is selected. */}
      <Button size="sm" variant="outline" disabled={!selected} onClick={() => chartRegion.current?.querySelector(`.wx-bar[data-id=":${selected}"]`)?.scrollIntoView({ inline: "center", block: "nearest" })}>{t("views.gantt.showSelected")}</Button>
      <Button size="sm" variant="outline" disabled={!tasks.some((task) => task.start)} onClick={() => {
        const earliest = tasks.filter((task) => task.start).sort((a, b) => a.start!.getTime() - b.start!.getTime())[0];
        chartRegion.current?.querySelector(`.wx-bar[data-id=":${earliest?.id}"]`)?.scrollIntoView({ inline: "start", block: "nearest" });
      }}>{t("views.gantt.fit")}</Button>
      {!!parentId && <>
        <label className="flex items-center gap-1 text-sm">{t("views.gantt.baseline")}
          <select value={baselineId} onChange={(event) => setBaselineId(event.target.value)} className="rounded border bg-background p-1">
            <option value="">{t("views.gantt.baselineNone")}</option>
            {baselines.data?.items.map((b) => <option key={b.id} value={b.id}>{b.name}</option>)}
          </select></label>
        {!readonly && <DropdownMenu>
          {/* Saving a baseline and sharing snapshots are occasional, so they sit in one menu instead of the toolbar row. */}
          <DropdownMenuTrigger render={<Button size="sm" variant="outline" />}><MoreHorizontal />{t("views.gantt.more")}</DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-48">
            {!naming && <DropdownMenuItem onClick={() => setNaming(true)}>{t("views.gantt.baselineSave")}</DropdownMenuItem>}
            <DropdownMenuItem onClick={() => setSharing(true)}>{t("views.snapshot.open")}</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>}
        <GanttSnapshotDialog projectId={parentId} open={sharing} onOpenChange={setSharing} />
        {!readonly && naming && <form className="flex items-end gap-1" onSubmit={(event) => {
          event.preventDefault();
          const name = String(new FormData(event.currentTarget).get("name") ?? "");
          void mutate(async () => {
            const made = await api<Baseline>(`/api/v1/items/${parentId}/baselines`, { method: "POST", body: JSON.stringify({ name }) });
            await qc.invalidateQueries({ queryKey: ["baselines", parentId] });
            setBaselineId(made.id);
            setNaming(false);
          });
        }}>
          <label className="text-sm">{t("views.gantt.baselineName")}<input name="name" required maxLength={120} className="block rounded border bg-background p-1" /></label>
          <Button type="submit" size="sm" disabled={busy}>{t("views.gantt.baselineCreate")}</Button>
        </form>}
      </>}
      {undated > 0 && <span className="text-xs font-medium text-muted-foreground">{t("views.gantt.undated", { count: undated })}</span>}
  </>;
  return <div className="space-y-2">
    <div className="flex flex-wrap items-center gap-2">{renderToolbar ? renderToolbar(controls) : controls}</div>
    <div className="relative">
    <div ref={chartRegion} role="application" aria-describedby="gantt-keys" aria-label={t("views.gantt.label")} tabIndex={0}
      onKeyDown={(event) => { if (drag && event.key === "Escape") { setDrag(undefined); return; } keyboard(event); }}
      title={onTimelineCreate && !readonly && scale !== "day" ? t("views.gantt.dayOnlyCreate") : undefined}
      onDoubleClick={(event) => {
        // On Week and Month the timeline does not create; say why instead of doing nothing (the row menu and + Add still work).
        if (onTimelineCreate && !readonly && scale !== "day" && onEmptyTimeline(event.target)) { toast.info(t("views.gantt.dayOnlyCreate")); return; }
        if (!timelineCreate || !onEmptyTimeline(event.target)) return;
        const index = dayAt(event.clientX);
        if (index !== null) { const day = dayDate(index); onTimelineCreate!(day, day); }
      }}
      onPointerDown={(event) => {
        if (!timelineCreate || event.button !== 0 || !onEmptyTimeline(event.target)) return;
        const index = dayAt(event.clientX);
        if (index !== null) setDrag({ from: index, to: index, fromX: event.clientX, moved: false });
      }}
      onPointerMove={(event) => {
        if (!drag) return;
        const index = dayAt(event.clientX);
        if (index === null) return;
        setDrag((current) => current && { ...current, to: index, moved: current.moved || isDragStep(current.fromX, event.clientX) });
      }}
      onPointerUp={() => {
        if (drag?.moved) { const { first, last } = dayRange(drag.from, drag.to); onTimelineCreate!(dayDate(first), dayDate(last)); }
        setDrag(undefined);
      }}
      onPointerLeave={() => setDrag(undefined)}
      onPointerCancel={() => setDrag(undefined)}
      className="gantt-chart relative h-[60vh] min-h-72 overflow-hidden rounded border focus-visible:outline-ring [&_.wx-theme]:h-full">
      {dragPreview}
      <RowActionsContext.Provider value={onRowAdd && !readonly ? { tasks, spaceId, byId, onRowAdd } : null}>
      <Theme>
        <Gantt taskTemplate={BarLabel} tasks={shownTasks} links={links} {...range} readonly={readonly} init={init} scales={scales} gridWidth={gridWidth} cellWidth={scaleGeometry[scale].cell}
          highlightTime={(date, unit) => unit !== "day" ? "" : date.getTime() === today.getTime() ? "gantt-today" : offDay(date) ? "gantt-off" : ""} schedule={{ auto: false }} unscheduledTasks
          columns={gridColumns} />
      </Theme>
      </RowActionsContext.Provider>
    </div>
      {/* Floating over the chart, top right: scale and zoom, as in the Wrike layout. Nothing here changes the grid or the rows. */}
      <div className="absolute right-2 top-2 z-30 flex items-center gap-1 rounded-md border bg-card p-1 shadow-sm">
        <DropdownMenu>
          <DropdownMenuTrigger render={<Button size="sm" variant="ghost" aria-label={t("views.gantt.scale")} />}>{t(`views.gantt.${scale}`)}<ChevronDown /></DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-36">
            {scaleOptions.map((value) => <DropdownMenuItem key={value} aria-current={scale === value ? "true" : undefined} onClick={() => setScale(value)}>{t(`views.gantt.${value}`)}</DropdownMenuItem>)}
          </DropdownMenuContent>
        </DropdownMenu>
        <Button size="icon-sm" variant="ghost" aria-label={t("views.gantt.zoomOut")} disabled={scale === "year"} onClick={() => setScale(scaleOptions[scaleOptions.indexOf(scale) - 1]!)}><Minus /></Button>
        <Button size="icon-sm" variant="ghost" aria-label={t("views.gantt.zoomIn")} disabled={scale === "day"} onClick={() => setScale(scaleOptions[scaleOptions.indexOf(scale) + 1]!)}><Plus /></Button>
      </div>
    </div>
    {onQuickCreate && !readonly && <form className="flex items-center gap-2 border-t px-3 py-2" onSubmit={(event) => {
      event.preventDefault();
      const input = event.currentTarget.elements.namedItem("newTask") as HTMLInputElement;
      const title = input.value.trim();
      if (!title) return;
      input.value = "";
      void onQuickCreate(title, crypto.randomUUID());
    }}>
      <Plus className="size-4 shrink-0 text-primary" aria-hidden />
      <input name="newTask" maxLength={500} aria-label={t("views.gantt.addTask")} placeholder={t("views.gantt.addTask")} className="min-w-0 flex-1 bg-transparent text-sm outline-none" />
    </form>}
    {/* The keys stay on screen under the chart: the chart above keeps its height, so timeline positions do not move. */}
    <p id="gantt-keys" className="text-xs text-muted-foreground">{readonly ? t("views.gantt.readonly") : t("views.gantt.keys")}
      {undated > 0 && <span className="ml-3 font-medium text-foreground">{t("views.gantt.undated", { count: undated })}</span>}</p>
    {(outside.left.length > 0 || outside.right.length > 0) && <div className="flex flex-wrap gap-2">
      {outside.left.length > 0 && <Button size="sm" variant="outline" onClick={() => jumpOutside("left")}><ChevronLeft />{t("views.gantt.offLeft", { count: outside.left.length })}</Button>}
      {outside.right.length > 0 && <Button size="sm" variant="outline" onClick={() => jumpOutside("right")}>{t("views.gantt.offRight", { count: outside.right.length })}<ChevronRight /></Button>}
    </div>}
    {onAdd && <Button variant="ghost" size="sm" className="text-primary" onClick={onAdd}><Plus />{t("views.add")}</Button>}
    <p role="status" className="sr-only">{current ? (current.start_date || current.due_date
      ? t("views.gantt.selected", { name: current.title, start: formatDate(current.start_date ?? current.due_date, profile), end: formatDate(current.due_date ?? current.start_date, profile) })
      : t("views.gantt.unscheduled", { name: current.title })) : ""}</p>
  </div>;
}

