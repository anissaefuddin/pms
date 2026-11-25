import { createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useId, useState, type FormEvent, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { FormError } from "@/components/auth/form-error";
import { ApiError, api } from "@/lib/api";
import { addDays, fromDay, toDay } from "@/lib/calendar-days";
import { formatDate, formatDateTime, useProfile } from "@/lib/profile";
import { useSearchResults } from "@/lib/search";
import { formatMinutes, parseMinutes, timeEntryPage, type TimeEntry, type TimeEntryPage } from "@/lib/time-entries";
import { TopbarTitle } from "@/components/topbar-title";

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
  const [history, setHistory] = useState<{ id: string; label: string }>();
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

  const toSubmit = (entries.data ?? []).filter((entry) => entry.status === "draft" || entry.status === "rejected").length;
  async function submitWeek() {
    setError("");
    setPending("submit");
    try {
      await api("/api/v1/time-entries/submit", { method: "POST", body: JSON.stringify({ from, to }) });
      await qc.invalidateQueries({ queryKey: ["time-entries"] });
    } catch (err) { setError((err as Error).message); } finally { setPending(""); }
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
    <TopbarTitle>{t("time.timesheet")}</TopbarTitle>
    <div className="flex flex-wrap items-center gap-2">
      <Button variant="outline" onClick={() => setWeek(week - 1)}>{t("time.previous")}</Button>
      <Button variant="outline" onClick={() => setWeek(0)}>{t("time.current")}</Button>
      <Button variant="outline" onClick={() => setWeek(week + 1)}>{t("time.next")}</Button>
      <p>{formatDate(from, profile)} – {formatDate(to, profile)}</p>
      <Button className="ms-auto" disabled={!toSubmit || !!pending || Object.keys(drafts).length > 0} onClick={() => void submitWeek()}>{t("time.submitWeek", { count: toSubmit })}</Button>
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
    {entries.isSuccess && <div className="overflow-auto rounded border" role="region" tabIndex={0} aria-label={t("time.timesheet")}>
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
            const closed = entry?.locked || entry?.status === "submitted" || entry?.status === "approved";
            function update(values: Partial<Draft>) { setDrafts((old) => ({ ...old, [key]: { ...draft, ...values } })); }
            return <td key={day} className="p-2">
              <form className="space-y-1" onSubmit={(event) => void save(event, key, task.id, day, draft)}>
                <Input data-duration aria-label={t("time.durationFor", { label })} placeholder="0:00" value={draft.duration}
                  disabled={closed || pending === key} onKeyDown={move} onChange={(event) => update({ duration: event.target.value })} />
                <Input aria-label={t("time.commentFor", { label })} placeholder={t("time.comment")} maxLength={2000}
                  disabled={closed || pending === key} value={draft.comment} onChange={(event) => update({ comment: event.target.value })} />
                {entry?.locked && <span>{t("time.locked")}</span>}
                {entry && entry.status !== "draft" && <p className={entry.status === "rejected" ? "text-xs text-destructive" : "text-xs text-muted-foreground"}>
                  {t(`time.status.${entry.status}`)}{entry.review_note && `: ${entry.review_note}`}</p>}
                {entry && entry.status !== "draft" && <Button type="button" variant="ghost" size="sm" aria-label={t("time.history.for", { label })}
                  onClick={() => setHistory({ id: entry.id, label })}>{t("time.history.title")}</Button>}
                {drafts[key] && <div className="flex gap-1">
                  <Button type="submit" size="sm" disabled={!!pending || closed}>{t("items.save")}</Button>
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
    <TimeReview />
    {history && <TimeHistory entry={history} onClose={() => setHistory(undefined)} />}
  </main>;
}

type HistoryStep = { action: string; actor_name: string; note: string; minutes: number; at: string };

function TimeHistory({ entry, onClose }: { entry: { id: string; label: string }; onClose: () => void }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const steps = useQuery({
    queryKey: ["time-entries", "history", entry.id],
    queryFn: ({ signal }) => api<{ items: HistoryStep[] }>(`/api/v1/time-entries/${entry.id}/history`, { signal }),
  });
  return <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
    <DialogContent>
      <DialogTitle>{t("time.history.for", { label: entry.label })}</DialogTitle>
      <FormError message={steps.error?.message ?? ""} />
      {steps.isPending && <p role="status">{t("items.loading")}</p>}
      {steps.data?.items.length === 0 && <p className="text-sm text-muted-foreground">{t("time.history.empty")}</p>}
      <ol className="space-y-1 text-sm">{steps.data?.items.map((step, index) => <li key={index}>
        {formatDateTime(step.at, profile)} · {step.actor_name} · {t(`time.history.${step.action}`)} · {formatMinutes(step.minutes)}{step.note && ` · ${step.note}`}
      </li>)}</ol>
    </DialogContent>
  </Dialog>;
}

/** Submitted time of other people on tasks this person has Full access to. Hidden when there is nothing to review. */
function TimeReview() {
  const { t } = useTranslation();
  const profile = useProfile();
  const qc = useQueryClient();
  const heading = useId();
  const [notes, setNotes] = useState<Record<string, string>>({});
  const list = useQuery({
    queryKey: ["time-entries", "review"],
    queryFn: ({ signal }) => api<TimeEntryPage>("/api/v1/time-entries/review", { signal }),
  });
  const decide = useMutation({
    mutationFn: ({ entry, decision }: { entry: TimeEntry; decision: "approved" | "rejected" }) => api(`/api/v1/time-entries/${entry.id}/review`, {
      method: "POST", headers: { "If-Match": `"${entry.version}"` }, body: JSON.stringify({ decision, note: notes[entry.id] ?? "" }),
    }),
    onSettled: () => qc.invalidateQueries({ queryKey: ["time-entries"] }),
  });
  if (!list.data?.items.length) return null;
  return <section aria-labelledby={heading} className="space-y-2">
    <h2 id={heading} className="font-semibold">{t("time.review.title")}</h2>
    <FormError message={decide.error?.message ?? ""} />
    <ul className="divide-y rounded border">{list.data.items.map((entry) => {
      const label = `${entry.user_name}, ${entry.item_title}, ${formatDate(entry.date, profile)}`;
      return <li key={entry.id} className="flex flex-wrap items-center gap-2 p-2 text-sm">
        <div className="min-w-0 flex-1"><p className="font-medium">{label} · {formatMinutes(entry.minutes)}</p>
          {entry.comment && <p className="text-muted-foreground">{entry.comment}</p>}</div>
        <Input className="w-56" aria-label={t("time.review.noteFor", { label })} placeholder={t("time.review.note")} maxLength={1000}
          value={notes[entry.id] ?? ""} onChange={(event) => setNotes((old) => ({ ...old, [entry.id]: event.target.value }))} />
        <Button size="sm" disabled={decide.isPending} aria-label={t("time.review.approveFor", { label })} onClick={() => decide.mutate({ entry, decision: "approved" })}>{t("time.review.approve")}</Button>
        <Button size="sm" variant="outline" disabled={decide.isPending} aria-label={t("time.review.rejectFor", { label })} onClick={() => decide.mutate({ entry, decision: "rejected" })}>{t("time.review.reject")}</Button>
      </li>;
    })}</ul>
    {list.data.next_offset != null && <p className="text-sm text-muted-foreground">{t("time.review.more")}</p>}
  </section>;
}
