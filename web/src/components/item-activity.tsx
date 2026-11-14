import { useId } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import type { Item } from "@/lib/items";
import { formatDateTime, useProfile } from "@/lib/profile";
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
    <ul className="space-y-1 text-sm">{entries.map((entry) => <li key={entry.id}>
      {t(`items.activity.action.${known.includes(entry.action) ? entry.action : "other"}`, { actor: entry.actor.name })}
      {entry.action === "update" && Object.keys(entry.changes).length > 0 && <> · {Object.keys(entry.changes).join(", ")}</>}
      <span className="text-muted-foreground"> · {formatDateTime(entry.created_at, profile)}</span>
    </li>)}</ul>
    {log.hasNextPage && <Button variant="outline" disabled={log.isFetchingNextPage} onClick={() => void log.fetchNextPage()}>{t("items.activity.more")}</Button>}
  </section>;
}
