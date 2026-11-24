import { createContext, useContext, useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { Gantt, Willow, WillowDark, type IApi, type ITask } from "@svar-ui/react-gantt";
import "@svar-ui/react-gantt/style.css";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ChevronsDownUp, ChevronsUpDown, Keyboard, Minus, MoreHorizontal, Plus } from "lucide-react";
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
type Scale = "day" | "week" | "month";
const scaleOptions: Scale[] = ["day", "week", "month"];
const gridPresets = ["minimal", "standard", "detail"] as const;
type GridPreset = (typeof gridPresets)[number];
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

export function ItemGantt({ initialOpen, onOpenChange, items, spaceId, parentId, readonly, customFields, columns, onOpen, onPatch, onAdd, onRowAdd, onTimelineCreate }: {
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
  /** Batch 3B: add a subitem, or a Task above/below a row (previousId is the row above it among its siblings, or null). */
  onRowAdd?: (kind: GanttRowAdd, item: Item, previousId: string | null) => void;
  /** Batch 3C: a double-click or a drag on empty timeline days opens Quick Create with that range (YYYY-MM-DD, inclusive). */
  onTimelineCreate?: (start: string, finish: string) => void;
  onPatch: (item: Item, update: { start_date: string | null; due_date: string | null }) => Promise<void>;
}) {
  const { t, i18n } = useTranslation();
  const profile = useProfile();
  const [scale, setScale] = useState<Scale>("day");
  const [selected, setSelected] = useState<string>();
  const [baselineId, setBaselineId] = useState("");
  const ganttPrefKey = `pms:gantt-grid:${spaceId}:${parentId ?? "root"}`;
  const [gridWidth, setGridWidth] = useState(() => { try { return Math.min(720, Math.max(280, Number(localStorage.getItem(`${ganttPrefKey}:width`)) || 480)); } catch { return 480; } });
  const [gridPreset, setGridPreset] = useState<GridPreset>(() => {
    const configuredDefault = columns.some((column) => column.startsWith("field:")) ? "detail" : "standard";
    try {
      const value = localStorage.getItem(`${ganttPrefKey}:preset`);
      return value === "minimal" || value === "detail" || value === "standard" ? value : configuredDefault;
    } catch { return configuredDefault; }
  });
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
  const shownFields = useMemo(() => gridPreset === "detail" ? customFields.filter((field) => columns.includes(fieldColumn(field.id))) : [], [customFields, columns, gridPreset]);
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
  const range = useMemo(() => {
    const dated = tasks.filter((task) => task.start && task.end);
    if (!dated.length) return {};
    const first = Math.min(...dated.map((task) => task.start!.getTime()));
    const last = Math.max(...dated.map((task) => task.end!.getTime()));
    return { start: addDays(new Date(first), -3), end: addDays(new Date(last), 21) };
  }, [tasks]);
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
    return scale === "day"
      ? [{ unit: "month", step: 1, format: label({ month: "long", year: "numeric" }) }, { unit: "day", step: 1, format: label({ day: "numeric" }) }]
      : scale === "week"
        ? [{ unit: "month", step: 1, format: label({ month: "long", year: "numeric" }) }, { unit: "week", step: 1, format: label({ day: "numeric", month: "short" }) }]
        : [{ unit: "year", step: 1, format: label({ year: "numeric" }) }, { unit: "month", step: 1, format: label({ month: "short" }) }];
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

  return <div className="space-y-2">
    <div className="flex flex-wrap items-center gap-2">
      <div role="group" aria-label={t("views.gantt.scale")} className="flex gap-1">
        {scaleOptions.map((value) => <Button key={value} size="sm" variant={scale === value ? "default" : "outline"}
          aria-pressed={scale === value} onClick={() => setScale(value)}>{t(`views.gantt.${value}`)}</Button>)}
      </div>
      <label className="flex items-center gap-1 text-sm"><span>{t("views.gantt.gridPreset")}</span>
        <select value={gridPreset} className="rounded border bg-background p-1" onChange={(event) => {
          const value = event.target.value as GridPreset;
          setGridPreset(value);
          try { localStorage.setItem(`${ganttPrefKey}:preset`, value); } catch { /* storage can be blocked */ }
        }}>
          {gridPresets.map((value) => <option key={value} value={value}>{t(`views.gantt.gridPresets.${value}`)}</option>)}
        </select>
      </label>
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
        {!readonly && !naming && <Button size="sm" variant="outline" onClick={() => setNaming(true)}>{t("views.gantt.baselineSave")}</Button>}
        {!readonly && <Button size="sm" variant="outline" onClick={() => setSharing(true)}>{t("views.snapshot.open")}</Button>}
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
      <Button size="icon-sm" variant="outline" aria-label={t("views.gantt.zoomOut")} disabled={scale === "month"} onClick={() => setScale(scaleOptions[scaleOptions.indexOf(scale) + 1]!)}><Minus /></Button>
      <Button size="icon-sm" variant="outline" aria-label={t("views.gantt.zoomIn")} disabled={scale === "day"} onClick={() => setScale(scaleOptions[scaleOptions.indexOf(scale) - 1]!)}><Plus /></Button>
      <Button size="icon-sm" variant="ghost" aria-label={t("views.gantt.keysLabel")} title={readonly ? t("views.gantt.readonly") : t("views.gantt.keys")}><Keyboard /></Button>
    </div>
    <p id="gantt-keys" className="sr-only">{readonly ? t("views.gantt.readonly") : t("views.gantt.keys")}</p>
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
        <Gantt taskTemplate={BarLabel} tasks={shownTasks} links={links} {...range} readonly={readonly} init={init} scales={scales} gridWidth={gridWidth} cellWidth={scale === "day" ? 36 : scale === "week" ? 80 : 90}
          highlightTime={(date, unit) => unit !== "day" ? "" : date.getTime() === today.getTime() ? "gantt-today" : offDay(date) ? "gantt-off" : ""} schedule={{ auto: false }} unscheduledTasks
          columns={[{ id: "text", header: t("views.gantt.name"), flexgrow: 1, width: 240 }, ...(gridPreset !== "minimal" && hasColumn("status") ? [{ id: "status_name", header: t("views.columns.status"), width: 120, cell: StatusCell }] : []),
            ...(gridPreset === "detail" && hasColumn("start_date") ? [{ id: "start_text", header: t("views.start_date"), width: 112 }] : []),
            ...(gridPreset === "detail" && hasColumn("due_date") ? [{ id: "due_text", header: t("views.due_date"), width: 112 }] : []),
            ...(baselineId ? [{ id: "baseline", header: t("views.gantt.baselineDates"), width: 150 }, { id: "variance", header: t("views.gantt.variance"), width: 90 }] : []),
            ...shownFields.map((field) => ({ id: `cf_${field.id}`, header: field.name, width: 120 })),
            ...(onRowAdd && !readonly ? [{ id: "row_actions", header: t("views.actions"), width: 44, cell: RowActionsCell }] : [])]} />
      </Theme>
      </RowActionsContext.Provider>
    </div>
    {onAdd && <Button variant="ghost" size="sm" className="text-primary" onClick={onAdd}><Plus />{t("views.add")}</Button>}
    {current && <section aria-label={t("views.gantt.dependencies", { name: current.title })} className="space-y-2 rounded border p-3">
      <h2 className="font-medium">{t("views.gantt.dependencies", { name: current.title })}</h2>
      <ul className="space-y-1">
        {deps.filter((dep) => dep.successor_id === current.id).map((dep) => <li key={dep.id} className="flex flex-wrap items-center gap-2">
          <span>{byId.get(dep.predecessor_id)?.title ?? dep.predecessor_id}</span>
          <select aria-label={t("views.gantt.depType", { name: byId.get(dep.predecessor_id)?.title })} value={dep.type} disabled={readonly || busy}
            className="rounded border bg-background p-1" onChange={(event) => void changeDependency(dep.id, event.target.value as DependencyType)}>
            {dependencyTypes.map((type) => <option key={type} value={type}>{type}</option>)}
          </select>
          {!readonly && <Button size="sm" variant="outline" disabled={busy} onClick={() => void removeDependency(dep.id)}
            aria-label={t("views.gantt.removeDep", { name: byId.get(dep.predecessor_id)?.title })}>{t("views.gantt.remove")}</Button>}
        </li>)}
        {!deps.some((dep) => dep.successor_id === current.id) && <li className="text-sm text-muted-foreground">{t("views.gantt.noDeps")}</li>}
      </ul>
      {!readonly && <form className="flex flex-wrap items-end gap-2" onSubmit={(event) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        const predecessor = String(data.get("predecessor") ?? "");
        if (predecessor) void addDependency(predecessor, current.id, String(data.get("type")) as DependencyType);
      }}>
        <label className="space-y-1"><span className="block text-sm">{t("views.gantt.predecessor")}</span>
          <select name="predecessor" required defaultValue="" className="rounded border bg-background p-1">
            <option value="" disabled />
            {[...byId.values()].filter((item) => item.id !== current.id).map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}
          </select></label>
        <label className="space-y-1"><span className="block text-sm">{t("views.gantt.type")}</span>
          <select name="type" defaultValue="FS" className="rounded border bg-background p-1">
            {dependencyTypes.map((type) => <option key={type} value={type}>{type}</option>)}
          </select></label>
        <Button type="submit" size="sm" disabled={busy}>{t("views.gantt.addDep")}</Button>
      </form>}
    </section>}
    <p role="status" className="sr-only">{current ? (current.start_date || current.due_date
      ? t("views.gantt.selected", { name: current.title, start: formatDate(current.start_date ?? current.due_date, profile), end: formatDate(current.due_date ?? current.start_date, profile) })
      : t("views.gantt.unscheduled", { name: current.title })) : ""}</p>
  </div>;
}

