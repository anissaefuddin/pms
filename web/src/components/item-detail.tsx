import { lazy, Suspense, useId, useRef, useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { MoreHorizontal } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { components } from "@/api/schema";
import { ApiError, api, type CurrentUser } from "@/lib/api";
import { getItem, isWorkday, itemKeys, listItems, type Item, type ItemUpdate } from "@/lib/items";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { FormError } from "@/components/auth/form-error";
import { CreateItemDialog } from "@/components/create-item-dialog";
import { ItemUserPicker, type PersonOption } from "@/components/item-user-picker";
import { ItemDates } from "@/components/item-dates";
const ItemDescription = lazy(async () => ({ default: (await import("@/components/item-description")).ItemDescription }));

type Props = { id: string; me: CurrentUser; onOpen: (id: string) => void; onDeleted: () => void };

export function ItemDetail(props: Props) {
  const { t } = useTranslation();
  const query = useQuery({
    queryKey: itemKeys.detail(props.id), queryFn: () => getItem(props.id),
    staleTime: Infinity,
  });
  const [revision, setRevision] = useState(0);
  if (query.isPending) return <p role="status">{t("items.loading")}</p>;
  if (query.isError) return <div className="space-y-2">
    <FormError message={t("items.loadError")} />
    <Button variant="outline" onClick={() => void query.refetch()}>{t("items.retry")}</Button>
  </div>;
  return <ItemEditor key={`${props.id}:${revision}`} {...props} detail={query.data} reload={async () => {
    const next = await query.refetch();
    if (next.isSuccess) setRevision((value) => value + 1);
  }} />;
}

function ItemEditor({ detail, me, onOpen, onDeleted, reload }: Omit<Props, "id"> & {
  detail: Awaited<ReturnType<typeof getItem>>;
  reload: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const statusInput = useId();
  const { item, etag } = detail;
  const caps = new Set(item.my_capabilities);
  const canEdit = caps.has("edit") && !!etag;
  const canStatus = caps.has("change_status") && !!etag;
  const canDelete = caps.has("delete");
  const canCreate = caps.has("create");
  const [busy, setBusy] = useState(false);
  const saving = useRef(false);
  const [error, setError] = useState("");
  const [stale, setStale] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [adding, setAdding] = useState(false);
  const [assignees, setAssignees] = useState<PersonOption[]>(item.assignees);
  const path = `/api/v1/items/${item.id}`;
  const children = useQuery({ queryKey: itemKeys.children(item.id), queryFn: () => listItems(path + "/children") });
  const parentId = item.locations.find((loc) => loc.parent_item_id)?.parent_item_id;
  const parent = useQuery({
    queryKey: itemKeys.detail(parentId ?? ""), queryFn: () => getItem(parentId!), enabled: !!parentId,
  });
  const workflows = useQuery({
    queryKey: ["workflows"], enabled: canStatus,
    queryFn: () => api<{ items: components["schemas"]["Workflow"][] }>("/api/v1/workflows"),
  });
  const statuses = (workflows.data?.items.find((workflow) => workflow.statuses.some((status) => status.id === item.status?.id))
    ?? workflows.data?.items.find((workflow) => workflow.is_default))?.statuses ?? [];
  const childLocation = item.locations[0] && { space_id: item.locations[0].space_id, parent_item_id: item.id };

  async function save(update: ItemUpdate) {
    if (saving.current || stale || !etag) return false;
    saving.current = true;
    setBusy(true);
    setError("");
    try {
      let tag: string | null = null;
      const updated = await api<Item>(path, {
        method: "PATCH", headers: { "If-Match": etag }, body: JSON.stringify(update),
      }, (headers) => { tag = headers.get("ETag"); });
      qc.setQueryData(itemKeys.detail(item.id), { item: updated, etag: tag });
      await qc.invalidateQueries({ queryKey: itemKeys.all, predicate: ({ queryKey }) => queryKey[1] !== "detail" });
      return true;
    } catch (err) {
      if (err instanceof ApiError && (err.status === 409 || err.status === 428)) {
        setStale(true);
        setError(t("items.stale"));
      } else setError((err as Error).message);
      return false;
    } finally {
      saving.current = false;
      setBusy(false);
    }
  }

  async function remove() {
    if (saving.current) return;
    saving.current = true;
    setBusy(true);
    setError("");
    try {
      await api(path, { method: "DELETE" });
      await qc.invalidateQueries({ queryKey: itemKeys.detail(item.id), exact: true, refetchType: "none" });
      await qc.invalidateQueries({ queryKey: itemKeys.all, predicate: ({ queryKey }) => queryKey[1] !== "detail" });
      onDeleted();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      saving.current = false;
      setBusy(false);
    }
  }

  function saveDates(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const start = String(data.get("start_date") ?? "");
    const due = String(data.get("due_date") ?? "");
    if (start && due && due < start) return setError(t("items.dateOrder"));
    if ([start, due].some((day) => day && !isWorkday(day))) return setError(t("items.workingDays"));
    void save({ start_date: start || null, due_date: due || null });
  }

  return <article className="space-y-5">
    {parentId && <Button size="sm" variant="ghost" onClick={() => onOpen(parentId)}>{parent.data?.item.title ?? t("items.parentLink")}</Button>}
    <div className="flex items-start justify-between gap-2">
      <h2 className="min-w-0 break-words text-xl font-semibold">{item.title}</h2>
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon" aria-label={t("items.actions")} />}><MoreHorizontal /></DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={async () => {
            try {
              await navigator.clipboard.writeText(new URL(`/tasks/${item.id}`, window.location.origin).href);
              toast.success(t("items.copied"));
            } catch (err) { setError((err as Error).message); }
          }}>{t("items.copyLink")}</DropdownMenuItem>
          {canEdit && item.type === "task" && !item.is_milestone && <DropdownMenuItem disabled={busy || stale}
            onClick={() => void save({ is_milestone: true })}>{t("items.milestone")}</DropdownMenuItem>}
          {canDelete && <DropdownMenuItem disabled={busy} variant="destructive"
            onClick={() => { setError(""); setDeleting(true); }}>{t("items.delete")}</DropdownMenuItem>}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
    <FormError message={error} />
    {stale && <Button variant="outline" onClick={() => void reload()}>{t("items.retry")}</Button>}
    {canEdit && <form className="flex items-end gap-2" onSubmit={(event) => {
      event.preventDefault();
      const title = String(new FormData(event.currentTarget).get("title") ?? "").trim();
      if (!title) return setError(t("items.nameRequired"));
      if (title !== item.title) void save({ title });
    }}>
      <label className="min-w-0 flex-1 space-y-1"><span>{t("items.name")}</span>
        <Input name="title" defaultValue={item.title} maxLength={500} required /></label>
      <Button disabled={busy || stale}>{t("items.save")}</Button>
    </form>}
    <div className="space-y-1">
      <label htmlFor={statusInput}>{t("items.status")}</label>
      <select id={statusInput} className="block w-full rounded border bg-background p-2" value={item.status?.id ?? ""}
        disabled={!canStatus || busy || stale || !statuses.length} onChange={(event) => void save({ status_id: event.target.value })}>
        {!item.status && <option value="" disabled>{t("items.status")}</option>}
        {!statuses.some((status) => status.id === item.status?.id) && item.status && <option value={item.status.id}>{item.status.name}</option>}
        {statuses.map((status) => <option key={status.id} value={status.id}>{status.name}</option>)}
      </select>
    </div>
    {canEdit ? <form className="space-y-2" onSubmit={saveDates}>
      <ItemDates initialStart={item.start_date} initialDue={item.due_date} />
      <Button className="w-fit" variant="outline" disabled={busy || stale}>{t("items.save")}</Button>
    </form> : <dl className="grid grid-cols-2 gap-2">
      {item.start_date && <div><dt>{t("items.start")}</dt><dd>{item.start_date}</dd></div>}
      {item.due_date && <div><dt>{t("items.due")}</dt><dd>{item.due_date}</dd></div>}
    </dl>}
    {canEdit ? <div className="space-y-2">
      <ItemUserPicker multiple label={t("items.assignees")} selected={assignees} onChange={setAssignees} />
      <Button variant="outline" disabled={busy || stale} onClick={() => void save({ assignee_ids: assignees.map((person) => person.id) })}>{t("items.save")}</Button>
    </div> : <section><h3>{t("items.assignees")}</h3><p>{item.assignees.map((person) => person.name).join(", ")}</p></section>}
    <Suspense fallback={<p role="status">{t("items.loading")}</p>}>
      <ItemDescription description={item.description} editable={canEdit && !stale} busy={busy} save={save} />
    </Suspense>
    <section className="space-y-2 border-t pt-4">
      <div className="flex items-center justify-between gap-2">
        <h3 className="font-medium">{t("items.children")}{children.data ? ` · ${children.data.length}` : ""}</h3>
        {canCreate && childLocation && <Button variant="outline" size="sm" onClick={() => setAdding(true)}>{t("items.addChild")}</Button>}
      </div>
      {children.isPending && <p role="status">{t("items.loading")}</p>}
      {children.isError && <FormError message={t("items.loadError")} />}
      {children.data?.length === 0 && <p className="text-muted-foreground">{t("items.noChildren")}</p>}
      <ul className="divide-y">{children.data?.map((child) => <li key={child.id}>
        <Button variant="ghost" className="h-auto w-full justify-between whitespace-normal text-left" onClick={() => onOpen(child.id)}>
          <span>{child.title}</span>
          {!!child.child_count && <span className="text-muted-foreground">{t("items.childCount", { count: child.child_count })}</span>}
        </Button>
      </li>)}</ul>
    </section>
    {childLocation && <CreateItemDialog open={adding} onOpenChange={setAdding} me={me} location={childLocation} onCreated={(child) => onOpen(child.id)} />}
    <ConfirmDialog open={deleting} onOpenChange={setDeleting} title={t("items.delete")} description={t("items.deleteDescription")}
      confirmLabel={t("items.delete")} cancelLabel={t("items.cancel")} onConfirm={remove} busy={busy} error={error} />
  </article>;
}
