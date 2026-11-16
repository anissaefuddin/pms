import { useId } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import type { Item } from "@/lib/items";
import { formatDate, formatDateTime, useProfile } from "@/lib/profile";
import { formatChange, groupByDay } from "@/lib/activity-format";
import { AvatarStack } from "@/components/ui/avatar-stack";
import { Button } from "@/components/ui/button";

type Activity = components["schemas"]["Activity"];
type Page = { items: Activity[]; next_cursor?: string | null };
const known = ["update", "delete", "restore", "detach", "create"];

/** The item's change history (SPC-07 activity log), newest first. */
export function ItemActivity({ item }: { item: Item }) {
  const { t } = useTranslation();
  const heading = useId();
  const profile = useProfile();
  const log = useInfiniteQuery({
    queryKey: ["item-activity", item.id, item.version],
    initialPageParam: "",
    queryFn: ({ pageParam }) => api<Page>(`/api/v1/items/${item.id}/activity?limit=20${pageParam ? `&cursor=${pageParam}` : ""}`),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
  const entries = log.data?.pages.flatMap((page) => page.items) ?? [];
  return <section aria-labelledby={heading} className="space-y-2">
    <h3 id={heading} className="font-semibold">{t("items.activity.title")}</h3>
    {log.isError && <p role="alert">{t("items.activity.error")}</p>}
    {log.isSuccess && !entries.length && <p className="text-sm text-muted-foreground">{t("items.activity.empty")}</p>}
    {groupByDay(entries, profile?.timezone).map((group) => <section key={group.day} aria-label={formatDate(group.day, profile)} className="space-y-1">
      <h4 className="text-xs font-medium text-muted-foreground">{formatDate(group.day, profile)}</h4>
      <ul className="space-y-2 text-sm">{group.items.map((entry) => <li key={entry.id} className="flex items-start gap-2">
        <AvatarStack people={[{ id: entry.actor.id, name: entry.actor.name }]} max={1} className="mt-0.5" />
        <div className="min-w-0 space-y-0.5">
          <p>{t(`items.activity.action.${known.includes(entry.action) ? entry.action : "other"}`, { actor: entry.actor.name })}</p>
          {entry.action === "update" && Object.entries(entry.changes).map(([field, change]) => <p key={field} className="text-muted-foreground">
            <span className="font-medium text-foreground">{field}</span>: {formatChange(change as { from?: unknown; to?: unknown })}
          </p>)}
          <p className="text-xs text-muted-foreground">{formatDateTime(entry.created_at, profile)}</p>
        </div>
      </li>)}</ul>
    </section>)}
    {log.hasNextPage && <Button variant="outline" disabled={log.isFetchingNextPage} onClick={() => void log.fetchNextPage()}>{t("items.activity.more")}</Button>}
  </section>;
}
