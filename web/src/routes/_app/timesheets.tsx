import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { FormError } from "@/components/auth/form-error";
import { ApiError, api } from "@/lib/api";
import { addDays, fromDay, toDay } from "@/lib/calendar-days";
import { formatDate, useProfile } from "@/lib/profile";
import { useSearchResults } from "@/lib/search";
import { formatMinutes, parseMinutes, timeEntryPage, type TimeEntry } from "@/lib/time-entries";

export const Route = createFileRoute("/_app/timesheets")({ component: Timesheets });
type Task = { id: string; title: string };
type Draft = { duration: string; comment: string; entry?: TimeEntry };

function Timesheets() {
  const profile = useProfile();
  return profile && <Timesheet key={profile.id} user={profile.id} start={profile.week_start}
    today={new Intl.DateTimeFormat("sv-SE", { timeZone: profile.timezone, dateStyle: "short" }).format(new Date())} />;
}

function Timesheet({ user, start, today }: { user: string; start: "monday" | "sunday" | "saturday"; today: string }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const qc = useQueryClient();
  const [week, setWeek] = useState(0);
  const [search, setSearch] = useState("");
  const storageKey = `pms:timesheet-pins:${user}`;
  const [pins, setPins] = useState<Task[]>(() => {
    try { return JSON.parse(localStorage.getItem(storageKey) ?? "[]"); } catch { return []; }
  });
  const [drafts, setDrafts] = useState<Record<string, Draft>>({});
  const [pending, setPending] = useState("");
  const [error, setError] = useState("");
  const date = toDay(today)!;
  const first = addDays(date, -((date.getDay() - { monday: 1, sunday: 0, saturday: 6 }[start] + 7) % 7) + 7 * week);
  const dates = Array.from({ length: 7 }, (_, i) => fromDay(addDays(first, i)));
  const from = dates[0], to = dates[6];
  const entries = useQuery({
    queryKey: ["time-entries", user, from, to],
    queryFn: async ({ signal }) => {
      const rows: TimeEntry[] = [];
      let offset: number | null = 0;
      do {
        const page = await timeEntryPage(new URLSearchParams({ from, to, offset: String(offset), limit: "200" }), signal);
        rows.push(...page.items);
        offset = page.next_offset;
      } while (offset != null);
      return rows;
    },
  });
  const results = useSearchResults(search.trim(), "everywhere", {}, 20);
  const choices = results.data?.items.filter((item) => item.type === "task" && item.my_capabilities.includes("change_status")) ?? [];
  const tasks = new Map(pins.map((task) => [task.id, task]));
  const cells = new Map<string, TimeEntry>();
  const totals = new Map<string, number>();
  for (const entry of entries.data ?? []) {
    tasks.set(entry.item_id, { id: entry.item_id, title: entry.item_title ?? entry.item_id });
    cells.set(`${entry.item_id}:${entry.date}`, entry);
    totals.set(entry.date, (totals.get(entry.date) ?? 0) + (entry.minutes ?? 0));
  }

  function togglePin(task: Task) {
    const next = pins.some(({ id }) => id === task.id) ? pins.filter(({ id }) => id !== task.id) : [...pins, task];
    setPins(next);
    localStorage.setItem(storageKey, JSON.stringify(next));
    setSearch("");
  }

  function discard(key: string) {
    setDrafts((previous) => {
      const next = { ...previous };
      delete next[key];
      return next;
    });
  }

  async function save(event: FormEvent, key: string, task: string, day: string, draft: Draft) {
    event.preventDefault();
    if (pending || !drafts[key]) return;
    const minutes = parseMinutes(draft.duration);
    if (!Number.isInteger(minutes) || minutes < 0 || minutes > 1440 || (!minutes && draft.comment)) {
      setError(t("time.invalidDuration"));
      return;
    }
    setError("");
    setPending(key);
    const entry = draft.entry;
    try {
      if (entry) {
        await api(`/api/v1/time-entries/${entry.id}`, {
          method: minutes ? "PATCH" : "DELETE",
          headers: { "If-Match": `"${entry.version}"` },
          body: minutes ? JSON.stringify({ minutes, comment: draft.comment }) : undefined,
        });
      } else if (minutes) {
        await api("/api/v1/time-entries", { method: "POST", body: JSON.stringify({ item_id: task, date: day, minutes, comment: draft.comment }) });
      }
      discard(key);
      await qc.invalidateQueries({ queryKey: ["time-entries"] });
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        await entries.refetch();
        setError(t("time.conflict"));
      } else setError((err as Error).message);
    } finally { setPending(""); }
  }

  function move(event: KeyboardEvent<HTMLInputElement>) {
    const delta = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 }[event.key];
    if (!event.altKey || delta === undefined) return;
    const inputs = [...(event.currentTarget.closest("table")?.querySelectorAll<HTMLInputElement>("[data-duration]") ?? [])];
    inputs[inputs.indexOf(event.currentTarget) + delta]?.focus();
    event.preventDefault();
  }

  return <main className="min-w-0 space-y-4 p-6">
    <h1 className="text-2xl font-semibold">{t("time.timesheet")}</h1>
    <div className="flex flex-wrap items-center gap-2">
      <Button variant="outline" onClick={() => setWeek(week - 1)}>{t("time.previous")}</Button>
      <Button variant="outline" onClick={() => setWeek(0)}>{t("time.current")}</Button>
      <Button variant="outline" onClick={() => setWeek(week + 1)}>{t("time.next")}</Button>
      <p>{formatDate(from, profile)} – {formatDate(to, profile)}</p>
    </div>
    <label className="block max-w-sm space-y-1">{t("time.findTask")}
      <Input type="search" value={search} onChange={(event) => setSearch(event.target.value)} />
    </label>
    {choices.map((task) => <Button key={task.id} size="sm" variant="outline" onClick={() => togglePin(task)}>
      {t(pins.some(({ id }) => id === task.id) ? "time.unpin" : "time.pin", { task: task.title })}
    </Button>)}
    <p className="text-sm text-muted-foreground">{t("time.help")}</p>
    <FormError message={error || entries.error?.message || ""} />
    {entries.isPending && <p role="status">{t("items.loading")}</p>}
    {entries.isSuccess && <div className="overflow-auto rounded border">
      <table aria-label={t("time.timesheet")} className="w-full border-collapse text-sm">
        <thead><tr className="bg-muted">
          <th scope="col" className="p-2 text-left">{t("items.task")}</th>
          {dates.map((day) => <th key={day} scope="col" className="min-w-32 p-2">{formatDate(day, profile)}</th>)}
          <th scope="col" className="p-2">{t("time.total")}</th>
        </tr></thead>
        <tbody>{[...tasks.values()].map((task) => <tr key={task.id} className="border-t align-top">
          <th scope="row" className="min-w-40 space-y-2 p-2 text-left">{task.title}
            {pins.some(({ id }) => id === task.id) && <Button variant="ghost" size="sm" onClick={() => togglePin(task)}>{t("time.unpin", { task: task.title })}</Button>}
          </th>
          {dates.map((day) => {
            const key = `${task.id}:${day}`;
            const entry = cells.get(key);
            const draft = drafts[key] ?? { duration: entry ? formatMinutes(entry.minutes ?? 0) : "", comment: entry?.comment ?? "", entry };
            const label = `${task.title}, ${day}`;
            function update(values: Partial<Draft>) { setDrafts((old) => ({ ...old, [key]: { ...draft, ...values } })); }
            return <td key={day} className="p-2">
              <form className="space-y-1" onSubmit={(event) => void save(event, key, task.id, day, draft)}>
                <Input data-duration aria-label={t("time.durationFor", { label })} placeholder="0:00" value={draft.duration}
                  disabled={entry?.locked || pending === key} onKeyDown={move} onChange={(event) => update({ duration: event.target.value })} />
                <Input aria-label={t("time.commentFor", { label })} placeholder={t("time.comment")} maxLength={2000}
                  disabled={entry?.locked || pending === key} value={draft.comment} onChange={(event) => update({ comment: event.target.value })} />
                {entry?.locked && <span>{t("time.locked")}</span>}
                {drafts[key] && <div className="flex gap-1">
                  <Button type="submit" size="sm" disabled={!!pending || entry?.locked}>{t("items.save")}</Button>
                  <Button variant="ghost" size="sm" type="button" disabled={pending === key} onClick={() => discard(key)}>{t("items.cancel")}</Button>
                </div>}
              </form>
            </td>;
          })}
          <td className="p-2 text-right">{formatMinutes(dates.reduce((sum, day) => sum + (cells.get(`${task.id}:${day}`)?.minutes ?? 0), 0))}</td>
        </tr>)}</tbody>
        <tfoot><tr className="border-t font-semibold">
          <th scope="row" className="p-2 text-left">{t("time.total")}</th>
          {dates.map((day) => <td key={day} className="p-2 text-center">{formatMinutes(totals.get(day) ?? 0)}</td>)}
          <td className="p-2 text-right">{formatMinutes([...totals.values()].reduce((sum, n) => sum + n, 0))}</td>
        </tr></tfoot>
      </table>
    </div>}
  </main>;
}
