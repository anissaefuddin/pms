import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { FileText, Folder, Trash2 } from "lucide-react";
import { api } from "@/lib/api";
import { formatDateTime, useProfile } from "@/lib/profile";
import { emptyRecycleBin, recycleKey, restoreItem } from "@/lib/recycle";
import type { ItemPage } from "@/lib/items";
import { Button } from "@/components/ui/button";
import { FormError } from "@/components/auth/form-error";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminEmptyState, AdminToolbar } from "@/components/admin/admin-ui";
import { Input } from "@/components/ui/input";

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
  const [query, setQuery] = useState("");
  const [space, setSpace] = useState("");
  const items = bin.data?.pages.flatMap((page) => page.items) ?? [];
  const spaces = [...new Set(items.flatMap((item) => item.locations.flatMap((location) => location.space_name ? [location.space_name] : [])))].sort();
  const visible = items.filter((item) => (!query || item.title.toLowerCase().includes(query.toLowerCase()))
    && (!space || item.locations.some((location) => location.space_name === space)));
  return <main className="space-y-4 p-6">
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="space-y-1">
        <h1 className="workspace-title">{t("nav.recycle.label")}</h1>
        <p className="text-sm text-muted-foreground">{t("nav.recycle.description")}</p>
      </div>
      {items.length > 0 && <Button variant="secondary" size="sm" onClick={() => setConfirming(true)}><Trash2 />{t("nav.recycle.purge.action")}</Button>}
    </div>
    <AdminToolbar searchLabel={t("nav.recycle.searchPlaceholder")} search={query} onSearch={setQuery}>
      <select aria-label={t("nav.recycle.spaceFilter")} className="h-9 rounded-md border border-input bg-background px-2 text-sm" value={space} onChange={(event) => setSpace(event.target.value)}>
        <option value="">{t("nav.recycle.allSpaces")}</option>
        {spaces.map((name) => <option key={name} value={name}>{name}</option>)}
      </select>
    </AdminToolbar>
    <div className="flex flex-wrap gap-3 border-b pb-3 text-sm">
      <label className="flex items-center gap-2">{t("items.type")}
        <select className="rounded-md border-0 bg-muted px-2 py-1 text-sm" value={type} onChange={(event) => setType(event.target.value)}>
          <option value="">{t("nav.recycle.allTypes")}</option>
          {types.map((kind) => <option key={kind} value={kind}>{t(`items.${kind}`)}</option>)}
        </select>
      </label>
      <label className="flex items-center gap-2">{t("views.sort")}
        <select className="rounded-md border-0 bg-muted px-2 py-1 text-sm" value={sort} onChange={(event) => setSort(event.target.value)}>
          <option value="deleted_at">{t("nav.recycle.deletedAt")}</option>
          <option value="title">{t("views.title")}</option>
        </select>
      </label>
    </div>
    {restore.error && <FormError message={restore.error.message} />}
    {bin.isPending && <p role="status">{t("items.loading")}</p>}
    {bin.isError && <div><FormError message={t("items.loadError")} />
      <Button variant="link" onClick={() => void bin.refetch()}>{t("items.retry")}</Button></div>}
    {bin.isSuccess && !items.length && <AdminEmptyState title={t("nav.recycle.empty")} description={t("nav.recycle.emptyDescription")} />}
    {bin.isSuccess && !!items.length && !visible.length && <p role="status" className="text-sm text-muted-foreground">{t("nav.recycle.noMatch")}</p>}
    <ul className="divide-y">{visible.map((item) => <li key={item.id} className="flex flex-wrap items-center gap-3 py-2">
      <span aria-hidden="true" className="grid size-8 shrink-0 place-items-center rounded-md bg-muted text-muted-foreground">{item.type === "folder" ? <Folder className="size-4" /> : <FileText className="size-4" />}</span>
      <div className="min-w-0 flex-1">
        <p className="break-words font-medium">{item.title}</p>
        <p className="text-sm text-muted-foreground">{[
          t(`items.${item.type}`),
          ...new Set(item.locations.flatMap((location) => location.space_name ? [location.space_name] : [])),
          item.deleted_by?.name,
          item.deleted_at && formatDateTime(item.deleted_at, me),
        ].filter(Boolean).join(" · ")}</p>
        {item.restore_blocked_reason && <p className="flex items-center gap-1.5 text-sm"><span aria-hidden="true">!</span>{t(`nav.recycle.blocked.${item.restore_blocked_reason}`)}</p>}
      </div>
      <Button variant="outline" size="sm" disabled={restore.isPending || !!item.restore_blocked_reason}
        onClick={() => restore.mutate(item.id)}>{t("nav.recycle.restore")}</Button>
    </li>)}</ul>
    {bin.hasNextPage && <Button variant="outline" disabled={bin.isFetchingNextPage}
      onClick={() => void bin.fetchNextPage()}>{t("nav.recycle.more")}</Button>}
    <ConfirmDialog open={confirming} onOpenChange={setConfirming} title={t("nav.recycle.purge.title")}
      description={t("nav.recycle.purge.body")} confirmLabel={t("nav.recycle.purge.confirm")} cancelLabel={t("items.cancel")}
      busy={empty.isPending} error={empty.error?.message} onConfirm={() => empty.mutate()} />
  </main>;
}
