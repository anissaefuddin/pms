import { useMemo, useState, type KeyboardEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { Diamond, ChevronLeft, ChevronRight } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { loadWorkSchedule, workScheduleKey } from "@/lib/work-schedule";
import { addDays, daysBetween, fromDay, toDay } from "@/lib/calendar-days";
import { MAX_LANES, layoutWeek, type Bar } from "@/lib/calendar-layout";
import { useProfile } from "@/lib/profile";
import type { Item } from "@/lib/items";
import { tintOf } from "@/components/ui/status-chip";
import { TypeGlyph } from "@/components/type-badge";

type Update = { start_date: string | null; due_date: string | null };

const calendarModes = ["month", "week"] as const;
const monthFormat: Intl.DateTimeFormatOptions = { month: "long", year: "numeric" };
const weekdayFormat: Intl.DateTimeFormatOptions = { weekday: "short" };
const fullFormat: Intl.DateTimeFormatOptions = { dateStyle: "full" };
const startOf = (value: Item) => toDay(value.start_date) ?? toDay(value.due_date);
const endOf = (value: Item) => toDay(value.due_date) ?? toDay(value.start_date);

export function ItemCalendar({ items, readonly, onOpen, onPatch, onCreateOn }: {
  items: Item[]; readonly: boolean; onOpen: (item: Item) => void; onPatch: (item: Item, update: Update) => Promise<void>;
  /** Create a Task on a working day (YYYY-MM-DD). Absent when the user cannot create here. */
  onCreateOn?: (day: string) => void;
}) {
  const { t, i18n } = useTranslation();
  const [overflowOpen, setOverflowOpen] = useState<string>();
  const profile = useProfile();
  const [month, setMonth] = useState(() => { const now = new Date(); return new Date(now.getFullYear(), now.getMonth(), 1); });
  // C-03: Month or Week. Week mode keeps its own anchor day, so moving by week never changes the month view.
  const [mode, setMode] = useState<"month" | "week">("month");
  const [weekDay, setWeekDay] = useState(() => { const now = new Date(); return new Date(now.getFullYear(), now.getMonth(), now.getDate()); });
  const [drag, setDrag] = useState<{ id: string; from?: Date }>();
  const schedule = useQuery({ queryKey: workScheduleKey, queryFn: loadWorkSchedule });
  const holidays = useMemo(() => new Map((schedule.data?.data.exceptions ?? []).map((entry) => [entry.date, entry.name])), [schedule.data]);
  const working = new Set(schedule.data?.data.working_days ?? [1, 2, 3, 4, 5]);
  const weekStart = profile?.week_start === "sunday" ? 0 : profile?.week_start === "saturday" ? 6 : 1;
  const locale = i18n.language;
  const fmt = (options: Intl.DateTimeFormatOptions) => new Intl.DateTimeFormat(locale, options);

  const anchor = mode === "week" ? weekDay : month;
  const first = addDays(anchor, -((anchor.getDay() - weekStart + 7) % 7));
  const weeks = mode === "week" ? 1 : Math.ceil(((month.getDay() - weekStart + 7) % 7 + new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate()) / 7);
  const days = Array.from({ length: weeks * 7 }, (_, index) => addDays(first, index));
  const scheduled = items.filter((item) => startOf(item));
  const unscheduled = items.filter((item) => !startOf(item));
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
        aria-label={item.start_date || item.due_date ? t("views.calendar.item", { title: item.title, start: item.start_date ?? item.due_date, end: item.due_date ?? item.start_date }) : item.title}
        className={`flex w-full items-center gap-1 truncate rounded border-l-4 border-current px-1.5 py-0.5 text-left text-xs font-medium ${tintOf(item.status?.color)}`}><TypeGlyph item={item} className="size-3" /><span className="flex min-w-0 items-center gap-1">{item.is_milestone && <Diamond aria-hidden className="size-3 shrink-0" />}<span className="truncate">{item.title}</span></span></button>
    </li>;
  };
  const names = Array.from({ length: 7 }, (_, index) => addDays(first, index));
  const barButton = (item: Item, bar: Bar, from: Date) => {
    const canEdit = !readonly && item.my_capabilities.includes("edit");
    const label = t("views.calendar.item", { title: item.title, start: item.start_date ?? item.due_date, end: item.due_date ?? item.start_date });
    // C-01: the card opens on hover or keyboard focus of the bar; the bar's accessible description points at it.
    const cardId = `calendar-card-${item.id}-${bar.lane}-${bar.startCol}`;
    return <span className="group relative block">
      <button type="button" draggable={canEdit} onClick={() => onOpen(item)} onKeyDown={(event) => keyboard(event, item)} aria-describedby={cardId}
        onDragStart={(event) => { event.dataTransfer.setData("text/plain", item.id); setDrag({ id: item.id, from }); }}
        onDragEnd={() => setDrag(undefined)} aria-label={`${label}${bar.continuesBefore ? ` · ${t("views.calendar.continuesBefore")}` : ""}${bar.continuesAfter ? ` · ${t("views.calendar.continuesAfter")}` : ""}`}
        className={`flex h-6 w-full items-center gap-1 truncate rounded border-l-4 border-current px-1.5 text-left text-xs font-medium ${tintOf(item.status?.color)} ${bar.continuesBefore ? "rounded-l-none" : ""} ${bar.continuesAfter ? "rounded-r-none" : ""}`}>
        {bar.continuesBefore && <span aria-hidden>‹</span>}<TypeGlyph item={item} className="size-3 shrink-0" /><span className="flex min-w-0 items-center gap-1">{item.is_milestone && <Diamond aria-hidden className="size-3 shrink-0" />}<span className="truncate">{item.title}</span></span>{bar.continuesAfter && <span aria-hidden className="ml-auto">›</span>}
      </button>
      <span role="tooltip" id={cardId} className="pointer-events-none invisible absolute left-0 top-full z-40 mt-1 w-64 space-y-1 rounded-md border bg-popover p-3 text-xs text-popover-foreground shadow-md group-focus-within:visible group-hover:visible">
        <span className="block font-medium">{item.title}</span>
        <span className="block">{t("views.calendar.cardType")}: {item.item_type?.name ?? t("items.type")}</span>
        <span className="block">{t("views.calendar.cardStatus")}: {item.status?.name ?? t("views.noStatus")}</span>
        <span className="block">{t("views.calendar.cardAssignees")}: {item.assignees.map((person) => person.name).join(", ") || t("items.chips.noAssignee")}</span>
        <span className="block">{t("views.calendar.cardDates")}: {[item.start_date, item.due_date].filter(Boolean).join(" – ") || t("items.chips.noDates")}</span>
      </span>
    </span>;
  };

  return <div className="min-w-0 space-y-2">
    <div className="flex flex-wrap items-center gap-2">
      <div role="group" aria-label={t("views.calendar.mode")} className="flex gap-1">
        {calendarModes.map((value) => <Button key={value} size="sm" variant={mode === value ? "default" : "outline"} aria-pressed={mode === value}
          onClick={() => { setMode(value); if (value === "week") setWeekDay(new Date(month.getFullYear(), month.getMonth(), Math.min(new Date().getDate(), new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate()))); }}>{t(`views.calendar.mode${value === "month" ? "Month" : "Week"}`)}</Button>)}
      </div>
      <Button size="sm" variant="outline" aria-label={mode === "week" ? t("views.calendar.prevWeek") : t("views.calendar.prev")} onClick={() => (mode === "week" ? setWeekDay(addDays(weekDay, -7)) : setMonth(new Date(month.getFullYear(), month.getMonth() - 1, 1)))}><ChevronLeft /></Button>
      <Button size="sm" variant="outline" onClick={() => { const now = new Date(); setMonth(new Date(now.getFullYear(), now.getMonth(), 1)); setWeekDay(new Date(now.getFullYear(), now.getMonth(), now.getDate())); }}>{t("views.calendar.today")}</Button>
      <Button size="sm" variant="outline" aria-label={mode === "week" ? t("views.calendar.nextWeek") : t("views.calendar.next")} onClick={() => (mode === "week" ? setWeekDay(addDays(weekDay, 7)) : setMonth(new Date(month.getFullYear(), month.getMonth() + 1, 1)))}><ChevronRight /></Button>
      <h2 className="font-semibold" aria-live="polite">{mode === "week" ? `${fmt(fullFormat).format(first)} – ${fmt(fullFormat).format(addDays(first, 6))}` : fmt(monthFormat).format(month)}</h2>
      {!readonly && <p className="text-sm text-muted-foreground">{t("views.calendar.keys")}</p>}
    </div>
    <div className="flex flex-col gap-3 lg:flex-row">
      <div role="grid" aria-label={t("views.calendar.label")} tabIndex={0} className="relative w-full min-w-0 overflow-x-auto focus-visible:outline-2 focus-visible:outline-ring lg:w-auto lg:flex-1">
        <div role="row" className="grid min-w-[44rem] grid-cols-7 text-sm font-medium">
          {names.map((day) => <div role="columnheader" key={day.getDay()} className="px-1">{fmt(weekdayFormat).format(day)}</div>)}
        </div>
        {Array.from({ length: weeks }, (_, week) => {
          // C2: one bar per item per week, drawn in the cell where the visible part starts and widened across its days.
          const weekDays = days.slice(week * 7, week * 7 + 7);
          const layout = layoutWeek(scheduled.map((item) => ({ id: item.id, start: startOf(item)!, end: endOf(item)! })), weekDays[0]);
          const byId = new Map(items.map((item) => [item.id, item]));
          const barsAt = (col: number) => layout.bars.filter((bar) => bar.startCol === col);
          const hiddenAt = (col: number) => layout.hidden.filter((entry) => entry.startCol <= col && col <= entry.endCol);
          return <div role="row" key={week} className="grid min-w-[44rem] grid-cols-7">
            {weekDays.map((day, col) => {
              const key = fromDay(day);
              const holiday = holidays.get(key);
              const off = !working.has(day.getDay()) || holiday !== undefined;
              const more = layout.overflowByCol[col];
              const openKey = `${week}-${col}`;
              return <div role="gridcell" key={key} onDragOver={(event) => { if (drag) event.preventDefault(); }} onDrop={(event) => { event.preventDefault(); dropOn(day); }}
                className={`relative h-28 min-w-0 border p-1 ${day.getMonth() !== month.getMonth() ? "bg-muted/30" : ""} ${off ? "bg-muted/50" : ""} ${key === today ? "ring-2 ring-primary" : ""}`}>
                {onCreateOn && !readonly && !off && <button type="button" className="absolute right-1 top-1 z-20 rounded px-1 text-xs text-primary hover:bg-accent"
                  aria-label={t("views.calendar.newOn", { date: fmt(fullFormat).format(day) })} onClick={() => onCreateOn(key)}>+</button>}
                <p className="text-xs text-muted-foreground"><span aria-hidden>{day.getDate()}</span>
                  <span className="sr-only">{t("views.calendar.day", { date: fmt(fullFormat).format(day), count: more + layout.bars.filter((bar) => bar.startCol <= col && col <= bar.endCol).length })}</span>{holiday ? ` · ${holiday}` : ""}</p>
                {barsAt(col).map((bar) => {
                  const item = byId.get(bar.id);
                  return item ? <div key={bar.id} className="absolute left-0 right-0 z-10" style={{ top: `${1.5 + bar.lane * 1.5}rem`, width: `${(bar.endCol - bar.startCol + 1) * 100}%` }}>
                    {barButton(item, bar, day)}
                  </div> : null;
                })}
                {more > 0 && <button type="button" aria-expanded={overflowOpen === openKey} onClick={() => setOverflowOpen(overflowOpen === openKey ? undefined : openKey)}
                  className="absolute bottom-0.5 left-1 z-20 rounded px-1 text-xs font-medium text-primary hover:underline">{t("views.calendar.more", { count: more })}</button>}
                {overflowOpen === openKey && <ul className="absolute left-0 top-6 z-30 w-56 space-y-0.5 rounded border bg-popover p-2 shadow-md">
                  {hiddenAt(col).map((entry) => byId.get(entry.id)).filter((item): item is Item => !!item).map((item) => <li key={item.id}>{chip(item)}</li>)}
                </ul>}
              </div>;
            })}
          </div>;
        })}
      </div>
      <section aria-label={t("views.calendar.unscheduled")} className="w-full min-w-0 shrink-0 space-y-1 break-words rounded border p-2 lg:w-56">
        <h3 className="font-medium">{t("views.calendar.unscheduled")}</h3>
        {unscheduled.length ? <ul className="space-y-0.5">{unscheduled.map((item) => chip(item))}</ul> : <p className="text-sm text-muted-foreground">{t("views.calendar.noneUnscheduled")}</p>}
      </section>
    </div>
  </div>;
}
