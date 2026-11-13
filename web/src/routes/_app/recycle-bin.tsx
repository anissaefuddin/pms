import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { formatDateTime, useProfile } from "@/lib/profile";
import { emptyRecycleBin, recycleKey, restoreItem } from "@/lib/recycle";
import type { ItemPage } from "@/lib/items";
import { Button } from "@/components/ui/button";
import { FormError } from "@/components/auth/form-error";
import { ConfirmDialog } from "@/components/confirm-dialog";

export const Route = createFileRoute("/_app/recycle-bin")({ component: RecycleBin });
const types = ["task", "project", "folder"] as const;

function RecycleBin() {
  const { t } = useTranslation();
  const me = useProfile();
  const client = useQueryClient();
  const [type, setType] = useState("");
  const [sort, setSort] = useState("deleted_at");
  const bin = useInfiniteQuery({
    queryKey: [...recycleKey, type, sort],
    initialPageParam: "",
    queryFn: ({ pageParam }) => {
      const params = new URLSearchParams({ sort });
      if (type) params.set("type", type);
      if (pageParam) params.set("cursor", pageParam);
      return api<ItemPage>(`/api/v1/recycle-bin?${params}`);
    },
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
  const restore = useMutation({ mutationFn: (id: string) => restoreItem(client, id) });
  const [confirming, setConfirming] = useState(false);
  const empty = useMutation({ mutationFn: () => emptyRecycleBin(client), onSuccess: () => setConfirming(false) });
  const items = bin.data?.pages.flatMap((page) => page.items) ?? [];
  return <main className="space-y-4 p-6">
    <h1 className="text-xl font-semibold">{t("nav.recycle.label")}</h1>
    <div className="flex flex-wrap gap-3">
      <label className="flex items-center gap-2">{t("items.type")}
        <select className="rounded border bg-background p-2 text-sm" value={type} onChange={(event) => setType(event.target.value)}>
          <option value="">{t("nav.recycle.allTypes")}</option>
          {types.map((kind) => <option key={kind} value={kind}>{t(`items.${kind}`)}</option>)}
        </select>
      </label>
      <label className="flex items-center gap-2">{t("views.sort")}
        <select className="rounded border bg-background p-2 text-sm" value={sort} onChange={(event) => setSort(event.target.value)}>
          <option value="deleted_at">{t("nav.recycle.deletedAt")}</option>
          <option value="title">{t("views.title")}</option>
        </select>
      </label>
    </div>
    {items.length > 0 && <Button variant="outline" onClick={() => setConfirming(true)}>{t("nav.recycle.purge.action")}</Button>}
    {restore.error && <FormError message={restore.error.message} />}
    {bin.isPending && <p role="status">{t("items.loading")}</p>}
    {bin.isError && <div><FormError message={t("items.loadError")} />
      <Button variant="link" onClick={() => void bin.refetch()}>{t("items.retry")}</Button></div>}
    {bin.isSuccess && !items.length && <p>{t("nav.recycle.empty")}</p>}
    <ul className="divide-y">{items.map((item) => <li key={item.id} className="flex flex-wrap items-center gap-3 py-3">
      <div className="min-w-0 flex-1">
        <p className="break-words font-medium">{item.title}</p>
        <p className="text-sm text-muted-foreground">{[
          t(`items.${item.type}`),
          ...new Set(item.locations.flatMap((location) => location.space_name ? [location.space_name] : [])),
          item.deleted_by?.name,
          item.deleted_at && formatDateTime(item.deleted_at, me),
        ].filter(Boolean).join(" · ")}</p>
        {item.restore_blocked_reason && <p className="text-sm">{t(`nav.recycle.blocked.${item.restore_blocked_reason}`)}</p>}
      </div>
      <Button variant="outline" disabled={restore.isPending || !!item.restore_blocked_reason}
        onClick={() => restore.mutate(item.id)}>{t("nav.recycle.restore")}</Button>
    </li>)}</ul>
    {bin.hasNextPage && <Button variant="outline" disabled={bin.isFetchingNextPage}
      onClick={() => void bin.fetchNextPage()}>{t("nav.recycle.more")}</Button>}
    <ConfirmDialog open={confirming} onOpenChange={setConfirming} title={t("nav.recycle.purge.title")}
      description={t("nav.recycle.purge.body")} confirmLabel={t("nav.recycle.purge.confirm")} cancelLabel={t("items.cancel")}
      busy={empty.isPending} error={empty.error?.message} onConfirm={() => empty.mutate()} />
  </main>;
}
