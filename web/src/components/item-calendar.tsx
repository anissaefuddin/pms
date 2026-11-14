import { useMemo, useState, type KeyboardEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { loadWorkSchedule, workScheduleKey } from "@/lib/work-schedule";
import { addDays, daysBetween, fromDay, toDay } from "@/lib/calendar-days";
import { useProfile } from "@/lib/profile";
import type { Item } from "@/lib/items";
import { tintOf } from "@/components/status-chip";
import { TypeGlyph } from "@/components/type-badge";

type Update = { start_date: string | null; due_date: string | null };

const monthFormat: Intl.DateTimeFormatOptions = { month: "long", year: "numeric" };
const weekdayFormat: Intl.DateTimeFormatOptions = { weekday: "short" };
const fullFormat: Intl.DateTimeFormatOptions = { dateStyle: "full" };
const startOf = (value: Item) => toDay(value.start_date) ?? toDay(value.due_date);
const endOf = (value: Item) => toDay(value.due_date) ?? toDay(value.start_date);

export function ItemCalendar({ items, readonly, onOpen, onPatch }: {
  items: Item[]; readonly: boolean; onOpen: (item: Item) => void; onPatch: (item: Item, update: Update) => Promise<void>;
}) {
  const { t, i18n } = useTranslation();
  const profile = useProfile();
  const [month, setMonth] = useState(() => { const now = new Date(); return new Date(now.getFullYear(), now.getMonth(), 1); });
  const [drag, setDrag] = useState<{ id: string; from?: Date }>();
  const schedule = useQuery({ queryKey: workScheduleKey, queryFn: loadWorkSchedule });
  const holidays = useMemo(() => new Map((schedule.data?.data.exceptions ?? []).map((entry) => [entry.date, entry.name])), [schedule.data]);
  const working = new Set(schedule.data?.data.working_days ?? [1, 2, 3, 4, 5]);
  const weekStart = profile?.week_start === "sunday" ? 0 : profile?.week_start === "saturday" ? 6 : 1;
  const locale = i18n.language;
  const fmt = (options: Intl.DateTimeFormatOptions) => new Intl.DateTimeFormat(locale, options);

  const first = addDays(month, -((month.getDay() - weekStart + 7) % 7));
  const weeks = Math.ceil(((month.getDay() - weekStart + 7) % 7 + new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate()) / 7);
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
        className={`flex w-full items-center gap-1 truncate rounded border-l-4 border-current px-1.5 py-0.5 text-left text-xs font-medium ${tintOf(item.status?.color)}`}><TypeGlyph item={item} className="size-3" /><span className="truncate">{item.title}</span></button>
    </li>;
  };
  const names = Array.from({ length: 7 }, (_, index) => addDays(first, index));

  return <div className="min-w-0 space-y-2">
    <div className="flex flex-wrap items-center gap-2">
      <Button size="sm" variant="outline" aria-label={t("views.calendar.prev")} onClick={() => setMonth(new Date(month.getFullYear(), month.getMonth() - 1, 1))}><ChevronLeft /></Button>
      <Button size="sm" variant="outline" onClick={() => { const now = new Date(); setMonth(new Date(now.getFullYear(), now.getMonth(), 1)); }}>{t("views.calendar.today")}</Button>
      <Button size="sm" variant="outline" aria-label={t("views.calendar.next")} onClick={() => setMonth(new Date(month.getFullYear(), month.getMonth() + 1, 1))}><ChevronRight /></Button>
      <h2 className="font-semibold" aria-live="polite">{fmt(monthFormat).format(month)}</h2>
      {!readonly && <p className="text-sm text-muted-foreground">{t("views.calendar.keys")}</p>}
    </div>
    <div className="flex flex-col gap-3 lg:flex-row">
      <div role="grid" aria-label={t("views.calendar.label")} tabIndex={0} className="relative w-full min-w-0 overflow-x-auto focus-visible:outline-2 focus-visible:outline-ring lg:w-auto lg:flex-1">
        <div role="row" className="grid min-w-[44rem] grid-cols-7 text-sm font-medium">
          {names.map((day) => <div role="columnheader" key={day.getDay()} className="px-1">{fmt(weekdayFormat).format(day)}</div>)}
        </div>
        {Array.from({ length: weeks }, (_, week) => <div role="row" key={week} className="grid min-w-[44rem] grid-cols-7">
          {days.slice(week * 7, week * 7 + 7).map((day) => {
            const key = fromDay(day);
            const here = scheduled.filter((item) => startOf(item)! <= day && day <= endOf(item)!);
            const holiday = holidays.get(key);
            const off = !working.has(day.getDay()) || holiday !== undefined;
            return <div role="gridcell" key={key} onDragOver={(event) => { if (drag) event.preventDefault(); }} onDrop={(event) => { event.preventDefault(); dropOn(day); }}
              className={`h-28 min-w-0 space-y-1 overflow-y-auto border p-1 ${day.getMonth() !== month.getMonth() ? "bg-muted/30" : ""} ${off ? "bg-muted/50" : ""} ${key === today ? "ring-2 ring-primary" : ""}`}>
              <p className="text-xs text-muted-foreground"><span aria-hidden>{day.getDate()}</span>
                <span className="sr-only">{t("views.calendar.day", { date: fmt(fullFormat).format(day), count: here.length })}</span>{holiday ? ` · ${holiday}` : ""}</p>
              <ul className="space-y-0.5">{here.map((item) => chip(item, day))}</ul>
            </div>;
          })}
        </div>)}
      </div>
      <section aria-label={t("views.calendar.unscheduled")} className="w-full min-w-0 shrink-0 space-y-1 break-words rounded border p-2 lg:w-56">
        <h3 className="font-medium">{t("views.calendar.unscheduled")}</h3>
        {unscheduled.length ? <ul className="space-y-0.5">{unscheduled.map((item) => chip(item))}</ul> : <p className="text-sm text-muted-foreground">{t("views.calendar.noneUnscheduled")}</p>}
      </section>
    </div>
  </div>;
}
