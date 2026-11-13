import { formatDate } from "@/lib/profile";
import { lazy, Suspense, useEffect, useId, useRef, useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { MoreHorizontal } from "lucide-react";
import { StarButton } from "@/components/star-button";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { components } from "@/api/schema";
import { recycleKey, restoreItem } from "@/lib/recycle";
import { ApiError, api, type CurrentUser } from "@/lib/api";
import { getItem, isProjectDetail, isWorkday, itemKeys, listItems, type Item, type ItemUpdate } from "@/lib/items";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { FormError } from "@/components/auth/form-error";
import { CreateItemDialog } from "@/components/create-item-dialog";
import { ProjectExcelExport } from "@/components/project-excel-export";
import { SaveBlueprintDialog } from "@/components/save-blueprint-dialog";
import { ItemUserPicker, type PersonOption } from "@/components/item-user-picker";
import { ItemDates } from "@/components/item-dates";
import { ItemEffort } from "@/components/item-effort";
import { ItemLocations } from "@/components/item-locations";
import { ItemApprovals } from "@/components/item-approvals";
import { ItemActivity } from "@/components/item-activity";
import { ItemCustomFields } from "@/components/item-custom-fields";
import { ItemAttachments } from "@/components/item-attachments";
import { ItemProgress } from "@/components/project-progress";
const ItemDescription = lazy(async () => ({ default: (await import("@/components/item-description")).ItemDescription }));
const ItemComments = lazy(async () => ({ default: (await import("@/components/item-comments")).ItemComments }));

type Props = { id: string; me: CurrentUser; onOpen: (id: string) => void; onDeleted: () => void };

const importanceLevels = ["high", "normal", "low"] as const;

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
  return <ItemEditor key={`${props.id}:${revision}`} {...props} detail={query.data}
    reset={() => setRevision((value) => value + 1)} />;
}

function ItemEditor({ detail, me, onOpen, onDeleted, reset }: Omit<Props, "id"> & {
  detail: Awaited<ReturnType<typeof getItem>>;
  reset: () => void;
}) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const statusInput = useId();
  const { item, etag } = detail;
  const titleInput = useRef<HTMLInputElement>(null);
  const draftVersions = useRef<Partial<Record<keyof ItemUpdate, string>>>({});
  function beginDraft(field: keyof ItemUpdate) {
    if (etag) draftVersions.current[field] ??= etag;
  }
  useEffect(() => {
    if (titleInput.current && !draftVersions.current.title) titleInput.current.value = item.title;
  }, [item.title]);
  const caps = new Set(item.my_capabilities);
  const canEdit = caps.has("edit") && !!etag;
  const canStatus = caps.has("change_status") && !!etag;
  const canDelete = caps.has("delete");
  const canCreate = caps.has("create");
  const [busy, setBusy] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [savingBlueprint, setSavingBlueprint] = useState(false);
  const saving = useRef(false);
  const [error, setError] = useState("");
  const [conflictDraft, setConflictDraft] = useState<ItemUpdate>();
  const stale = !!conflictDraft;
  const [deleting, setDeleting] = useState(false);
  const [adding, setAdding] = useState(false);
  const [assigneeDraft, setAssigneeDraft] = useState<PersonOption[]>();
  const assignees = assigneeDraft ?? item.assignees;
  const path = `/api/v1/items/${item.id}`;
  const children = useQuery({ queryKey: itemKeys.children(item.id), queryFn: () => listItems(path + "/children").catch((err) => {
    // A requester who can open only this item gets 404 for its children: show none, not an error.
    if (err instanceof ApiError && err.status === 404) return [];
    throw err;
  }) });
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

  async function save(update: ItemUpdate, reapply = false, draftField?: keyof ItemUpdate) {
    if (saving.current || (!reapply && (stale || !etag))) return false;
    saving.current = true;
    setBusy(true);
    setError("");
    try {
      const fields = Object.keys(update) as (keyof ItemUpdate)[];
      const draftKey = draftField ?? fields[0];
      const expected = reapply ? (await getItem(item.id)).etag : draftVersions.current[draftKey] ?? etag;
      if (!expected) throw new Error(t("items.stale"));
      let tag: string | null = null;
      const updated = await api<Item>(path, {
        method: "PATCH", headers: { "If-Match": expected }, body: JSON.stringify(update),
      }, (headers) => { tag = headers.get("ETag"); });
      for (const field of fields) delete draftVersions.current[field];
      delete draftVersions.current[draftKey];
      if (update.assignee_ids) setAssigneeDraft(undefined);
      qc.setQueryData(itemKeys.detail(item.id), { item: updated, etag: tag });
      await qc.invalidateQueries({ queryKey: itemKeys.all,
        predicate: (query) => query.queryKey[1] !== "detail" || (query.queryKey[2] !== item.id && isProjectDetail(query)),
      });
      setConflictDraft(undefined);
      if (reapply) reset();
      return true;
    } catch (err) {
      if (err instanceof ApiError && (err.status === 409 || err.status === 428)) {
        const statusOnly = !!update.status_id;
        if (!statusOnly) setConflictDraft(update);
        try {
          qc.setQueryData(itemKeys.detail(item.id), await getItem(item.id));
          await qc.invalidateQueries({ queryKey: itemKeys.all, predicate: ({ queryKey }) => queryKey[1] !== "detail" });
          if (statusOnly) toast.error(t("items.statusConflict"));
        } catch (refreshError) { setError((refreshError as Error).message); }
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
      await qc.invalidateQueries({ queryKey: itemKeys.all,
        predicate: (query) => query.queryKey[1] !== "detail" || (query.queryKey[2] !== item.id && isProjectDetail(query)),
      });
      void qc.invalidateQueries({ queryKey: recycleKey });
      toast.success(t("nav.recycle.deleted"), { duration: 8000, action: {
        label: t("nav.recycle.undo"),
        onClick: () => void restoreItem(qc, item.id).then(
          () => toast.success(t("nav.recycle.restored")),
          (err: Error) => toast.error(err.message),
        ),
      } });
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
    {parentId && !parent.isError && <Button size="sm" variant="ghost" onClick={() => onOpen(parentId)}>{parent.data?.item.title ?? t("items.parentLink")}</Button>}
    <div className="flex items-start justify-between gap-2">
      <h2 className="min-w-0 break-words text-xl font-semibold">{item.title}</h2>
      <div className="flex shrink-0 items-center">
      <StarButton itemId={item.id} />
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
          {item.type === "project" && item.locations.length > 0 && <DropdownMenuItem onClick={() => setExporting(true)}>{t("excel.exportTitle")}</DropdownMenuItem>}
          <DropdownMenuItem onClick={() => setSavingBlueprint(true)}>{t("blueprints.menu")}</DropdownMenuItem>
          {canDelete && <DropdownMenuItem disabled={busy} variant="destructive"
            onClick={() => { setError(""); setDeleting(true); }}>{t("items.delete")}</DropdownMenuItem>}
        </DropdownMenuContent>
      </DropdownMenu>
      </div>
    </div>
    <FormError message={error} />
    {conflictDraft && <div className="space-y-2 rounded border border-warning bg-warning-subtle p-2">
      <p role="alert">{t("items.stale")}</p>
      <div className="flex flex-wrap gap-2">
        <Button disabled={busy} onClick={() => void save(conflictDraft, true)}>{t("items.reapply")}</Button>
        <Button variant="outline" disabled={busy} onClick={reset}>{t("items.discardChanges")}</Button>
      </div>
    </div>}
    {canEdit && <form className="flex items-end gap-2" onSubmit={(event) => {
      event.preventDefault();
      const title = String(new FormData(event.currentTarget).get("title") ?? "").trim();
      if (!title) return setError(t("items.nameRequired"));
      if (title !== item.title) void save({ title });
    }}>
      <label className="min-w-0 flex-1 space-y-1"><span>{t("items.name")}</span>
        <Input ref={titleInput} name="title" defaultValue={item.title} onChange={() => beginDraft("title")} maxLength={500} required /></label>
      <Button disabled={busy || stale}>{t("items.save")}</Button>
    </form>}
    {canStatus ? <div className="space-y-1">
      <label htmlFor={statusInput}>{t("items.status")}</label>
      <select id={statusInput} className="block w-full rounded border bg-background p-2" value={item.status?.id ?? ""}
        disabled={busy || stale || !statuses.length} onChange={(event) => void save({ status_id: event.target.value })}>
        {!item.status && <option value="" disabled>{t("items.status")}</option>}
        {!statuses.some((status) => status.id === item.status?.id) && item.status && <option value={item.status.id}>{item.status.name}</option>}
        {statuses.map((status) => <option key={status.id} value={status.id}>{status.name}</option>)}
      </select>
    </div> : <dl><dt>{t("items.status")}</dt><dd>{item.status?.name}</dd></dl>}
    {canEdit ? <div className="space-y-1">
      <label htmlFor="item-importance">{t("items.importance.label")}</label>
      <select id="item-importance" className="block w-full rounded border bg-background p-2" value={item.importance ?? "normal"}
        disabled={busy || stale} onChange={(event) => void save({ importance: event.target.value as NonNullable<Item["importance"]> })}>
        {importanceLevels.map((level) => <option key={level} value={level}>{t(`items.importance.${level}`)}</option>)}
      </select>
    </div> : <dl><dt>{t("items.importance.label")}</dt><dd>{t(`items.importance.${item.importance ?? "normal"}`)}</dd></dl>}
    <ItemProgress item={item} editable={canEdit} disabled={busy || stale}
      onEdit={() => beginDraft("progress_percent")} onSave={(update) => save(update)} />
    {canEdit ? <form className="space-y-2" onSubmit={saveDates}>
      <ItemDates key={draftVersions.current.start_date ?? etag} initialStart={item.start_date}
        initialDue={item.due_date} onEdit={() => beginDraft("start_date")} />
      <Button className="w-fit" variant="outline" disabled={busy || stale}>{t("items.save")}</Button>
    </form> : <dl className="grid grid-cols-2 gap-2">
      {item.start_date && <div><dt>{t("items.start")}</dt><dd>{formatDate(item.start_date, me)}</dd></div>}
      {item.due_date && <div><dt>{t("items.due")}</dt><dd>{formatDate(item.due_date, me)}</dd></div>}
    </dl>}
    {canEdit ? <div className="space-y-2">
      <ItemUserPicker multiple label={t("items.assignees")} selected={assignees} onChange={(people) => {
        beginDraft("assignee_ids");
        setAssigneeDraft(people);
      }} />
      <Button variant="outline" disabled={busy || stale} onClick={() => void save({ assignee_ids: assignees.map((person) => person.id) })}>{t("items.save")}</Button>
    </div> : <section><h3>{t("items.assignees")}</h3><p>{item.assignees.map((person) => person.name).join(", ")}</p></section>}
    <ItemEffort item={item} editable={canEdit} disabled={busy || stale}
      onEdit={() => beginDraft("effort_minutes")}
      onSave={(update) => save(update, false, "effort_minutes")} />
    <ItemLocations item={item} editable={canEdit} disabled={busy || stale} />
    <ItemCustomFields item={item} profile={me} editable={canEdit} disabled={busy || stale}
      onEdit={() => {
        delete draftVersions.current.fields;
        beginDraft("fields");
      }}
      onCancel={() => { delete draftVersions.current.fields; }}
      onSave={(fieldId, value) => save({ fields: { [fieldId]: value } })} />
    <Suspense fallback={<p role="status">{t("items.loading")}</p>}>
      <ItemDescription description={item.description} editable={canEdit && !stale} busy={busy} save={save}
        onEdit={() => beginDraft("description")} />
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
    <Suspense fallback={null}><ItemComments item={item} userId={me.id} /></Suspense>
    <ItemApprovals item={item} userId={me.id} />
    <ItemActivity item={item} />
    <ItemAttachments item={item} userId={me.id} />
    {childLocation && <CreateItemDialog open={adding} onOpenChange={setAdding} me={me} location={childLocation} onCreated={(child) => onOpen(child.id)} />}
    {exporting && <ProjectExcelExport open={exporting} onOpenChange={setExporting} item={item} />}
    <SaveBlueprintDialog open={savingBlueprint} onOpenChange={setSavingBlueprint} itemID={item.id} title={item.title} />
    <ConfirmDialog open={deleting} onOpenChange={setDeleting} title={t("items.delete")} description={t("items.deleteDescription")}
      confirmLabel={t("items.delete")} cancelLabel={t("items.cancel")} onConfirm={remove} busy={busy} error={error} />
  </article>;
}
