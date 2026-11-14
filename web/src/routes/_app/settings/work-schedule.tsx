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

function ScheduleForm({ schedule, etag, reload }: { schedule: WorkSchedule; etag: string; reload: (conflict: boolean) => Promise<unknown> }) {
  const { t } = useTranslation();
  const admin = useProfile()?.user_type === "admin";
  const [days, setDays] = useState(new Set(schedule.working_days));
  const [hours, setHours] = useState(String(schedule.daily_hours));
  const [holidays, setHolidays] = useState(schedule.exceptions);
  const [draft, setDraft] = useState({ date: "", name: "" });
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

  return <form className="max-w-2xl space-y-6" onSubmit={save}>
    <header>
      <h2 className="text-lg font-semibold">{t("settings.workSchedule.title")}</h2>
      <p className="text-sm text-muted-foreground">{admin ? t("settings.workSchedule.description") : t("settings.workSchedule.readOnly")}</p>
    </header>
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
    <FormError message={error} />
    {admin && <Button type="submit" disabled={busy || days.size === 0}>{t("settings.workSchedule.save")}</Button>}
  </form>;
}
