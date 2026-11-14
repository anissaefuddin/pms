import { formatDateTime, useProfile } from "@/lib/profile";
import { useState } from "react";
import { Link, createFileRoute, useNavigate } from "@tanstack/react-router";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Archive, Bell } from "lucide-react";
import { api } from "@/lib/api";
import { listNotifications, notificationKey, type Notification } from "@/lib/notifications";
import { Button } from "@/components/ui/button";
import { FormError } from "@/components/auth/form-error";
import { ConfirmDialog } from "@/components/confirm-dialog";

export const Route = createFileRoute("/_app/inbox")({
  validateSearch: (search: Record<string, unknown>): { unread?: boolean; box?: Box } => ({
    unread: search.unread === true ? true : undefined,
    box: search.box === "archive" || search.box === "sent" ? search.box : undefined,
  }),
  component: Inbox,
});

type Box = "incoming" | "archive" | "sent";
const boxes: Box[] = ["incoming", "sent", "archive"];

function relative(date: string, locale: string) {
  const minutes = Math.floor(Math.max(0, Date.now() - Date.parse(date)) / 60000);
  const format = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
  if (minutes >= 1440) return format.format(-Math.floor(minutes / 1440), "day");
  if (minutes >= 60) return format.format(-Math.floor(minutes / 60), "hour");
  return format.format(-minutes, "minute");
}

function Inbox() {
  const { t, i18n } = useTranslation();
  const profile = useProfile();
  const { unread, box: boxParam } = Route.useSearch();
  const box: Box = boxParam ?? "incoming";
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const navigate = useNavigate();
  const client = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [confirmArchive, setConfirmArchive] = useState(false);
  const [error, setError] = useState("");
  const query = useInfiniteQuery({
    queryKey: [...notificationKey, "inbox", box, !!unread],
    initialPageParam: "",
    queryFn: ({ pageParam }) => listNotifications(30, unread, pageParam, box),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  const entries = query.data?.pages.flatMap((page) => page.items) ?? [];
  const selected = entries.find((entry) => entry.id === selectedId) ?? null;
  const todayKey = new Date().toDateString();
  const sections = [
    { key: "today", rows: entries.filter((e) => new Date(e.created_at).toDateString() === todayKey) },
    { key: "earlier", rows: entries.filter((e) => new Date(e.created_at).toDateString() !== todayKey) },
  ].filter((section) => section.rows.length > 0);

  async function mutate(path: string, init: RequestInit) {
    if (busy) return false;
    setBusy(true);
    setError("");
    try {
      await api(path, init);
      await client.invalidateQueries({ queryKey: notificationKey });
      return true;
    } catch (err) {
      setError((err as Error).message);
      return false;
    } finally { setBusy(false); }
  }

  async function open(entry: Notification) {
    setSelectedId(entry.id);
    if (box === "incoming" && !entry.read_at) await mutate(`/api/v1/notifications/${entry.id}`, { method: "PATCH", body: JSON.stringify({ read: true }) });
  }

  const eventText = (entry: Notification) => t(`nav.inbox.event.${entry.kind}`, {
    actor: entry.actor?.name ?? t("nav.inbox.someone"), title: entry.item_title ?? entry.space_title,
  });

  return <main className="space-y-4 p-6">
    <h1 className="workspace-title">{t("nav.inbox.label")}</h1>
    <div role="tablist" aria-label={t("nav.inbox.tabs")} className="flex gap-1 border-b">
      {boxes.map((value) => <button key={value} type="button" role="tab" aria-selected={box === value}
        className={`-mb-px px-3 py-2 text-sm ${box === value ? "border-b-2 border-primary font-medium" : "text-muted-foreground hover:text-foreground"}`}
        onClick={() => { setSelectedId(null); void navigate({ to: "/inbox", search: (current) => ({ ...current, box: value === "incoming" ? undefined : value }) }); }}>
        {t(`nav.inbox.tab.${value}`)}</button>)}
    </div>
    {box === "incoming" && <div className="flex flex-wrap items-center gap-3">
      <label className="flex items-center gap-2"><input type="checkbox" checked={!!unread}
        onChange={(event) => void navigate({ to: "/inbox", search: (current) => ({ ...current, unread: event.target.checked || undefined }) })} />
        {t("nav.inbox.unreadOnly")}</label>
      <Button variant="outline" size="sm" disabled={busy || !query.data?.pages[0]?.unread_count}
        onClick={() => void mutate("/api/v1/notifications/read-all", { method: "POST" })}>{t("nav.inbox.readAll")}</Button>
      <Button variant="outline" size="sm" disabled={busy || (!entries.length && !unread)}
        onClick={() => setConfirmArchive(true)}>{t("nav.inbox.archiveAll")}</Button>
    </div>}
    {!confirmArchive && <FormError message={error} />}
    {query.isPending && <p role="status">{t("items.loading")}</p>}
    {query.isError && <div><FormError message={t("items.loadError")} />
      <Button variant="link" onClick={() => void query.refetch()}>{t("items.retry")}</Button></div>}
    {query.isSuccess && !entries.length && <div className="flex flex-col items-center gap-2 py-16 text-center text-muted-foreground">
      <span aria-hidden="true" className="grid size-16 place-items-center rounded-full bg-primary/10 text-primary">{box === "archive" ? <Archive className="size-8" /> : <Bell className="size-8" />}</span>
      <p className="font-medium text-foreground">{t(box === "archive" ? "nav.inbox.emptyArchive" : box === "sent" ? "nav.inbox.emptySent" : "nav.inbox.empty")}</p>
      {box === "incoming" && <p className="text-sm">{t("nav.inbox.emptyHint")}</p>}
    </div>}
    <div className="grid gap-4 lg:grid-cols-[minmax(0,26rem)_minmax(0,1fr)]">
      <div className={selected ? "hidden lg:block" : ""}>
        {sections.map((section) => <section key={section.key} aria-label={t(`nav.inbox.group.${section.key}`)}>
          <h2 className="mt-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{t(`nav.inbox.group.${section.key}`)}</h2>
          <ul className="divide-y">
            {section.rows.map((entry) => <li key={entry.id} className={`flex items-center gap-2 ${entry.id === selectedId ? "border-l-2 border-primary" : ""}`}>
        <button type="button" disabled={busy} onClick={() => void open(entry)}
          className="flex min-w-0 flex-1 items-start gap-3 rounded p-3 text-left hover:bg-muted/40 focus-visible:outline-ring">
          <span aria-hidden="true" className="grid size-8 shrink-0 place-items-center rounded-full bg-muted text-xs font-medium">
            {entry.actor?.name.slice(0, 1)}
          </span>
          <span className="min-w-0 flex-1">
            <span className="block break-words text-sm">{eventText(entry)}</span>
            {entry.snippet && <span className="block truncate text-sm text-muted-foreground">{entry.snippet}</span>}
            <time dateTime={entry.created_at} title={formatDateTime(entry.created_at, profile)} className="text-xs text-muted-foreground">{relative(entry.created_at, i18n.language)}</time>
          </span>
          {!entry.read_at && <span className="mt-2 size-2 shrink-0 rounded-full bg-primary">
            <span className="sr-only">{t("nav.inbox.unread")}</span>
          </span>}
        </button>
        {box === "incoming" && <Button variant="ghost" size="sm" disabled={busy} onClick={() => void mutate(`/api/v1/notifications/${entry.id}`, {
          method: "PATCH", body: JSON.stringify({ archived: true }),
        })}>{t("nav.inbox.archive")}</Button>}
      </li>
            )}
          </ul></section>)}
      </div>
      <aside aria-label={t("nav.inbox.label")} className={`rounded border p-4 ${selected ? "" : "hidden lg:block"}`}>
        {selected ? <div className="space-y-3">
          <Button variant="link" className="px-0 lg:hidden" onClick={() => setSelectedId(null)}>{t("nav.inbox.back")}</Button>
          <p className="text-sm font-medium">{eventText(selected)}</p>
          {selected.recipient && box === "sent" && <p className="text-sm text-muted-foreground">{t("nav.inbox.sentTo", { name: selected.recipient.name })}</p>}
          {selected.snippet && <p className="whitespace-pre-wrap text-sm">{selected.snippet}</p>}
          <time className="block text-xs text-muted-foreground" dateTime={selected.created_at}>{formatDateTime(selected.created_at, profile)}</time>
          {selected.item_id && <Link className="text-sm underline" to="/tasks/$itemId" params={{ itemId: selected.item_id }}>{t("nav.inbox.open")}</Link>}
          {!selected.item_id && selected.space_id && <Link className="text-sm underline" to="/spaces/$spaceId" params={{ spaceId: selected.space_id }}>{t("nav.inbox.open")}</Link>}
        </div> : <p className="text-sm text-muted-foreground">{t("nav.inbox.select")}</p>}
      </aside>
    </div>
    {query.hasNextPage && <Button variant="outline" disabled={query.isFetchingNextPage}
      onClick={() => void query.fetchNextPage()}>{t("nav.inbox.more")}</Button>}
    <ConfirmDialog open={confirmArchive} onOpenChange={setConfirmArchive} title={t("nav.inbox.archiveAll")}
      description={t("nav.inbox.archiveDescription")} confirmLabel={t("nav.inbox.archiveAll")}
      cancelLabel={t("items.cancel")} busy={busy} error={error}
      onConfirm={async () => {
        if (await mutate("/api/v1/notifications/archive-all", { method: "POST" })) setConfirmArchive(false);
      }} />
  </main>;
}
