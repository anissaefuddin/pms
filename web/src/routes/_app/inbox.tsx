import { formatDateTime, useProfile } from "@/lib/profile";
import { useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { listNotifications, notificationKey, type Notification } from "@/lib/notifications";
import { Button } from "@/components/ui/button";
import { FormError } from "@/components/auth/form-error";
import { ConfirmDialog } from "@/components/confirm-dialog";

export const Route = createFileRoute("/_app/inbox")({
  validateSearch: (search: Record<string, unknown>): { unread?: boolean } => ({ unread: search.unread === true ? true : undefined }),
  component: Inbox,
});

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
  const { unread } = Route.useSearch();
  const navigate = useNavigate();
  const client = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [confirmArchive, setConfirmArchive] = useState(false);
  const [error, setError] = useState("");
  const query = useInfiniteQuery({
    queryKey: [...notificationKey, "inbox", !!unread],
    initialPageParam: "",
    queryFn: ({ pageParam }) => listNotifications(30, unread, pageParam),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  const entries = query.data?.pages.flatMap((page) => page.items) ?? [];

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
    if (!entry.read_at && !await mutate(`/api/v1/notifications/${entry.id}`, { method: "PATCH", body: JSON.stringify({ read: true }) })) return;
    if (entry.item_id) await navigate({ to: "/inbox", search: (current) => ({ ...current, task: entry.item_id! }) });
    if (entry.space_id) await navigate({ to: "/spaces/$spaceId", params: { spaceId: entry.space_id } });
  }

  return <main className="mx-auto max-w-4xl space-y-4 p-6">
    <h1 className="workspace-title">{t("nav.inbox.label")}</h1>
    <div className="flex flex-wrap items-center gap-3">
      <label className="flex items-center gap-2"><input type="checkbox" checked={!!unread}
        onChange={(event) => void navigate({ to: "/inbox", search: (current) => ({ ...current, unread: event.target.checked || undefined }) })} />
        {t("nav.inbox.unreadOnly")}</label>
      <Button variant="outline" size="sm" disabled={busy || !query.data?.pages[0]?.unread_count}
        onClick={() => void mutate("/api/v1/notifications/read-all", { method: "POST" })}>{t("nav.inbox.readAll")}</Button>
      <Button variant="outline" size="sm" disabled={busy || (!entries.length && !unread)}
        onClick={() => setConfirmArchive(true)}>{t("nav.inbox.archiveAll")}</Button>
    </div>
    {!confirmArchive && <FormError message={error} />}
    {query.isPending && <p role="status">{t("items.loading")}</p>}
    {query.isError && <div><FormError message={t("items.loadError")} />
      <Button variant="link" onClick={() => void query.refetch()}>{t("items.retry")}</Button></div>}
    {query.isSuccess && !entries.length && <p className="text-muted-foreground">{t("nav.inbox.empty")}</p>}
    <ul className="divide-y">
      {entries.map((entry) => <li key={entry.id} className="flex items-center gap-2">
        <button type="button" disabled={busy} onClick={() => void open(entry)}
          className="flex min-w-0 flex-1 items-start gap-3 rounded p-3 text-left hover:bg-accent focus-visible:outline-ring">
          <span aria-hidden="true" className="grid size-8 shrink-0 place-items-center rounded-full bg-muted text-xs font-medium">
            {entry.actor?.name.slice(0, 1)}
          </span>
          <span className="min-w-0 flex-1">
            <span className="block break-words text-sm">{t(`nav.inbox.event.${entry.kind}`, {
              actor: entry.actor?.name ?? t("nav.inbox.someone"), title: entry.item_title ?? entry.space_title,
            })}</span>
            {entry.snippet && <span className="block truncate text-sm text-muted-foreground">{entry.snippet}</span>}
            <time dateTime={entry.created_at} title={formatDateTime(entry.created_at, profile)} className="text-xs text-muted-foreground">{relative(entry.created_at, i18n.language)}</time>
          </span>
          {!entry.read_at && <span className="mt-2 size-2 shrink-0 rounded-full bg-primary">
            <span className="sr-only">{t("nav.inbox.unread")}</span>
          </span>}
        </button>
        <Button variant="ghost" size="sm" disabled={busy} onClick={() => void mutate(`/api/v1/notifications/${entry.id}`, {
          method: "PATCH", body: JSON.stringify({ archived: true }),
        })}>{t("nav.inbox.archive")}</Button>
      </li>)}
    </ul>
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
