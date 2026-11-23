import { useEffect, useLayoutEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { CalendarClock, CalendarDays, Diamond, ChevronDown, ChevronLeft, ChevronRight, CircleHelp, Plus, Settings2, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuGroup, DropdownMenuLabel, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { api } from "@/lib/api";
import { loadWorkSchedule, workScheduleKey } from "@/lib/work-schedule";
import { addDays, daysBetween, fromDay, toDay } from "@/lib/calendar-days";
import { calendarEntries, layoutDays, type Bar } from "@/lib/calendar-layout";
import { formatDate, useProfile } from "@/lib/profile";
import type { Item } from "@/lib/items";
import { tintOf } from "@/components/ui/status-chip";
import { TypeGlyph } from "@/components/type-badge";
import { ItemChartMenu } from "@/components/item-chart-menu";
import type { ItemStatus, ItemUpdate } from "@/lib/items";

type Update = ItemUpdate;

const calendarModes = ["month", "week", "quarter", "year"] as const;
type CalendarMode = (typeof calendarModes)[number];
const scheduleModes = ["boundaries", "all"] as const;
const monthFormat: Intl.DateTimeFormatOptions = { month: "long", year: "numeric" };
const shortMonthFormat: Intl.DateTimeFormatOptions = { month: "short" };
const weekdayFormat: Intl.DateTimeFormatOptions = { weekday: "short" };
const fullFormat: Intl.DateTimeFormatOptions = { dateStyle: "full" };
const startOf = (value: Item) => toDay(value.start_date) ?? toDay(value.due_date);
const endOf = (value: Item) => toDay(value.due_date) ?? toDay(value.start_date);
const schedulePreferenceKey = "pms:calendar:schedule-display";
const optionsPreferenceKey = "pms:calendar:options";
type CalendarOptions = { weekends: boolean; weekNumbers: boolean; colorBy: "status" | "type"; subitems: boolean; maxItems: number; density: "comfortable" | "compact" };
const defaultOptions: CalendarOptions = { weekends: true, weekNumbers: false, colorBy: "status", subitems: true, maxItems: 3, density: "comfortable" };
const typeTints: Record<string, string> = { project: "bg-purple-100 text-purple-900", folder: "bg-amber-100 text-amber-900", task: "bg-sky-100 text-sky-900" };
const densityTints = ["", "bg-primary/10", "bg-primary/25", "bg-primary/45", "bg-primary/70 text-primary-foreground"];
const weekNumber = (date: Date) => {
  const value = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()));
  value.setUTCDate(value.getUTCDate() + 4 - (value.getUTCDay() || 7));
  return Math.ceil((((value.getTime() - Date.UTC(value.getUTCFullYear(), 0, 1)) / 86_400_000) + 1) / 7);
};

export function ItemCalendar({ items, statuses, readonly, onOpen, onPatch, onDelete, onCreateOn, renderToolbar }: {
  items: Item[]; readonly: boolean; onOpen: (item: Item) => void; onPatch: (item: Item, update: Update) => Promise<void>;
  statuses: ItemStatus[]; onDelete: (item: Item) => void;
  /** Create a Task on a working day (YYYY-MM-DD). Absent when the user cannot create here. */
  onCreateOn?: (day: string) => void;
  renderToolbar?: (controls: ReactNode) => ReactNode;
}) {
  const { t, i18n } = useTranslation();
  const [selectedDay, setSelectedDay] = useState<Date>();
  const [pickerOpen, setPickerOpen] = useState(false);
  const [unscheduledOpen, setUnscheduledOpen] = useState(false);
  const [helpOpen, setHelpOpen] = useState(false);
  const calendarRef = useRef<HTMLDivElement>(null);
  const [calendarHeight, setCalendarHeight] = useState(480);
  const profile = useProfile();
  const initialPeriod = () => {
    const picked = toDay(new URL(window.location.href).searchParams.get("calDate"));
    const now = picked ?? new Date();
    return new Date(now.getFullYear(), now.getMonth(), now.getDate());
  };
  const [month, setMonth] = useState(() => { const date = initialPeriod(); return new Date(date.getFullYear(), date.getMonth(), 1); });
  // C-03: Month or Week. Week mode keeps its own anchor day, so moving by week never changes the month view.
  const [mode, setMode] = useState<CalendarMode>(() => {
    const value = new URL(window.location.href).searchParams.get("calMode") as CalendarMode | null;
    return value && calendarModes.includes(value) ? value : "month";
  });
  const [scheduleMode, setScheduleMode] = useState<"boundaries" | "all">(() => {
    try { return localStorage.getItem(schedulePreferenceKey) === "boundaries" ? "boundaries" : "all"; }
    catch { return "all"; }
  });
  const [options, setOptions] = useState<CalendarOptions>(() => {
    try { return { ...defaultOptions, ...JSON.parse(localStorage.getItem(optionsPreferenceKey) ?? "{}") }; }
    catch { return defaultOptions; }
  });
  useEffect(() => { try { localStorage.setItem(schedulePreferenceKey, scheduleMode); } catch { /* storage can be blocked */ } }, [scheduleMode]);
  useEffect(() => { try { localStorage.setItem(optionsPreferenceKey, JSON.stringify(options)); } catch { /* storage can be blocked */ } }, [options]);
  useLayoutEffect(() => {
    const fit = () => setCalendarHeight(Math.max(240, window.innerHeight - (calendarRef.current?.getBoundingClientRect().top ?? 0) - 16));
    fit();
    window.addEventListener("resize", fit);
    return () => window.removeEventListener("resize", fit);
  }, [mode, unscheduledOpen]);
  const [weekDay, setWeekDay] = useState(initialPeriod);
  const [yearDraft, setYearDraft] = useState(() => String(initialPeriod().getFullYear()));
  const [drag, setDrag] = useState<{ id: string; from?: Date }>();
  const schedule = useQuery({ queryKey: workScheduleKey, queryFn: loadWorkSchedule });
  const holidays = useMemo(() => new Map((schedule.data?.data.exceptions ?? []).map((entry) => [entry.date, entry.name])), [schedule.data]);
  const working = new Set(schedule.data?.data.working_days ?? [1, 2, 3, 4, 5]);
  const weekStart = profile?.week_start === "sunday" ? 0 : profile?.week_start === "saturday" ? 6 : 1;
  const locale = i18n.language;
  const fmt = (options: Intl.DateTimeFormatOptions) => new Intl.DateTimeFormat(locale, options);
  const itemRange = (item: Item) => [item.start_date, item.due_date].filter(Boolean).map((date) => formatDate(date, profile)).join(" – ");

  function showPeriod(nextMode: CalendarMode, date: Date, replace = false) {
    const local = new Date(date.getFullYear(), date.getMonth(), date.getDate());
    setMode(nextMode);
    setMonth(new Date(local.getFullYear(), local.getMonth(), 1));
    setWeekDay(local);
    setYearDraft(String(local.getFullYear()));
    const url = new URL(window.location.href);
    url.searchParams.set("calMode", nextMode);
    url.searchParams.set("calDate", fromDay(local));
    window.history[replace ? "replaceState" : "pushState"]({}, "", url);
  }
  useEffect(() => {
    const restore = () => {
      const url = new URL(window.location.href);
      const nextMode = url.searchParams.get("calMode") as CalendarMode | null;
      const date = toDay(url.searchParams.get("calDate")) ?? new Date();
      showPeriod(nextMode && calendarModes.includes(nextMode) ? nextMode : "month", date, true);
    };
    window.addEventListener("popstate", restore);
    return () => window.removeEventListener("popstate", restore);
  }, []);

  const periodMonth = mode === "quarter" ? new Date(month.getFullYear(), Math.floor(month.getMonth() / 3) * 3, 1)
    : mode === "year" ? new Date(month.getFullYear(), 0, 1) : month;
  const periodEnd = mode === "quarter" ? new Date(periodMonth.getFullYear(), periodMonth.getMonth() + 3, 0)
    : mode === "year" ? new Date(periodMonth.getFullYear(), 11, 31) : new Date(month.getFullYear(), month.getMonth() + 1, 0);
  const anchor = mode === "week" ? weekDay : periodMonth;
  const first = addDays(anchor, -((anchor.getDay() - weekStart + 7) % 7));
  const weeks = mode === "week" ? 1 : Math.ceil((daysBetween(first, periodEnd) + 1) / 7);
  const days = Array.from({ length: weeks * 7 }, (_, index) => addDays(first, index));
  const itemById = new Map(items.map((item) => [item.id, item]));
  const isSubitem = (item: Item) => item.locations.some((location) => location.parent_item_id && itemById.get(location.parent_item_id)?.type === "task");
  const visibleItems = options.subitems ? items : items.filter((item) => !isSubitem(item));
  const scheduled = visibleItems.filter((item) => startOf(item));
  const allEntries = calendarEntries(scheduled.map((item) => ({ id: item.id, start: startOf(item)!, end: endOf(item)! })), false);
  const boundaryEntries = calendarEntries(scheduled.map((item) => ({ id: item.id, start: startOf(item)!, end: endOf(item)! })), true);
  const visible = (entry: { start: Date; end: Date }) => entry.start <= days[days.length - 1] && entry.end >= days[0];
  const boundariesAreEmpty = mode !== "week" && scheduleMode === "boundaries" && !boundaryEntries.some(visible) && allEntries.some(visible);
  useEffect(() => { if (boundariesAreEmpty) setScheduleMode("all"); }, [boundariesAreEmpty]);
  const entries = mode !== "week" && scheduleMode === "boundaries" ? boundaryEntries : allEntries;
  const unscheduled = visibleItems.filter((item) => !startOf(item));
  const today = fromDay(new Date());

  function shift(item: Item, delta: number) {
    const start = toDay(item.start_date), due = toDay(item.due_date);
    void onPatch(item, { start_date: start ? fromDay(addDays(start, delta)) : null, due_date: due ? fromDay(addDays(due, delta)) : null });
  }
  function dropOn(day: Date) {
    const item = items.find((entry) => entry.id === drag?.id);
    const from = drag?.from;
    setDrag(undefined);
    if (!item || !item.my_capabilities.includes("edit") || readonly) return;
    if (!from) void onPatch(item, { start_date: fromDay(day), due_date: fromDay(day) }); // from the Unscheduled list
    else if (daysBetween(from, day)) shift(item, daysBetween(from, day));
  }
  function keyboard(event: KeyboardEvent<HTMLButtonElement>, item: Item) {
    const delta = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 }[event.key];
    if (!event.altKey || !delta || readonly || !item.my_capabilities.includes("edit") || !startOf(item)) return;
    event.preventDefault();
    shift(item, delta);
  }
  const chip = (item: Item, day?: Date) => {
    const canEdit = !readonly && item.my_capabilities.includes("edit");
    return <li key={item.id}>
      <button type="button" draggable={canEdit} onClick={() => onOpen(item)} onKeyDown={(event) => keyboard(event, item)}
        onDragStart={(event) => { event.dataTransfer.setData("text/plain", item.id); setDrag({ id: item.id, from: day }); }} onDragEnd={() => setDrag(undefined)}
        aria-label={item.start_date || item.due_date ? t("views.calendar.item", { title: item.title, start: formatDate(item.start_date ?? item.due_date, profile), end: formatDate(item.due_date ?? item.start_date, profile) }) : item.title}
        className={`flex w-full items-center gap-1 truncate rounded border-l-4 border-current px-1.5 py-0.5 text-left text-xs font-medium ${options.colorBy === "status" ? tintOf(item.status?.color) : typeTints[item.type]}`}><TypeGlyph item={item} className="size-3" /><span className="flex min-w-0 items-center gap-1">{item.is_milestone && <Diamond aria-hidden className="size-3 shrink-0" />}<span className="truncate">{item.title}</span></span></button>
    </li>;
  };
  const names = Array.from({ length: 7 }, (_, index) => addDays(first, index));
  const compactMonths = Array.from({ length: mode === "quarter" ? 3 : 12 }, (_, index) => new Date(periodMonth.getFullYear(), periodMonth.getMonth() + index, 1));
  const itemsOn = (day: Date) => scheduled.filter((item) => startOf(item)! <= day && endOf(item)! >= day);
  const barButton = (item: Item, bar: Bar, from: Date, showLabel: boolean) => {
    const canEdit = !readonly && item.my_capabilities.includes("edit");
    const boundary = bar.id.endsWith(":start") ? t("views.calendar.starts") : bar.id.endsWith(":end") ? t("views.calendar.deadline") : "";
    // C-01: the card opens on hover or keyboard focus of the bar; the bar's accessible description points at it.
    const cardId = `calendar-card-${item.id}-${bar.lane}-${bar.startCol}`;
    return <span className="group relative block">
      <button type="button" draggable={canEdit} onClick={() => onOpen(item)} onKeyDown={(event) => keyboard(event, item)} aria-describedby={cardId}
        onDragStart={(event) => { event.dataTransfer.setData("text/plain", item.id); setDrag({ id: item.id, from }); }}
        onDragEnd={() => setDrag(undefined)}
        className={`flex h-6 w-full items-center gap-1 truncate rounded border-current px-1.5 text-left text-xs font-medium ${options.colorBy === "status" ? tintOf(item.status?.color) : typeTints[item.type]} ${bar.continuesBefore ? "rounded-l-none border-l-0" : "border-l-4"} ${bar.continuesAfter ? "rounded-r-none" : ""}`}>
        {bar.continuesBefore && <span aria-hidden>‹</span>}{showLabel ? <><TypeGlyph item={item} className="size-3 shrink-0" />{boundary && <span className="shrink-0 text-[10px] font-semibold uppercase tracking-wide">{boundary}</span>}<span className="flex min-w-0 items-center gap-1">{item.is_milestone && <Diamond aria-hidden className="size-3 shrink-0" />}<span className="truncate">{item.title}</span></span></> : <span className="sr-only">{item.title}</span>}{bar.continuesAfter && <span aria-hidden className="ml-auto">›</span>}
      </button>
      <span className="absolute right-0.5 top-0.5 z-20"><ItemChartMenu item={item} statuses={statuses} readonly={readonly} onOpen={onOpen} onPatch={onPatch} onDelete={onDelete} /></span>
      <span role="tooltip" id={cardId} className="pointer-events-none invisible absolute left-0 top-full z-40 mt-1 w-64 space-y-1 rounded-md border bg-popover p-3 text-xs text-popover-foreground shadow-md group-focus-within:visible group-hover:visible">
        <span className="block font-medium">{item.title}</span>
        <span className="block">{t("views.calendar.cardType")}: {item.item_type?.name ?? t("items.type")}</span>
        <span className="block">{t("views.calendar.cardStatus")}: {item.status?.name ?? t("views.noStatus")}</span>
        <span className="block">{t("views.calendar.cardAssignees")}: {item.assignees.map((person) => person.name).join(", ") || t("items.chips.noAssignee")}</span>
        <span className="block">{t("views.calendar.cardDates")}: {itemRange(item) || t("items.chips.noDates")}</span>
        {item.type === "project" && item.progress_percent != null && <span className="block">{t("items.progress.project")}: {t("items.progress.percent", { value: item.progress_percent })}{item.progress_percent === 0 ? ` · ${t("items.progress.notStarted")}` : ""}</span>}
      </span>
    </span>;
  };

  const periodLabel = mode === "week" ? `${fmt({ day: "numeric", month: "short", year: "numeric" }).format(first)} – ${fmt({ day: "numeric", month: "short", year: "numeric" }).format(addDays(first, 6))}`
    : mode === "quarter" ? `${t("views.calendar.quarter", { value: Math.floor(periodMonth.getMonth() / 3) + 1 })} ${periodMonth.getFullYear()}`
    : mode === "year" ? String(periodMonth.getFullYear()) : fmt(monthFormat).format(month);
  const chooseYear = () => {
    const year = Number(yearDraft);
    if (Number.isInteger(year) && year >= 1900 && year <= 9999) showPeriod(mode, new Date(year, mode === "quarter" ? periodMonth.getMonth() : month.getMonth(), 1));
  };
  const picker = <Popover open={pickerOpen} onOpenChange={setPickerOpen}>
    <PopoverTrigger render={<Button size="sm" variant="outline" aria-label={t("views.calendar.choosePeriod")} />}><CalendarDays />{periodLabel}<ChevronDown /></PopoverTrigger>
    <PopoverContent align="center" className="w-80 space-y-3">
      <form className="flex items-end gap-2" onSubmit={(event) => { event.preventDefault(); chooseYear(); }}>
        <label className="flex-1 text-xs font-medium">{t("views.calendar.yearInput")}<Input type="number" min={1900} max={9999} value={yearDraft} onChange={(event) => setYearDraft(event.target.value)} className="mt-1" /></label>
        <Button type="submit" size="sm">{t("views.calendar.go")}</Button>
      </form>
      {mode === "week" ? <label className="block text-xs font-medium">{t("views.calendar.chooseDate")}<Input type="date" className="mt-1" value={fromDay(weekDay)} onChange={(event) => {
        const date = toDay(event.target.value); if (date) { showPeriod("week", date); setPickerOpen(false); }
      }} /></label> : <div className="grid grid-cols-4 gap-1">
        {(mode === "quarter" ? [0, 1, 2, 3] : Array.from({ length: 12 }, (_, index) => index)).map((value) => {
          const date = new Date(Number(yearDraft) || periodMonth.getFullYear(), mode === "quarter" ? value * 3 : value, 1);
          const label = mode === "quarter" ? t("views.calendar.quarter", { value: value + 1 }) : fmt(shortMonthFormat).format(date);
          return <Button key={value} size="sm" variant={(mode === "quarter" ? Math.floor(periodMonth.getMonth() / 3) : month.getMonth()) === value ? "secondary" : "ghost"} onClick={() => {
            showPeriod(mode === "year" ? "month" : mode, date); setPickerOpen(false);
          }}>{label}</Button>;
        })}
      </div>}
    </PopoverContent>
  </Popover>;
  const calendarControls = <>
    <h2 className="sr-only" aria-live="polite">{periodLabel}</h2>
    <div role="group" aria-label={t("views.calendar.mode")} className="flex gap-1">
      {calendarModes.map((value) => <Button key={value} size="sm" variant={mode === value ? "default" : "outline"} aria-pressed={mode === value}
        onClick={() => showPeriod(value, value === "week" ? weekDay : month)}>{t(`views.calendar.mode${value[0].toUpperCase()}${value.slice(1)}`)}</Button>)}
    </div>
    <div role="group" aria-label={t("views.calendar.navigate")} className="flex items-center gap-1">
      <Button size="icon-sm" variant="outline" aria-label={mode === "week" ? t("views.calendar.prevWeek") : t("views.calendar.prev")} onClick={() => showPeriod(mode, mode === "week" ? addDays(weekDay, -7) : new Date(month.getFullYear(), month.getMonth() + (mode === "quarter" ? -3 : mode === "year" ? -12 : -1), 1))}><ChevronLeft /></Button>
      {picker}
      <Button size="icon-sm" variant="outline" aria-label={mode === "week" ? t("views.calendar.nextWeek") : t("views.calendar.next")} onClick={() => showPeriod(mode, mode === "week" ? addDays(weekDay, 7) : new Date(month.getFullYear(), month.getMonth() + (mode === "quarter" ? 3 : mode === "year" ? 12 : 1), 1))}><ChevronRight /></Button>
      <Button size="sm" variant="outline" onClick={() => showPeriod(mode, new Date())}>{t("views.calendar.today")}</Button>
    </div>
    {mode !== "week" && <div role="group" aria-label={t("views.calendar.scheduleDisplay")} className="flex gap-1">
      {scheduleModes.map((value) => <Button key={value} size="sm" variant={scheduleMode === value ? "secondary" : "ghost"} aria-pressed={scheduleMode === value}
        onClick={() => setScheduleMode(value)}>{t(`views.calendar.schedule${value === "boundaries" ? "Boundaries" : "All"}`)}</Button>)}
    </div>}
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button size="sm" variant="outline" aria-label={t("views.calendar.options")} />}><Settings2 />{t("views.calendar.options")}</DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-56">
        <DropdownMenuGroup><DropdownMenuLabel>{t("views.calendar.options")}</DropdownMenuLabel>
          <DropdownMenuCheckboxItem checked={options.weekends} onCheckedChange={(value) => setOptions((current) => ({ ...current, weekends: value === true }))}>{t("views.calendar.showWeekends")}</DropdownMenuCheckboxItem>
          <DropdownMenuCheckboxItem checked={options.weekNumbers} onCheckedChange={(value) => setOptions((current) => ({ ...current, weekNumbers: value === true }))}>{t("views.calendar.showWeekNumbers")}</DropdownMenuCheckboxItem>
          <DropdownMenuCheckboxItem checked={options.subitems} onCheckedChange={(value) => setOptions((current) => ({ ...current, subitems: value === true }))}>{t("views.calendar.showSubitems")}</DropdownMenuCheckboxItem>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuGroup><DropdownMenuLabel>{t("views.calendar.density")}</DropdownMenuLabel>
          <DropdownMenuRadioGroup value={options.density} onValueChange={(value) => setOptions((current) => ({ ...current, density: value as CalendarOptions["density"] }))}>
            <DropdownMenuRadioItem value="comfortable">{t("views.calendar.comfortable")}</DropdownMenuRadioItem>
            <DropdownMenuRadioItem value="compact">{t("views.calendar.compact")}</DropdownMenuRadioItem>
          </DropdownMenuRadioGroup>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuGroup><DropdownMenuLabel>{t("views.calendar.maxItems")}</DropdownMenuLabel>
          <DropdownMenuRadioGroup value={String(options.maxItems)} onValueChange={(value) => setOptions((current) => ({ ...current, maxItems: Number(value) }))}>
            {[2, 3, 4].map((count) => <DropdownMenuRadioItem key={count} value={String(count)}>{count}</DropdownMenuRadioItem>)}
          </DropdownMenuRadioGroup>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuGroup><DropdownMenuLabel>{t("views.calendar.colorBy")}</DropdownMenuLabel>
          <DropdownMenuRadioGroup value={options.colorBy} onValueChange={(value) => setOptions((current) => ({ ...current, colorBy: value as CalendarOptions["colorBy"] }))}>
            <DropdownMenuRadioItem value="status">{t("views.calendar.colorStatus")}</DropdownMenuRadioItem>
            <DropdownMenuRadioItem value="type">{t("views.calendar.colorType")}</DropdownMenuRadioItem>
          </DropdownMenuRadioGroup>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
    <Button size="sm" variant={unscheduledOpen ? "secondary" : "outline"} aria-expanded={unscheduledOpen} onClick={() => setUnscheduledOpen((value) => !value)}>
      <CalendarClock />{t("views.calendar.unscheduledCount", { count: unscheduled.length })}
    </Button>
    {!readonly && <Button size="icon-sm" variant="ghost" aria-label={t("views.calendar.help")} aria-expanded={helpOpen} onClick={() => setHelpOpen(true)}><CircleHelp /></Button>}
  </>;

  return <div className="min-w-0 space-y-2">
    {renderToolbar ? renderToolbar(calendarControls) : <div className="flex flex-wrap items-center gap-2">{calendarControls}</div>}
    <div className="flex flex-col gap-3 lg:flex-row">
      {(mode === "quarter" || mode === "year") ? <div ref={calendarRef} role="grid" aria-label={t("views.calendar.label")} className={`grid min-h-0 flex-1 gap-2 ${mode === "quarter" ? "grid-cols-1 md:grid-cols-3" : "grid-cols-2 lg:grid-cols-4"}`}>
        {compactMonths.map((compactMonth) => {
          const compactFirst = addDays(compactMonth, -((compactMonth.getDay() - weekStart + 7) % 7));
          const compactDays = Array.from({ length: 42 }, (_, index) => addDays(compactFirst, index));
          const compactNames = compactDays.slice(0, 7);
          const monthEnd = new Date(compactMonth.getFullYear(), compactMonth.getMonth() + 1, 0);
          const monthItems = scheduled.filter((item) => startOf(item)! <= monthEnd && endOf(item)! >= compactMonth);
          const deadlines = monthItems.filter((item) => endOf(item)!.getMonth() === compactMonth.getMonth() && endOf(item)!.getFullYear() === compactMonth.getFullYear()).length;
          const milestones = monthItems.filter((item) => item.is_milestone).length;
          const overdue = monthItems.filter((item) => endOf(item)! < new Date()).length;
          return <section key={compactMonth.toISOString()} aria-label={fmt(monthFormat).format(compactMonth)} className={`grid min-h-0 grid-rows-[auto_auto_auto_1fr] overflow-hidden rounded border bg-card ${mode === "quarter" ? "h-72" : "h-36"}`}>
            <button type="button" className="px-2 py-1 text-left text-sm font-semibold hover:underline" onClick={() => showPeriod("month", compactMonth)}>{fmt(monthFormat).format(compactMonth)}</button>
            <p className="truncate px-2 pb-1 text-[10px] text-muted-foreground">{t("views.calendar.monthSummary", { deadlines, milestones, overdue })}</p>
            <div role="row" className="grid grid-cols-7 border-y bg-muted/30 text-center text-[10px] font-medium text-muted-foreground">
              {compactNames.map((day) => <div role="columnheader" key={day.getDay()}>{fmt(weekdayFormat).format(day).slice(0, 1)}</div>)}
            </div>
            <div className="grid min-h-0 grid-cols-7 grid-rows-6">
              {compactDays.map((day) => {
                const key = fromDay(day);
                const dayItems = itemsOn(day);
                const boundaries = dayItems.filter((item) => fromDay(startOf(item)!) === key || fromDay(endOf(item)!) === key).length;
                const holiday = holidays.get(key);
                const outside = day.getMonth() !== compactMonth.getMonth();
                const density = densityTints[dayItems.length === 0 ? 0 : dayItems.length <= 2 ? 1 : dayItems.length <= 5 ? 2 : dayItems.length <= 9 ? 3 : 4];
                return <button role="gridcell" type="button" key={key} disabled={outside}
                  aria-label={`${fmt(fullFormat).format(day)}${dayItems.length ? `: ${dayItems.map((item) => item.title).join(", ")}` : ""}`}
                  title={holiday ?? t("views.calendar.activity", { active: dayItems.length, boundaries })}
                  onClick={() => setSelectedDay(day)}
                  className={`relative min-h-0 border-b border-r p-0.5 text-left text-[10px] leading-none ${outside ? "text-muted-foreground/30" : `hover:ring-1 hover:ring-inset hover:ring-primary ${density}`} ${key === today ? "font-bold text-primary ring-1 ring-inset ring-primary" : ""}`}>
                  <span>{day.getDate()}</span>
                  {!outside && holiday && <span className="absolute bottom-0.5 left-0.5 size-1.5 rounded-full bg-rose-500" />}
                  {!outside && boundaries > 0 && <span className="absolute bottom-0 right-0.5 text-[9px] font-semibold">{boundaries}</span>}
                </button>;
              })}
            </div>
          </section>;
        })}
      </div> :
      <div ref={calendarRef} role="grid" aria-label={t("views.calendar.label")} tabIndex={0} className="relative w-full min-w-0 overflow-x-auto focus-visible:outline-2 focus-visible:outline-ring lg:w-auto lg:flex-1">
        <div role="row" className="grid min-w-[44rem] text-sm font-medium" style={{ gridTemplateColumns: `${options.weekNumbers ? "3rem " : ""}repeat(${options.weekends ? 7 : 5}, minmax(0, 1fr))` }}>
          {options.weekNumbers && <div role="columnheader" className="px-1">{t("views.calendar.weekShort")}</div>}
          {names.filter((day) => options.weekends || (day.getDay() !== 0 && day.getDay() !== 6)).map((day) => <div role="columnheader" key={day.getDay()} className="px-1">{fmt(weekdayFormat).format(day)}</div>)}
        </div>
        {Array.from({ length: weeks }, (_, week) => {
          // C2: one bar per item per week, drawn in the cell where the visible part starts and widened across its days.
          const rawWeekDays = days.slice(week * 7, week * 7 + 7);
          const weekDays = rawWeekDays.filter((day) => options.weekends || (day.getDay() !== 0 && day.getDay() !== 6));
          const layout = layoutDays(entries, weekDays, options.maxItems);
          const entryById = new Map(entries.map((entry) => [entry.id, itemById.get(entry.itemId)]));
          const barsAt = (col: number) => layout.bars.filter((bar) => bar.startCol === col);
          const hiddenAt = (col: number) => layout.hidden.filter((entry) => entry.startCol === col && (!entry.continuesBefore || week === 0));
          return <div role="row" key={week} className="grid min-w-[44rem]" style={{ gridTemplateColumns: `${options.weekNumbers ? "3rem " : ""}repeat(${weekDays.length}, minmax(0, 1fr))` }}>
            {options.weekNumbers && <div role="rowheader" className="border p-1 text-xs font-medium text-muted-foreground">{weekNumber(rawWeekDays[0])}</div>}
            {weekDays.map((day, col) => {
              const key = fromDay(day);
              const holiday = holidays.get(key);
              const off = !working.has(day.getDay()) || holiday !== undefined;
              const hidden = hiddenAt(col);
              const more = hidden.length;
              const canCreate = !!onCreateOn && !off;
              return <div role="gridcell" key={key} onClick={(event) => { if (!(event.target as HTMLElement).closest("button")) setSelectedDay(day); }}
                onDoubleClick={(event) => { if (canCreate && !(event.target as HTMLElement).closest("button")) onCreateOn(key); }}
                onDragOver={(event) => { if (drag) event.preventDefault(); }} onDrop={(event) => { event.preventDefault(); dropOn(day); }}
                className={`relative min-h-16 min-w-0 border p-1 ${day < periodMonth || day > periodEnd ? "bg-muted/30" : ""} ${off ? "bg-muted/50" : ""} ${key === today ? "ring-2 ring-inset ring-primary" : ""}`}
                style={{ height: mode === "week" ? `${Math.min(320, Math.max(176, 96 + scheduled.filter((item) => startOf(item)! <= addDays(first, 6) && endOf(item)! >= first).length * 32))}px` : `clamp(${options.density === "compact" ? "3.25rem" : "4rem"}, ${(calendarHeight - 24) / weeks}px, ${options.density === "compact" ? "5rem" : "7rem"})` }}>
                {canCreate && <button type="button" className="absolute right-1 top-1 z-20 rounded px-1 text-xs text-primary hover:bg-accent"
                  aria-label={t("views.calendar.newOn", { date: fmt(fullFormat).format(day) })} onClick={(event) => { event.stopPropagation(); onCreateOn(key); }}>+</button>}
                <p className="flex min-w-0 items-center gap-1 text-xs text-muted-foreground"><span className="rounded px-0.5">{day.getDate()}</span>
                  <span className="sr-only">{t("views.calendar.day", { date: fmt(fullFormat).format(day), count: more + layout.bars.filter((bar) => bar.startCol <= col && col <= bar.endCol).length })}</span>{holiday && <span className="truncate rounded bg-rose-100 px-1 font-medium text-rose-700 dark:bg-rose-500/20 dark:text-rose-300" title={holiday}>{holiday}</span>}</p>
                {barsAt(col).map((bar) => {
                  const item = entryById.get(bar.id);
                  return item ? <div key={bar.id} className="absolute left-0 z-10" style={{ top: `${1.5 + bar.lane * 1.5}rem`, width: `${(bar.endCol - bar.startCol + 1) * 100}%` }}>
                    {barButton(item, bar, day, true)}
                  </div> : null;
                })}
                {more > 0 && <button type="button" aria-haspopup="dialog" onClick={(event) => { event.stopPropagation(); setSelectedDay(day); }}
                  className="absolute bottom-0.5 left-1 z-20 rounded px-1 text-xs font-medium text-primary hover:underline">{t("views.calendar.more", { count: more })}</button>}
              </div>;
            })}
          </div>;
        })}
      </div>}
      {unscheduledOpen && <section aria-label={t("views.calendar.unscheduled")} className="w-full min-w-0 shrink-0 space-y-1 break-words rounded border p-2 lg:w-56">
        <div className="flex items-center justify-between"><h3 className="font-medium">{t("views.calendar.unscheduled")}</h3><Button size="icon-sm" variant="ghost" aria-label={t("views.calendar.closeUnscheduled")} onClick={() => setUnscheduledOpen(false)}><X /></Button></div>
        {unscheduled.length ? <ul className="space-y-0.5">{unscheduled.map((item) => chip(item))}</ul> : <p className="text-sm text-muted-foreground">{t("views.calendar.noneUnscheduled")}</p>}
      </section>}
    </div>
    {mode === "week" && <section className="rounded border bg-card p-3" aria-label={t("views.calendar.weekAgenda")}>
      <h3 className="mb-2 font-semibold">{t("views.calendar.weekAgenda")}</h3>
      {scheduled.some((item) => startOf(item)! <= addDays(first, 6) && endOf(item)! >= first)
        ? <ul className="grid gap-1 md:grid-cols-2">{scheduled.filter((item) => startOf(item)! <= addDays(first, 6) && endOf(item)! >= first).map((item) => chip(item))}</ul>
        : <p className="text-sm text-muted-foreground">{t("views.calendar.noItemsDate")}</p>}
    </section>}
    <Dialog open={!!selectedDay} onOpenChange={(open) => { if (!open) setSelectedDay(undefined); }}>
      <DialogContent className="inset-y-0 left-auto right-0 top-0 h-dvh w-full max-w-[26rem] translate-x-0 translate-y-0 rounded-none border-y-0 p-0 sm:rounded-l-lg">
        {selectedDay && <div className="flex h-full flex-col">
          <div className="space-y-3 border-b p-5 pr-12">
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{t("views.calendar.dayDetails")}</p>
            <DialogTitle>{fmt(fullFormat).format(selectedDay)}</DialogTitle>
            <DialogDescription>{t("views.calendar.scheduledCount", { count: itemsOn(selectedDay).length })}</DialogDescription>
            <div className="flex flex-wrap gap-2">
              {onCreateOn && working.has(selectedDay.getDay()) && !holidays.has(fromDay(selectedDay)) && <Button size="sm" onClick={() => onCreateOn(fromDay(selectedDay))}><Plus />{t("views.calendar.newItem")}</Button>}
              <Button size="sm" variant="outline" onClick={() => { showPeriod("week", selectedDay); setSelectedDay(undefined); }}><CalendarDays />{t("views.calendar.openWeek")}</Button>
            </div>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto p-5">
            <h3 className="mb-3 font-semibold">{t("views.calendar.itemsOnDate")}</h3>
            {itemsOn(selectedDay).length ? <ul className="space-y-2">{itemsOn(selectedDay).map((item) => <li key={item.id} className="space-y-1 border-b pb-2">{chip(item)}
              <p className="px-1 text-xs text-muted-foreground">{item.status?.name ?? t("views.noStatus")} · {itemRange(item)}</p></li>)}</ul>
              : <p className="text-sm text-muted-foreground">{t("views.calendar.noItemsDate")}</p>}
          </div>
        </div>}
      </DialogContent>
    </Dialog>
    <Dialog open={helpOpen} onOpenChange={setHelpOpen}>
      <DialogContent>
        <DialogTitle>{t("views.calendar.help")}</DialogTitle>
        <p>{t("views.calendar.keys")}</p>
      </DialogContent>
    </Dialog>
  </div>;
}
