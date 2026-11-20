import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ArrowDown, ArrowUp, MoreHorizontal } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { useSpaces } from "@/lib/space-queries";
import { can } from "@/lib/spaces";
import { AdminEmptyState, AdminStatus, AdminToolbar } from "@/components/admin/admin-ui";
import { CustomFieldDrawer, type FieldScope } from "@/components/custom-field-drawer";
import { FieldImpactList, type Field, type Impact } from "@/components/custom-field-form";
import { FormError } from "@/components/auth/form-error";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";

type Scope = "workspace" | "space";
type Status = "all" | "active" | "archived";
type SortKey = "name" | "type" | "status";

const SCOPES: Scope[] = ["workspace", "space"];
const STATUSES: Status[] = ["all", "active", "archived"];
const SORT_COLUMNS: [SortKey, string][] = [["name", "settings.customFields.col.name"], ["type", "settings.customFields.col.type"]];
const selectClass = "h-9 rounded-lg border border-input bg-background px-2 text-sm";

/**
 * Settings > Custom fields: every field the person may see, by scope (Workspace or one Space), with search, status, and sort.
 * Creating, editing, and archive confirmation share the same components as the Space dialog. Workspace fields need an administrator;
 * a Space's fields need the Space's full role, which the server checks on every write.
 */
/** The Workspace custom-field registry. `spaceId` pins it to one Space (C: Space Settings > Custom fields); the same components then manage that Space's fields. */
export function CustomFieldRegistry({ spaceId: pinnedSpaceId }: { spaceId?: string } = {}) {
  const { t, i18n } = useTranslation();
  const client = useQueryClient();
  const isAdmin = useProfile()?.user_type === "admin";
  const spaces = useSpaces();
  const manageable = (spaces.data ?? []).filter((space) => can(space, "share"));
  const [scope, setScopeState] = useState<Scope>(pinnedSpaceId ? "space" : isAdmin ? "workspace" : "space");
  const setScope = (next: Scope) => { if (!pinnedSpaceId) setScopeState(next); };
  const [spaceId, setSpaceId] = useState("");
  const chosenSpace = pinnedSpaceId
    ? spaces.data?.find((space) => space.id === pinnedSpaceId) ?? (pinnedSpaceId ? { id: pinnedSpaceId, name: "" } as (typeof manageable)[number] : undefined)
    : manageable.find((space) => space.id === spaceId) ?? manageable[0];
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState<Status>("all");
  const [sort, setSort] = useState<{ key: SortKey; dir: 1 | -1 }>({ key: "name", dir: 1 });
  const [drawer, setDrawer] = useState<{ field?: Field } | null>(null);
  const [archiving, setArchiving] = useState<{ field: Field; impact?: Impact; error?: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState("");

  const workspaceAllowed = isAdmin;
  const spaceScopeId = scope === "space" ? chosenSpace?.id ?? null : null;
  const path = scope === "workspace" ? "/api/v1/custom-fields" : chosenSpace ? `/api/v1/spaces/${chosenSpace.id}/custom-fields` : "";
  const enabled = scope === "workspace" ? workspaceAllowed : !!path;
  const fields = useQuery({
    queryKey: ["custom-field-registry", scope, spaceScopeId ?? "workspace"], enabled, retry: false,
    queryFn: async () => (await api<{ items: Field[] }>(path)).items,
  });

  const rows = useMemo(() => {
    const needle = search.trim().toLowerCase();
    const list = (fields.data ?? []).filter((field) => {
      if (status === "active" && field.archived_at) return false;
      if (status === "archived" && !field.archived_at) return false;
      return !needle || field.name.toLowerCase().includes(needle);
    });
    const value = (field: Field) => sort.key === "name" ? field.name.toLowerCase()
      : sort.key === "type" ? field.type : field.archived_at ? "1" : "0";
    return [...list].sort((a, b) => (value(a) < value(b) ? -1 : value(a) > value(b) ? 1 : 0) * sort.dir);
  }, [fields.data, search, status, sort]);

  const scopeInfo: FieldScope = scope === "workspace"
    ? { spaceId: null, ownerName: t("spaces.customFields.workspace") }
    : { spaceId: chosenSpace?.id ?? null, ownerName: chosenSpace?.name ?? "" };
  const ownerOf = (field: Field) => field.space_id ? (spaces.data ?? []).find((space) => space.id === field.space_id)?.name ?? "" : t("spaces.customFields.workspace");

  function toggleSort(key: SortKey) {
    setSort((current) => current.key === key ? { key, dir: current.dir === 1 ? -1 : 1 } : { key, dir: 1 });
  }
  const ariaSort = (key: SortKey) => sort.key === key ? (sort.dir === 1 ? "ascending" : "descending") : "none";
  const refresh = () => client.invalidateQueries({ queryKey: ["custom-field-registry"] });

  async function startArchive(field: Field) {
    setActionError("");
    setArchiving({ field });
    try {
      const impact = await api<Impact>(`${path}/${field.id}/impact`);
      setArchiving({ field, impact });
    } catch (err) {
      setArchiving({ field, error: (err as Error).message });
    }
  }

  async function archive() {
    if (!archiving) return;
    setBusy(true);
    try {
      await api(`${path}/${archiving.field.id}`, { method: "DELETE" });
      setArchiving(null);
      await refresh();
    } catch (err) {
      setArchiving({ ...archiving, error: (err as Error).message });
    } finally {
      setBusy(false);
    }
  }

  async function restore(field: Field) {
    setActionError("");
    try {
      await api(`${path}/${field.id}/restore`, { method: "POST" });
      await refresh();
    } catch (err) {
      setActionError(err instanceof ApiError && err.status === 422 ? err.message : (err as Error).message);
    }
  }

  const denied = (fields.error instanceof ApiError && (fields.error.status === 403 || fields.error.status === 404));
  const noSpace = scope === "space" && manageable.length === 0 && spaces.isSuccess;

  return <section className="space-y-5">
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="space-y-1">
        <h2 className="text-lg font-semibold">{t("settings.customFields.title")}</h2>
        <p className="text-sm text-muted-foreground">{t("settings.customFields.description")}</p>
      </div>
      <Button disabled={!enabled || !fields.isSuccess} onClick={() => setDrawer({})}>{t("settings.customFields.create")}</Button>
    </div>

    <div className="flex flex-wrap items-center gap-3">
      {!pinnedSpaceId && <SegmentedControl aria-label={t("settings.customFields.scopeLabel")} value={scope}
        items={SCOPES.map((value) => ({ value, label: t(`settings.customFields.scope.${value}`) }))}
        onChange={(value) => { setScope(value as Scope); setArchiving(null); setDrawer(null); setActionError(""); }} />}
      {scope === "space" && <label className="flex items-center gap-2 text-sm">
        <span>{t("settings.customFields.spaceLabel")}</span>
        <select className={selectClass} value={chosenSpace?.id ?? ""} disabled={manageable.length === 0}
          onChange={(event) => setSpaceId(event.target.value)}>
          {manageable.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
        </select>
      </label>}
    </div>

    <AdminToolbar searchLabel={t("settings.customFields.search")} search={search} onSearch={setSearch}>
      <select aria-label={t("settings.customFields.statusLabel")} className={selectClass} value={status}
        onChange={(event) => setStatus(event.target.value as Status)}>
        {STATUSES.map((value) => <option key={value} value={value}>{t(`settings.customFields.status.${value}`)}</option>)}
      </select>
    </AdminToolbar>

    {!workspaceAllowed && scope === "workspace" && <p role="status" className="rounded-md border p-4 text-sm text-muted-foreground">
      {t("settings.customFields.deniedWorkspace")}</p>}
    {noSpace && <p role="status" className="rounded-md border p-4 text-sm text-muted-foreground">{t("settings.customFields.noSpaces")}</p>}
    {fields.isPending && enabled && <p role="status" className="text-sm text-muted-foreground">{t("settings.customFields.loading")}</p>}
    {fields.isError && !denied && <div className="flex items-center justify-between gap-2">
      <FormError message={t("settings.customFields.loadError")} />
      <Button variant="outline" size="sm" onClick={() => void fields.refetch()}>{t("spaces.page.retry")}</Button>
    </div>}
    {denied && <p role="status" className="rounded-md border p-4 text-sm text-muted-foreground">{t("settings.customFields.denied")}</p>}
    {actionError && <FormError message={actionError} />}

    {fields.isSuccess && fields.data.length === 0 && <AdminEmptyState title={t("settings.customFields.empty")} description={t("settings.customFields.emptyHint")} />}
    {fields.isSuccess && fields.data.length > 0 && rows.length === 0 && <p role="status" className="text-sm text-muted-foreground">{t("settings.customFields.noMatch")}</p>}

    {rows.length > 0 && <div className="overflow-x-auto rounded-lg border">
      <table className="w-full text-left text-sm">
        <thead className="border-b bg-muted/40">
          <tr>
            {SORT_COLUMNS.map(([key, label]) => (
              <th key={key} scope="col" aria-sort={ariaSort(key)} className="px-3 py-2 font-medium">
                <button type="button" className="inline-flex items-center gap-1" onClick={() => toggleSort(key)}>
                  {t(label)}{sort.key === key && (sort.dir === 1 ? <ArrowUp className="size-3.5" aria-hidden="true" /> : <ArrowDown className="size-3.5" aria-hidden="true" />)}
                </button></th>
            ))}
            <th scope="col" className="px-3 py-2 font-medium">{t("settings.customFields.col.owner")}</th>
            <th scope="col" aria-sort={ariaSort("status")} className="px-3 py-2 font-medium">
              <button type="button" className="inline-flex items-center gap-1" onClick={() => toggleSort("status")}>
                {t("settings.customFields.col.status")}{sort.key === "status" && (sort.dir === 1 ? <ArrowUp className="size-3.5" aria-hidden="true" /> : <ArrowDown className="size-3.5" aria-hidden="true" />)}
              </button></th>
            <th scope="col" className="px-3 py-2"><span className="sr-only">{t("settings.customFields.col.actions")}</span></th>
          </tr>
        </thead>
        <tbody className="divide-y">
          {rows.map((field) => <tr key={field.id} className="align-middle">
            <th scope="row" className="px-3 py-2 text-start font-medium break-words">{field.name}</th>
            <td className="px-3 py-2">{t(`spaces.customFields.type.${field.type}`)}</td>
            <td className="px-3 py-2 text-muted-foreground">{ownerOf(field)}</td>
            <td className="px-3 py-2"><AdminStatus tone={field.archived_at ? "neutral" : "success"}
              label={t(field.archived_at ? "settings.customFields.status.archived" : "settings.customFields.status.active")} /></td>
            <td className="px-3 py-2 text-end">
              <DropdownMenu>
                <DropdownMenuTrigger render={<Button size="sm" variant="outline" aria-label={t("settings.customFields.actionsFor", { name: field.name })} />}>
                  <MoreHorizontal /></DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  {!field.archived_at && <DropdownMenuItem onClick={() => setDrawer({ field })}>{t("settings.customFields.edit")}</DropdownMenuItem>}
                  {!field.archived_at && <DropdownMenuItem onClick={() => void startArchive(field)}>{t("settings.customFields.archive")}</DropdownMenuItem>}
                  {field.archived_at && <DropdownMenuItem onClick={() => void restore(field)}>{t("settings.customFields.restore")}</DropdownMenuItem>}
                </DropdownMenuContent>
              </DropdownMenu>
            </td>
          </tr>)}
        </tbody>
      </table>
    </div>}

    {drawer && <CustomFieldDrawer key={drawer.field?.id ?? "new"} open scope={scopeInfo} field={drawer.field}
      onOpenChange={(open) => { if (!open) setDrawer(null); }} onSaved={() => void refresh()} />}

    <Dialog open={!!archiving} onOpenChange={(open) => { if (!open) setArchiving(null); }}>
      <DialogContent className="sm:max-w-lg">
        <DialogTitle>{t("spaces.customFields.archiveTitle", { name: archiving?.field.name ?? "" })}</DialogTitle>
        <DialogDescription>{t("spaces.customFields.archiveHint")}</DialogDescription>
        {!archiving?.impact && !archiving?.error && <p role="status" className="text-sm text-muted-foreground">{t("spaces.customFields.usage.loading")}</p>}
        {archiving?.impact && <FieldImpactList impact={archiving.impact} />}
        {archiving?.error && <FormError message={archiving.error} />}
        <DialogFooter>
          <Button variant="ghost" disabled={busy} onClick={() => setArchiving(null)}>{t("spaces.customFields.cancel")}</Button>
          <Button variant="outline" disabled={busy || !archiving?.impact || archiving.impact.formulas.length > 0} onClick={() => void archive()}>
            {t("spaces.customFields.archiveConfirm")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </section>;
}
