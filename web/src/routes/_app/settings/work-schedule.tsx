import { useState, type FormEvent } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { loadWorkSchedule, workScheduleKey, type WorkSchedule } from "@/lib/work-schedule";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/settings/work-schedule")({ component: WorkScheduleRoute });

const path = "/api/v1/settings/work-schedule";
const weekOrder = [1, 2, 3, 4, 5, 6, 0]; // Monday first

function WorkScheduleRoute() {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [conflict, setConflict] = useState(false); // outlives the form, which remounts on the new version
  const query = useQuery({
    queryKey: workScheduleKey,
    queryFn: loadWorkSchedule,
  });
  if (!query.data) return null;
  const { data, etag } = query.data;
  // key on the version so a reload after a conflict resets the form to the server's values
  return <>
    {conflict && <p role="alert" className="mb-4 max-w-2xl text-sm text-destructive">{t("settings.workSchedule.conflict")}</p>}
    <ScheduleForm key={data.version} schedule={data} etag={etag}
      reload={(stale) => { setConflict(stale); return client.invalidateQueries({ queryKey: workScheduleKey }); }} />
  </>;
}


/** 1 Jan 2024 was a Monday: these seven dates give Monday-first weekday labels without literal strings. */
const locale = "en";
const weekdayFormat: Intl.DateTimeFormatOptions = { weekday: "narrow" };
const mondayFirst = Array.from({ length: 7 }, (_, i) => new Date(2024, 0, 1 + i));

/** Tailwind class strings for calendar days (kept as constants so every class is listed literally). */
const dayTone = { exception: "bg-warning/30 text-foreground", working: "text-foreground", off: "bg-muted text-foreground" };

const dayKey = (date: Date) => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;

/** 12-month view: working days, non-working days, and exceptions. Clicking a day prepares it as a new exception (or removes it). */
function AnnualCalendar({ year, days, holidays, disabled, onYear, onPick }: {
  year: number; days: Set<number>; holidays: { date: string; name: string }[]; disabled: boolean;
  onYear: (year: number) => void; onPick: (key: string, exception?: string) => void;
}) {
  const { t } = useTranslation();
  const exceptions = new Map(holidays.map((h) => [h.date, h.name]));
  const months = Array.from({ length: 12 }, (_, month) => {
    const first = new Date(year, month, 1);
    const offset = (first.getDay() + 6) % 7; // Monday first
    const count = new Date(year, month + 1, 0).getDate();
    const cells: (Date | null)[] = [...Array(offset).fill(null), ...Array.from({ length: count }, (_, i) => new Date(year, month, i + 1))];
    return { label: first.toLocaleString("en", { month: "long" }), cells };
  });
  return <div className="space-y-3">
    <div className="flex items-center gap-2">
      <Button type="button" variant="outline" size="sm" aria-label={t("settings.workSchedule.prevYear")} onClick={() => onYear(year - 1)}>‹</Button>
      <span className="min-w-16 text-center text-base font-semibold tabular-nums">{year}</span>
      <Button type="button" variant="outline" size="sm" aria-label={t("settings.workSchedule.nextYear")} onClick={() => onYear(year + 1)}>›</Button>
      <span className="ml-auto flex flex-wrap gap-3 text-xs text-muted-foreground">
        <span className="flex items-center gap-1"><span className="size-3 rounded-sm border border-muted-foreground/40" aria-hidden="true" />{t("settings.workSchedule.legendWorking")}</span>
        <span className="flex items-center gap-1"><span className="size-3 rounded-sm bg-muted" aria-hidden="true" />{t("settings.workSchedule.legendOff")}</span>
        <span className="flex items-center gap-1"><span className="size-3 rounded-sm bg-warning/30" aria-hidden="true" />{t("settings.workSchedule.legendException")}</span>
      </span>
    </div>
    <div className="grid grid-cols-2 gap-4 md:grid-cols-3 xl:grid-cols-4">
      {months.map((month) => <section key={month.label} className="space-y-1">
        <h2 className="text-sm font-medium">{month.label}</h2>
        <div className="grid grid-cols-7 gap-0.5 text-center text-[11px]">
          {mondayFirst.map((d) => <span key={d.toISOString()} className="pb-0.5 text-muted-foreground" aria-hidden="true">{d.toLocaleString(locale, weekdayFormat)}</span>)}
          {month.cells.map((date, index) => {
            if (!date) return <span key={`blank-${index}`} aria-hidden="true" />;
            const key = dayKey(date);
            const exception = exceptions.get(key);
            const working = days.has(date.getDay()) && !exception;
            const tone = exception ? dayTone.exception : working ? dayTone.working : dayTone.off;
            const label = exception ? `${key}: ${exception}` : `${key}: ${working ? t("settings.workSchedule.legendWorking") : t("settings.workSchedule.legendOff")}`;
            const name = `${date.getDate()} · ${label}`; // the visible day number must stay part of the accessible name
            return <button key={key} type="button" disabled={disabled} aria-label={name} title={label}
              onClick={() => onPick(key, exception)} className={`rounded-sm py-1 tabular-nums hover:outline hover:outline-1 hover:outline-ring ${tone}`}>
              {date.getDate()}
            </button>;
          })}
        </div>
      </section>)}
    </div>
  </div>;
}

function ScheduleForm({ schedule, etag, reload }: { schedule: WorkSchedule; etag: string; reload: (conflict: boolean) => Promise<unknown> }) {
  const { t } = useTranslation();
  const admin = useProfile()?.user_type === "admin";
  const [days, setDays] = useState(new Set(schedule.working_days));
  const [hours, setHours] = useState(String(schedule.daily_hours));
  const [holidays, setHolidays] = useState(schedule.exceptions);
  const [draft, setDraft] = useState({ date: "", name: "" });
  const [year, setYear] = useState(new Date().getFullYear());
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  function toggle(day: number) {
    const next = new Set(days);
    if (next.has(day)) next.delete(day); else next.add(day);
    setDays(next);
  }
  function addHoliday() {
    if (!draft.date || !draft.name.trim() || holidays.some((h) => h.date === draft.date)) return;
    setHolidays([...holidays, { date: draft.date, name: draft.name.trim() }].sort((a, b) => a.date.localeCompare(b.date)));
    setDraft({ date: "", name: "" });
  }
  async function save(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      await api<WorkSchedule>(path, {
        method: "PUT", headers: { "If-Match": etag },
        body: JSON.stringify({ working_days: [...days].sort(), daily_hours: Number(hours), exceptions: holidays }),
      });
      toast.success(t("settings.workSchedule.saved"));
      await reload(false);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) await reload(true);
      else setError((err as Error).message);
    } finally { setBusy(false); }
  }

  // Wide layout: rules on the left, the annual calendar uses the remaining width instead of leaving it empty.
  return <form className="space-y-6" onSubmit={save}>
    <header>
      <p className="text-sm text-muted-foreground">{admin ? t("settings.workSchedule.description") : t("settings.workSchedule.readOnly")}</p>
    </header>
    <div className="grid gap-8 lg:grid-cols-[22rem_minmax(0,1fr)] lg:items-start">
    <div className="space-y-6">
    <fieldset className="space-y-2" disabled={!admin || busy}>
      <legend className="mb-2 text-sm font-medium">{t("settings.workSchedule.workingDays")}</legend>
      <div className="flex flex-wrap gap-4">
        {weekOrder.map((day) => <label key={day} className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={days.has(day)} onChange={() => toggle(day)} />
          {t(`settings.workSchedule.days.${day}`)}
        </label>)}
      </div>
    </fieldset>
    <label className="block max-w-xs space-y-1 text-sm">
      <span>{t("settings.workSchedule.dailyHours")}</span>
      <Input type="number" min={1} max={24} step={1} value={hours} required disabled={!admin || busy}
        onChange={(event) => setHours(event.target.value)} />
    </label>
    <fieldset className="space-y-3" disabled={!admin || busy}>
      <legend className="mb-2 text-sm font-medium">{t("settings.workSchedule.holidays")}</legend>
      {holidays.length === 0 && <p className="text-sm text-muted-foreground">{t("settings.workSchedule.noHolidays")}</p>}
      <ul className="space-y-1">
        {holidays.map((holiday) => <li key={holiday.date} className="flex items-center gap-3 text-sm">
          <span className="w-28 tabular-nums">{holiday.date}</span>
          <span className="flex-1">{holiday.name}</span>
          <Button type="button" variant="ghost" size="sm"
            aria-label={t("settings.workSchedule.removeHoliday", { name: holiday.name })}
            onClick={() => setHolidays(holidays.filter((h) => h.date !== holiday.date))}>×</Button>
        </li>)}
      </ul>
      <div className="flex flex-wrap items-end gap-2">
        <label className="space-y-1 text-sm"><span>{t("settings.workSchedule.holidayDate")}</span>
          <Input type="date" value={draft.date} onChange={(event) => setDraft({ ...draft, date: event.target.value })} /></label>
        <label className="space-y-1 text-sm"><span>{t("settings.workSchedule.holidayName")}</span>
          <Input value={draft.name} maxLength={120} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label>
        <Button type="button" variant="outline" onClick={addHoliday}>{t("settings.workSchedule.addHoliday")}</Button>
      </div>
    </fieldset>
    </div>
    <div className="min-w-0">
    <fieldset className="space-y-3" disabled={!admin || busy}>
      <legend className="mb-2 text-sm font-medium">{t("settings.workSchedule.calendar")}</legend>
      <AnnualCalendar year={year} days={days} holidays={holidays} disabled={!admin || busy} onYear={setYear}
        onPick={(key, exception) => {
          if (exception) { setHolidays(holidays.filter((h) => h.date !== key)); return; }
          setDraft({ date: key, name: draft.name });
        }} />
    </fieldset>
    </div>
    </div>
    <FormError message={error} />
    {admin && <Button type="submit" disabled={busy || days.size === 0}>{t("settings.workSchedule.save")}</Button>}
  </form>;
}
