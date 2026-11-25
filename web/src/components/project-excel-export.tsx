import { useState, type FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import type { Item } from "@/lib/items";
import { useLocationName, useSpaces } from "@/lib/space-queries";
import { Button, buttonVariants } from "@/components/ui/button";
import { loadPresets, presetKey, removePreset, upsertPreset, type ExportPreset } from "@/lib/export-presets";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { FormError } from "@/components/auth/form-error";

// total_items and processed_items count real items; the bar is shown only once the server has counted the tree.
type ExportJob = { id: string; status: "pending" | "ready" | "failed"; error?: string; total_items?: number; processed_items?: number; scope?: string };

// What the Table View shows: search text, the preset (active or mine), a sort key, and the visible columns.
export type TableState = { search: string; preset: string; sort: string; columns: string[]; typeFilter?: string; fieldFilter?: string; viewId?: string };
const sortable = ["title", "start_date", "due_date"];

// tableFilter turns the Table View state into the export filter. The server applies the same item engine as the table,
// and refuses a filter it cannot apply rather than dropping it.
export function tableFilter(table: TableState) {
  const filter: Record<string, unknown> = {};
  if (table.preset) {
    filter.status_groups = ["active"];
    if (table.preset === "mine") filter.assignee = "me";
  }
  if (table.search.trim()) filter.search = table.search.trim();
  if (table.typeFilter) filter.types = table.typeFilter.split(",");
  if (table.fieldFilter) filter.fields = table.fieldFilter.split(",");
  if (sortable.includes(table.sort)) filter.sort = table.sort;
  const columns = table.columns.filter((column) => column === "status" || column === "start_date" || column === "due_date" || column.startsWith("field:"));
  if (columns.length) filter.columns = ["title", ...columns];
  return filter;
}

// SpaceExcelExport exports the whole Space: every root the caller can see, with the hierarchy under it (EXP-03).
export function SpaceExcelExport({ spaceId, spaceName, open, onOpenChange }: {
  spaceId: string;
  spaceName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation();
  return <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent>
      <DialogTitle>{t("excel.exportSpaceTitle")}</DialogTitle>
      <DialogDescription>{t("excel.exportSpaceDescription", { name: spaceName })}</DialogDescription>
      {open && <ExportForm spaceId={spaceId} spaceName={spaceName} namespaceView="space" onClose={() => onOpenChange(false)} />}
    </DialogContent>
  </Dialog>;
}

export function ProjectExcelExport({ item, open, onOpenChange, table }: {
  item: Item;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  table?: TableState;
}) {
  const { t } = useTranslation();
  return <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent>
      <DialogTitle>{t("excel.exportTitle")}</DialogTitle>
      <DialogDescription>{t("excel.exportDescription")}</DialogDescription>
      {open && <ExportForm item={item} table={table} namespaceView={table?.viewId ?? "item"} onClose={() => onOpenChange(false)} />}
    </DialogContent>
  </Dialog>;
}

// Column choices the export can keep. External Key and Parent Key are always written so the file imports again.
const exportColumns = ["status", "workflow", "start_date", "due_date", "duration_days", "is_milestone", "type_key", "depends_on"];


// ExportForm exports a project (item) tree, or a whole Space when spaceId is given.
function ExportForm({ item, table, spaceId, spaceName, namespaceView, onClose }: {
  item?: Item; table?: TableState; spaceId?: string; spaceName?: string; namespaceView: string; onClose: () => void;
}) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const userId = qc.getQueryData<{ id: string }>(["me"])?.id ?? "";
  const key = presetKey(userId, spaceId ?? item?.locations[0]?.space_id ?? "", namespaceView);
  const [presets, setPresets] = useState<ExportPreset[]>(() => loadPresets(key));
  const [presetName, setPresetName] = useState("");
  const [selectedPreset, setSelectedPreset] = useState("");
  const [columns, setColumns] = useState<string[]>(exportColumns);
  const { data: spaces } = useSpaces();
  const locationName = useLocationName();
  const choices: [string, string | undefined][] = spaceId
    ? [[spaceId, spaceName]]
    : [...new Map((item?.locations ?? []).map((location) => [location.space_id, locationName(location)])).entries()];
  const [selected, setSelected] = useState("");
  // Scope: the whole tree, or only tasks in one status group (folders and projects that hold a match stay).
  const [scope, setScope] = useState("");
  const [job, setJob] = useState<ExportJob>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const target = choices.some(([id]) => id === selected) ? selected : choices[0]?.[0] ?? "";
  const path = job ? `/api/v1/excel-exports/${job.id}` : "";
  const status = useQuery({
    queryKey: ["excel-export", job?.id],
    queryFn: ({ signal }) => api<ExportJob>(path, { signal }),
    enabled: job?.status === "pending",
    refetchInterval: ({ state }) => state.status !== "error" && (!state.data || state.data.status === "pending") ? 1000 : false,
    retry: false,
  });
  const current = status.data ?? job;

  // The filter sent with the export: the Table View snapshot (when opened from a table) or the scope, plus the chosen columns.
  function buildFilter() {
    const base: Record<string, unknown> = table ? tableFilter(table) : scope ? { status_groups: [scope] } : {};
    const chosen = columns.length === exportColumns.length ? [] : columns;
    if (chosen.length) base.columns = ["title", ...chosen];
    return Object.keys(base).length ? base : undefined;
  }

  function applyPreset(name: string) {
    setSelectedPreset(name);
    const preset = presets.find((entry) => entry.name === name);
    if (!preset) return;
    setPresetName(preset.name);
    setScope(preset.scope);
    setColumns(preset.columns);
  }

  // Save creates a preset, or updates the one with the same name.
  function saveCurrentPreset() {
    const name = presetName.trim();
    if (!name) return;
    setPresets(upsertPreset(key, { name, scope, columns }));
    setSelectedPreset(name);
  }

  function deleteSelectedPreset() {
    if (!selectedPreset) return;
    setPresets(removePreset(key, selectedPreset));
    setSelectedPreset("");
    setPresetName("");
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!target || busy || current?.status === "pending") return;
    setJob(undefined);
    setBusy(true);
    setError("");
    try {
      const body = spaceId ? { filter: buildFilter() } : { space_id: target, filter: buildFilter() };
      const path = spaceId ? `/api/v1/spaces/${spaceId}/excel-exports` : `/api/v1/items/${item?.id}/excel-exports`;
      setJob(await api<ExportJob>(path, { method: "POST", body: JSON.stringify(body) }));
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return <form onSubmit={submit} className="space-y-4">
    {!spaceId && <label className="block space-y-1">
      <span>{t("items.space")}</span>
      <select value={target} onChange={(event) => setSelected(event.target.value)}
        className="block w-full rounded border bg-background p-2" disabled={busy || current?.status === "pending" || current?.status === "ready"}>
        {choices.map(([id, name]) => <option key={id} value={id}>{name || spaces?.find((space) => space.id === id)?.name || id}</option>)}
      </select>
    </label>}
    {current?.status === "pending" && !status.error && <p role="status">{t("excel.exportPending")}</p>}
    {current?.status === "pending" && !!current.total_items && <div className="space-y-1">
      <progress className="w-full" max={current.total_items} value={current.processed_items ?? 0} aria-label={t("excel.exportProgress")} />
      <p className="text-sm text-muted-foreground">{t("excel.exportCount", { done: current.processed_items ?? 0, total: current.total_items })}</p>
    </div>}
    {current?.status === "ready" && <p role="status">{t("excel.exportReady")}</p>}
    <FormError message={error || status.error?.message || (current?.status === "failed" ? current.error || t("excel.exportFailed") : "")} />
    {!table && <fieldset className="space-y-2">
      <legend className="text-sm font-medium">{t("excel.columns")}</legend>
      <div className="grid grid-cols-2 gap-1 text-sm">
        {exportColumns.map((column) => <label key={column} className="flex items-center gap-2">
          <input type="checkbox" checked={columns.includes(column)} disabled={busy}
            onChange={(event) => setColumns(event.target.checked ? [...columns, column] : columns.filter((entry) => entry !== column))} />
          {t(`excel.exportColumn.${column}`)}
        </label>)}
      </div>
    </fieldset>}
    <div className="flex flex-wrap items-end gap-2 text-sm">
      <label className="space-y-1"><span>{t("excel.preset")}</span>
        <select aria-label={t("excel.preset")} className="block rounded border bg-background p-2" value={selectedPreset} onChange={(event) => applyPreset(event.target.value)}>
          <option value="">{t("excel.presetNone")}</option>
          {presets.map((preset) => <option key={preset.name} value={preset.name}>{preset.name}</option>)}
        </select></label>
      <label className="space-y-1"><span>{t("excel.presetName")}</span>
        <input className="block rounded border bg-background p-2" value={presetName} onChange={(event) => setPresetName(event.target.value)} maxLength={60} /></label>
      <Button type="button" variant="outline" disabled={!presetName.trim()} onClick={saveCurrentPreset}>{t("excel.savePreset")}</Button>
      <Button type="button" variant="outline" disabled={!selectedPreset} onClick={deleteSelectedPreset}>{t("excel.deletePreset")}</Button>
    </div>
    <label className="block space-y-1">
      <span>{t("excel.scope")}</span>
      {!table && <select value={scope} onChange={(event) => setScope(event.target.value)} aria-label={t("excel.scope")}
        className="block w-full rounded border bg-background p-2" disabled={busy || current?.status === "pending" || current?.status === "ready"}>
        <option value="">{t("excel.scopeAll")}</option>
        <option value="active">{t("excel.scopeActive")}</option>
        <option value="completed">{t("excel.scopeCompleted")}</option>
      </select>}
    </label>
    <DialogFooter>
      <Button variant="outline" onClick={onClose}>{t("excel.close")}</Button>
      {current?.status === "ready"
        ? <a className={buttonVariants()} href={path + "/file"} download>{t("excel.download")}</a>
        : status.error
          ? <Button type="button" disabled={status.isFetching} onClick={() => void status.refetch()}>{t("items.retry")}</Button>
          : <Button type="submit" disabled={!target || busy || current?.status === "pending"}>{t("excel.startExport")}</Button>}
    </DialogFooter>
  </form>;
}
