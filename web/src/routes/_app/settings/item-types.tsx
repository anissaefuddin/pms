import { useState, type FormEvent } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { LAYOUT_BLOCKS, itemTypeKeys, useItemTypes, type ItemTypeDef } from "@/lib/item-types";
import { useProfile } from "@/lib/profile";
import { useSpaces } from "@/lib/space-queries";
import { can } from "@/lib/spaces";
import { FormError } from "@/components/auth/form-error";
import { Layers } from "lucide-react";
import { AdminStatus } from "@/components/admin/admin-ui";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { CustomFieldsDialog } from "@/components/custom-fields-dialog";
import { TYPE_ICON_NAMES } from "@/components/type-icon";

export const Route = createFileRoute("/_app/settings/item-types")({ component: ItemTypesRoute });

function ItemTypesRoute() {
  return <ItemTypesPanel />;
}

const kinds = ["task", "project", "folder"] as const;
const levels = ["high", "normal", "low"] as const;
const selectClass = "block w-full rounded border bg-background px-2 py-1 text-sm";
type Section = { title: string; fields: string[] };

type Column = "key" | "base" | "scope" | "status" | "usage";
type SortKey = "name" | Column;
type GroupBy = "kind" | "scope" | "base";
const columnKey = "pms.itemTypes.columns";
const allColumns: Column[] = ["key", "base", "scope", "status", "usage"];

/** The registry: a sortable table with optional grouping, visible columns (kept on this device), and bulk activation.
 *  Without spaceId it is the Workspace settings page (account admins). With spaceId it is that Space's Item Types:
 *  Space editors manage the types of that Space, and the Workspace types appear read-only. */
export function ItemTypesPanel({ spaceId }: { spaceId?: string }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const admin = useProfile()?.user_type === "admin";
  const types = useItemTypes();
  const spaces = useSpaces();
  const inSpace = spaces.data?.find((space) => space.id === spaceId);
  const manage = spaceId ? !!inSpace && can(inSpace, "edit") : admin;
  const canEdit = (type: ItemTypeDef) => spaceId ? manage && type.space_id === spaceId : admin;
  // Global counts are for account administrators only (CIT-D06); the server enforces it too.
  const usage = useQuery({ queryKey: ["item-types", "usage"], enabled: admin, retry: false,
    queryFn: () => api<{ items: Record<string, number> }>("/api/v1/item-types/usage") });
  const [editing, setEditing] = useState<ItemTypeDef | "new">();
  const [duplicating, setDuplicating] = useState<ItemTypeDef>();
  const [workspaceFields, setWorkspaceFields] = useState(false);
  const [search, setSearch] = useState("");
  const [scopeFilter, setScopeFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");
  const [sortKey, setSortKey] = useState<SortKey>("name");
  const [sortDesc, setSortDesc] = useState(false);
  const [groupBy, setGroupBy] = useState<GroupBy>("kind");
  const [columns, setColumns] = useState<Column[]>(() => {
    try { const raw = JSON.parse(window.localStorage.getItem(columnKey) ?? "null"); return Array.isArray(raw) ? raw.filter((c) => allColumns.includes(c)) : allColumns; } catch { return allColumns; }
  });
  const [selected, setSelected] = useState<string[]>([]);
  const [bulkBusy, setBulkBusy] = useState(false);
  if (!types.data) return null;
  const spaceName = (id?: string | null) => (id ? spaces.data?.find((space) => space.id === id)?.name : undefined) ?? t("settings.itemTypes.scopeWorkspace");
  const scopeOf = (type: ItemTypeDef) => spaceName(type.space_id);
  const baseOf = (type: ItemTypeDef) => t(`items.${type.base}`);
  const query = search.trim().toLowerCase();
  const matches = (type: ItemTypeDef) => (!query || type.name.toLowerCase().includes(query) || type.key.toLowerCase().includes(query))
    && (!spaceId || !type.space_id || type.space_id === spaceId)
    && (scopeFilter === "all" || (scopeFilter === "workspace" ? !type.space_id : type.space_id === scopeFilter))
    && (statusFilter === "all" || (statusFilter === "active") === type.active);
  const value = (type: ItemTypeDef, key: SortKey): string | number => {
    switch (key) {
      case "key": return type.key;
      case "base": return baseOf(type);
      case "scope": return scopeOf(type);
      case "status": return type.active ? 0 : 1;
      case "usage": return usage.data?.items[type.id] ?? 0;
      default: return type.name;
    }
  };
  const sorted = types.data.filter(matches).sort((a, b) => {
    const x = value(a, sortKey), y = value(b, sortKey);
    const order = typeof x === "number" && typeof y === "number" ? x - y : String(x).localeCompare(String(y));
    return sortDesc ? -order : order;
  });
  const groupLabel = (type: ItemTypeDef) => groupBy === "scope" ? scopeOf(type) : groupBy === "base" ? baseOf(type)
    : type.is_builtin ? t("settings.itemTypes.groupBuiltIn") : t("settings.itemTypes.groupCustom");
  const groups = [...new Map(sorted.map((type) => [groupLabel(type), [] as ItemTypeDef[]])).keys()]
    .map((label) => ({ label, rows: sorted.filter((type) => groupLabel(type) === label) }));
  const toggleColumn = (column: Column) => {
    const next = columns.includes(column) ? columns.filter((c) => c !== column) : [...columns, column];
    setColumns(next);
    try { window.localStorage.setItem(columnKey, JSON.stringify(next)); } catch { /* the setting is optional */ }
  };
  const sortBy = (key: SortKey) => { if (sortKey === key) setSortDesc(!sortDesc); else { setSortKey(key); setSortDesc(false); } };
  const shownColumns = (["key", "base", "scope", "status", "usage"] as Column[]).filter((c) => columns.includes(c) && (c !== "usage" || admin));
  const columnLabel: Record<Column, string> = { key: t("settings.itemTypes.keyColumn"), base: t("settings.itemTypes.basedOnLabel"), scope: t("settings.itemTypes.scopeLabel"), status: t("settings.itemTypes.statusLabel"), usage: t("settings.itemTypes.usage") };
  // Deactivating keeps every existing item; the impact is the live items of the selected custom types, shown before the change.
  const impact = (types.data ?? []).filter((x) => selected.includes(x.id) && !x.is_builtin && x.active).reduce((sum, x) => sum + (usage.data?.items[x.id] ?? 0), 0);
  const toggleSelected = (id: string) => setSelected((rows) => rows.includes(id) ? rows.filter((x) => x !== id) : [...rows, id]);
  // Bulk activation applies to custom types only; each change needs the type's current version.
  async function setActiveFor(active: boolean) {
    setBulkBusy(true);
    try {
      for (const type of (types.data ?? []).filter((x) => selected.includes(x.id) && !x.is_builtin && x.active !== active)) {
        await api(`/api/v1/item-types/${type.id}`, { method: "PATCH", headers: { "If-Match": `"${type.version}"` }, body: JSON.stringify({ active }) });
      }
      setSelected([]);
      await client.invalidateQueries({ queryKey: itemTypeKeys.all });
    } finally { setBulkBusy(false); }
  }
  const row = (type: ItemTypeDef) => <tr key={type.id} className="border-t">
    {manage && <td className="p-2"><input type="checkbox" aria-label={`${t("settings.itemTypes.select")}: ${type.name}`} disabled={type.is_builtin || !canEdit(type)} checked={selected.includes(type.id)} onChange={() => toggleSelected(type.id)} /></td>}
    <td className="p-2"><div className="flex items-center gap-2"><span aria-hidden="true" className="grid size-7 shrink-0 place-items-center rounded-md bg-muted text-muted-foreground"><Layers className="size-4" /></span><span className="font-medium">{type.name}</span></div></td>
    {shownColumns.includes("key") && <td className="p-2 font-mono text-xs">{type.key}</td>}
    {shownColumns.includes("base") && <td className="p-2">{baseOf(type)}</td>}
    {shownColumns.includes("scope") && <td className="p-2">{scopeOf(type)}</td>}
    {shownColumns.includes("status") && <td className="p-2"><AdminStatus tone={type.active ? "success" : "neutral"} label={type.active ? t("settings.itemTypes.statusActive") : t("settings.itemTypes.inactive")} /></td>}
    {shownColumns.includes("usage") && <td className="p-2 tabular-nums">{usage.data?.items[type.id] ?? 0}</td>}
    <td className="p-2"><div className="flex flex-wrap gap-2">
      {canEdit(type) && <Button variant="outline" size="sm" aria-label={`${t("settings.itemTypes.edit")}: ${type.name}`} onClick={() => setEditing(type)}>{t("settings.itemTypes.edit")}</Button>}
      {canEdit(type) && <Button variant="outline" size="sm" aria-label={`${t("settings.itemTypes.duplicate")}: ${type.name}`} onClick={() => setDuplicating(type)}>{t("settings.itemTypes.duplicate")}</Button>}
    </div></td>
  </tr>;
  const nameSort: SortKey = "name";
  const sortButton = (key: SortKey, label: string) => <button type="button" className="font-medium" onClick={() => sortBy(key)}
    aria-label={`${t("settings.itemTypes.sortBy")}: ${label}`}>{label}{sortKey === key ? (sortDesc ? " ↓" : " ↑") : ""}</button>;
  return <div className="space-y-6">
    <header className="flex flex-wrap items-start justify-between gap-3">
      <p className="max-w-2xl text-sm text-muted-foreground">{manage ? t(spaceId ? "settings.itemTypes.spaceDescription" : "settings.itemTypes.description") : t("settings.itemTypes.readOnly")}</p>
      {manage && <div className="flex flex-wrap gap-2">
        {!spaceId && <Button variant="outline" onClick={() => setWorkspaceFields(true)}>{t("settings.itemTypes.manageWorkspaceFields")}</Button>}
        <Button onClick={() => setEditing("new")}>{t("settings.itemTypes.create")}</Button>
      </div>}
    </header>
    <div className="flex flex-wrap items-center gap-2">
      <Input type="search" aria-label={t("settings.itemTypes.search")} placeholder={t("settings.itemTypes.search")} value={search} onChange={(event) => setSearch(event.target.value)} className="max-w-xs" />
      {!spaceId && <select aria-label={t("settings.itemTypes.filterScope")} className={selectClass} value={scopeFilter} onChange={(event) => setScopeFilter(event.target.value)}>
        <option value="all">{t("settings.itemTypes.filterAll")}</option>
        <option value="workspace">{t("settings.itemTypes.scopeWorkspace")}</option>
        {(spaces.data ?? []).map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
      </select>}
      <select aria-label={t("settings.itemTypes.filterStatus")} className={selectClass} value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
        <option value="all">{t("settings.itemTypes.filterAll")}</option>
        <option value="active">{t("settings.itemTypes.filterActive")}</option>
        <option value="inactive">{t("settings.itemTypes.filterInactive")}</option>
      </select>
      <select aria-label={t("settings.itemTypes.groupBy")} className={selectClass} value={groupBy} onChange={(event) => setGroupBy(event.target.value as GroupBy)}>
        <option value="kind">{t("settings.itemTypes.groupByKind")}</option>
        <option value="scope">{t("settings.itemTypes.groupByScope")}</option>
        <option value="base">{t("settings.itemTypes.groupByBase")}</option>
      </select>
    </div>
    <fieldset className="flex flex-wrap items-center gap-3 text-sm"><legend className="sr-only">{t("settings.itemTypes.columns")}</legend>
      <span className="text-muted-foreground">{t("settings.itemTypes.columns")}:</span>
      {allColumns.filter((c) => c !== "usage" || admin).map((column) => <label key={column} className="flex items-center gap-1">
        <input type="checkbox" checked={columns.includes(column)} onChange={() => toggleColumn(column)} />{columnLabel[column]}</label>)}
    </fieldset>
    {manage && selected.length > 0 && <div className="flex flex-wrap items-center gap-2 rounded-lg border bg-muted/40 p-3 text-sm" role="region" aria-label={t("settings.itemTypes.bulk")}>
      <span>{t("settings.itemTypes.selectedCount", { count: selected.length })}</span>
      {impact > 0 && <span className="text-muted-foreground">{t("settings.itemTypes.deactivateImpact", { count: impact })}</span>}
      <Button variant="outline" size="sm" disabled={bulkBusy} onClick={() => void setActiveFor(true)}>{t("settings.itemTypes.activate")}</Button>
      <Button variant="outline" size="sm" disabled={bulkBusy} onClick={() => void setActiveFor(false)}>{t("settings.itemTypes.deactivate")}</Button>
      <Button variant="ghost" size="sm" onClick={() => setSelected([])}>{t("settings.itemTypes.clearSelection")}</Button>
    </div>}
    {sorted.length === 0 && <p role="status" className="text-sm text-muted-foreground">{t("settings.itemTypes.noMatch")}</p>}
    {groups.map((group) => <section key={group.label} aria-label={group.label} className="space-y-2">
      <h2 className="text-sm font-medium">{group.label}</h2>
      <div className="overflow-x-auto rounded-lg border"><table className="w-full text-left text-sm">
        <thead className="bg-muted/40"><tr>
          {manage && <th scope="col" className="p-2"><span className="sr-only">{t("settings.itemTypes.select")}</span></th>}
          <th scope="col" className="p-2" aria-sort={sortKey === "name" ? (sortDesc ? "descending" : "ascending") : "none"}>{sortButton(nameSort, t("settings.itemTypes.nameColumn"))}</th>
          {shownColumns.map((column) => <th key={column} scope="col" className="p-2" aria-sort={sortKey === column ? (sortDesc ? "descending" : "ascending") : "none"}>{sortButton(column, columnLabel[column])}</th>)}
          <th scope="col" className="p-2"><span className="sr-only">{t("settings.itemTypes.actions")}</span></th>
        </tr></thead>
        <tbody>{group.rows.map(row)}</tbody>
      </table></div>
    </section>)}
    {admin && !spaceId && <CustomFieldsDialog open={workspaceFields} onOpenChange={setWorkspaceFields} spaceId={null} />}
    {editing && <TypeDialog type={editing === "new" ? undefined : editing} all={types.data} spaces={spaceId ? [] : (spaces.data ?? []).filter((space) => can(space, "edit"))}
      fixedScope={spaceId} spaceName={spaceName} onClose={() => setEditing(undefined)} />}
    {duplicating && <TypeDialog template={duplicating} all={types.data} spaces={spaceId ? [] : (spaces.data ?? []).filter((space) => can(space, "edit"))}
      fixedScope={spaceId} spaceName={spaceName} onClose={() => setDuplicating(undefined)} />}
  </div>;
}

/** A template copies a type's definition into a new type: the key is new, and the fields are chosen after it is created. */
function TypeDialog({ type, template, all, spaces, fixedScope, spaceName, onClose }: { type?: ItemTypeDef; template?: ItemTypeDef; all: ItemTypeDef[]; spaces: { id: string; name: string }[]; fixedScope?: string; spaceName: (id?: string | null) => string; onClose: () => void }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const source = type ?? template;
  const [base, setBase] = useState<ItemTypeDef["base"]>(source?.base ?? "task");
  const [children, setChildren] = useState<string[]>(source?.allowed_children ?? []);
  const [active, setActive] = useState(type?.active ?? true);
  // CIT-D05: Quick Access is off for existing types and shows the type in the Create menu only.
  const [quick, setQuick] = useState(type?.quick_access ?? false);
  const [scope, setScope] = useState(fixedScope ?? template?.space_id ?? "");
  // Default workflow applies to projects and folders only: a task takes its workflow from its parent.
  const [workflowId, setWorkflowId] = useState(source?.default_workflow_id ?? "");
  const workflows = useQuery({ queryKey: ["workflows"], queryFn: () => api<{ items: { id: string; name: string }[] }>("/api/v1/workflows") });
  // Custom fields the type carries: Workspace types take Workspace fields only; Space types also see their Space's fields.
  const workspaceFields = useQuery({ queryKey: ["custom-fields", "workspace"], enabled: !!source, queryFn: () => api<{ items: { id: string; name: string; archived_at?: string | null }[] }>("/api/v1/custom-fields") });
  const spaceFields = useQuery({ queryKey: ["custom-fields", source?.space_id ?? "none"], enabled: !!source?.space_id,
    queryFn: () => api<{ items: { id: string; name: string; archived_at?: string | null }[] }>(`/api/v1/spaces/${source?.space_id}/custom-fields`) });
  const assigned = useQuery({ queryKey: ["item-type-fields", source?.id], enabled: !!source,
    queryFn: () => api<{ items: { field_id: string }[] }>(`/api/v1/item-types/${source?.id}/fields`) });
  const [picked, setPicked] = useState<string[]>();
  const assignedIds = assigned.data?.items.map((f) => f.field_id) ?? [];
  const chosen = picked ?? assignedIds;
  const candidates = [...(workspaceFields.data?.items ?? []), ...(source?.space_id ? spaceFields.data?.items ?? [] : [])].filter((f) => !f.archived_at);
  const [inherit, setInherit] = useState(Array.isArray(source?.defaults?.inherit) && (source?.defaults?.inherit as string[]).includes("importance"));
  const [sections, setSections] = useState<Section[]>(((source?.layout as { sections?: Section[] } | undefined)?.sections) ?? []);
  const eligible = all.filter((other) => other.id !== type?.id && other.active && (base !== "task" || other.base === "task"));

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const text = (name: string) => String(data.get(name) ?? "").trim();
    const defaults: Record<string, unknown> = {};
    if (text("importance")) defaults.importance = text("importance");
    for (const name of ["duration_days", "effort_minutes"]) if (text(name)) defaults[name] = Number(text(name));
    if (inherit) defaults.inherit = ["importance"];
    const body: Record<string, unknown> = { name: text("name"), icon: text("icon") || undefined, allowed_children: children, defaults,
      ...(base !== "task" ? { default_workflow_id: workflowId || null } : {}),
      layout: sections.length ? { sections: sections.filter((section) => section.title.trim()).map((section) => ({ ...section, fields: section.fields.filter((block) => blockOptions.includes(block)) })) } : {} };
    if (!type) Object.assign(body, { key: text("key"), base, space_id: scope || null });
    else body.active = active;
    if (type && !type.is_builtin) body.quick_access = active && quick;
    setBusy(true); setError("");
    try {
      const saved = await api<ItemTypeDef>(type ? `/api/v1/item-types/${type.id}` : "/api/v1/item-types", { method: type ? "PATCH" : "POST", headers: type ? { "If-Match": `"${type.version}"` } : undefined, body: JSON.stringify(body) });
      if (!type && template) {
        for (const fieldId of chosen) await api(`/api/v1/item-types/${saved.id}/fields/${fieldId}`, { method: "PUT", body: "{}" });
      }
      if (type && picked) {
        for (const fieldId of chosen.filter((id) => !assignedIds.includes(id))) await api(`/api/v1/item-types/${type.id}/fields/${fieldId}`, { method: "PUT", body: "{}" });
        for (const fieldId of assignedIds.filter((id) => !chosen.includes(id))) await api(`/api/v1/item-types/${type.id}/fields/${fieldId}`, { method: "DELETE" });
        await client.invalidateQueries({ queryKey: ["item-type-fields", type.id] });
      }
      await client.invalidateQueries({ queryKey: itemTypeKeys.all });
      toast.success(t("settings.itemTypes.saved"));
      onClose();
    } catch (err) { setError(err instanceof ApiError ? err.message : (err as Error).message); } finally { setBusy(false); }
  }
  // Custom fields the type carries can be placed in a section like the built-in blocks; their block id is the field id.
  const blockOptions: string[] = [...LAYOUT_BLOCKS, ...chosen.filter((id) => candidates.some((field) => field.id === id))];
  const blockLabel = (block: string) => (LAYOUT_BLOCKS as readonly string[]).includes(block)
    ? t(`settings.itemTypes.block.${block}`) : candidates.find((field) => field.id === block)?.name ?? block;
  const moveSection = (index: number, delta: number) => setSections((rows) => {
    const next = [...rows]; const target = index + delta;
    [next[index], next[target]] = [next[target], next[index]];
    return next;
  });
  const moveBlock = (sectionIndex: number, position: number, delta: number) => {
    const fields = [...sections[sectionIndex].fields];
    const target = position + delta;
    [fields[position], fields[target]] = [fields[target], fields[position]];
    updateSection(sectionIndex, { fields });
  };
  const updateSection = (index: number, patch: Partial<Section>) => setSections((rows) => rows.map((row, i) => i === index ? { ...row, ...patch } : row));
  const toggleBlock = (index: number, block: string) => updateSection(index, {
    fields: sections[index].fields.includes(block) ? sections[index].fields.filter((x) => x !== block) : [...sections[index].fields, block] });

  return <Dialog open onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="max-h-[90vh] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>{type ? t("settings.itemTypes.editTitle", { name: type.name }) : t("settings.itemTypes.createTitle")}</DialogTitle>
        <DialogDescription>{t("settings.itemTypes.dialogHint")}</DialogDescription>
      </DialogHeader>
      <form className="space-y-3" onSubmit={submit}>
        <label className="block space-y-1 text-sm"><span>{t("settings.itemTypes.nameColumn")}</span>
          <Input name="name" defaultValue={type?.name ?? (template ? t("settings.itemTypes.copyOf", { name: template.name }) : "")} maxLength={80} required /></label>
        {!type && <>
          <label className="block space-y-1 text-sm"><span>{t("settings.itemTypes.key")}</span>
            <Input name="key" pattern="[a-z][a-z0-9_]{1,39}" maxLength={40} required /></label>
          <label className="block space-y-1 text-sm"><span>{t("settings.itemTypes.base")}</span>
            <select className={selectClass} value={base} onChange={(event) => setBase(event.target.value as ItemTypeDef["base"])}>
              {kinds.map((kind) => <option key={kind} value={kind}>{t(`items.${kind}`)}</option>)}</select></label>
          {/* CIT-D01: a Workspace type works in every Space; a Space type works only in that Space. Fixed once created. */}
          {!fixedScope && <label className="block space-y-1 text-sm"><span>{t("settings.itemTypes.scope")}</span>
            <select className={selectClass} value={scope} onChange={(event) => setScope(event.target.value)}>
              <option value="">{t("settings.itemTypes.scopeWorkspace")}</option>
              {spaces.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}</select></label>}
        </>}
        {type && <p className="text-sm text-muted-foreground">{t("settings.itemTypes.scope")}: {spaceName(type.space_id)}</p>}
        <label className="block space-y-1 text-sm"><span>{t("settings.itemTypes.icon")}</span>
          <select name="icon" className={selectClass} defaultValue={source?.icon ?? ""}>
            <option value="">{t("settings.itemTypes.iconNone")}</option>
            {[...TYPE_ICON_NAMES, ...(source?.icon && !TYPE_ICON_NAMES.includes(source.icon) ? [source.icon] : [])].map((name) => <option key={name} value={name}>{name}</option>)}
          </select></label>
        {type && !type.is_builtin && <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={active} onChange={(event) => setActive(event.target.checked)} />{t("settings.itemTypes.active")}</label>}
        {type && !type.is_builtin && <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={quick && active} disabled={!active} onChange={(event) => setQuick(event.target.checked)} />{t("settings.itemTypes.quickAccess")}</label>}
        {type && !type.is_builtin && <p className="text-xs text-muted-foreground">{t("settings.itemTypes.quickAccessHint")}</p>}
        <fieldset className="space-y-1 text-sm"><legend className="font-medium">{t("settings.itemTypes.children")}</legend>
          <p className="text-muted-foreground">{t("settings.itemTypes.childrenHint")}</p>
          {eligible.map((other) => <label key={other.id} className="flex items-center gap-2">
            <input type="checkbox" checked={children.includes(other.id)} onChange={() => setChildren((rows) => rows.includes(other.id) ? rows.filter((x) => x !== other.id) : [...rows, other.id])} />{other.name}</label>)}
        </fieldset>
        <fieldset className="space-y-1 text-sm"><legend className="font-medium">{t("settings.itemTypes.fields")}</legend>
          {!source ? <p className="text-muted-foreground">{t("settings.itemTypes.fieldsSaveFirst")}</p> : <>
            <p className="text-muted-foreground">{t("settings.itemTypes.fieldsHint")}</p>
            {candidates.map((field) => <label key={field.id} className="flex items-center gap-2">
              <input type="checkbox" checked={chosen.includes(field.id)} onChange={() => setPicked(chosen.includes(field.id) ? chosen.filter((id) => id !== field.id) : [...chosen, field.id])} />{field.name}</label>)}
          </>}
        </fieldset>
        {base !== "task" && <label className="block space-y-1 text-sm"><span>{t("settings.itemTypes.workflow")}</span>
          <select className={selectClass} value={workflowId} onChange={(event) => setWorkflowId(event.target.value)}>
            <option value="">{t("settings.itemTypes.workflowDefault")}</option>
            {workflows.data?.items.map((wf) => <option key={wf.id} value={wf.id}>{wf.name}</option>)}</select></label>}
        <fieldset className="grid gap-2 text-sm sm:grid-cols-3"><legend className="font-medium">{t("settings.itemTypes.defaults")}</legend>
          <label className="space-y-1"><span>{t("items.importance.label")}</span>
            <select name="importance" className={selectClass} defaultValue={String(source?.defaults?.importance ?? "")}>
              <option value="">—</option>{levels.map((level) => <option key={level} value={level}>{t(`items.importance.${level}`)}</option>)}</select></label>
          <label className="space-y-1"><span>{t("settings.itemTypes.duration")}</span>
            <Input name="duration_days" type="number" min={0} defaultValue={String(source?.defaults?.duration_days ?? "")} /></label>
          <label className="space-y-1"><span>{t("settings.itemTypes.effort")}</span>
            <Input name="effort_minutes" type="number" min={0} defaultValue={String(source?.defaults?.effort_minutes ?? "")} /></label>
          <label className="flex items-center gap-2 sm:col-span-3"><input type="checkbox" checked={inherit} onChange={(event) => setInherit(event.target.checked)} />{t("settings.itemTypes.inherit")}</label>
        </fieldset>
        <fieldset className="space-y-2 text-sm"><legend className="font-medium">{t("settings.itemTypes.layout")}</legend>
          <p className="text-muted-foreground">{t("settings.itemTypes.layoutHint")}</p>
          {sections.map((section, index) => <div key={index} className="space-y-2 rounded border p-2">
            <div className="flex flex-wrap items-end gap-2">
              <label className="min-w-0 flex-1 space-y-1"><span>{t("settings.itemTypes.sectionTitle", { number: index + 1 })}</span>
                <Input value={section.title} maxLength={60} onChange={(event) => updateSection(index, { title: event.target.value })} /></label>
              <Button type="button" variant="outline" size="sm" disabled={index === 0} aria-label={t("settings.itemTypes.moveSectionUp", { number: index + 1 })} onClick={() => moveSection(index, -1)}>↑</Button>
              <Button type="button" variant="outline" size="sm" disabled={index === sections.length - 1} aria-label={t("settings.itemTypes.moveSectionDown", { number: index + 1 })} onClick={() => moveSection(index, 1)}>↓</Button>
              <Button type="button" variant="ghost" size="sm" aria-label={t("settings.itemTypes.removeSection", { number: index + 1 })}
                onClick={() => setSections((rows) => rows.filter((_, i) => i !== index))}>×</Button>
            </div>
            <div className="flex flex-wrap gap-3">{blockOptions.map((block) => {
              const usedElsewhere = sections.some((other, i) => i !== index && other.fields.includes(block));
              return <label key={block} className="flex items-center gap-1"><input type="checkbox" disabled={usedElsewhere} checked={section.fields.includes(block)} onChange={() => toggleBlock(index, block)} />{blockLabel(block)}</label>;
            })}</div>
            {section.fields.length > 0 && <ol aria-label={t("settings.itemTypes.blockOrder", { number: index + 1 })} className="space-y-1">
              {section.fields.map((block, position) => <li key={block} className="flex items-center gap-2">
                <span className="flex-1">{blockLabel(block)}</span>
                <Button type="button" variant="outline" size="sm" disabled={position === 0} aria-label={t("settings.itemTypes.moveBlockUp", { name: blockLabel(block) })} onClick={() => moveBlock(index, position, -1)}>↑</Button>
                <Button type="button" variant="outline" size="sm" disabled={position === section.fields.length - 1} aria-label={t("settings.itemTypes.moveBlockDown", { name: blockLabel(block) })} onClick={() => moveBlock(index, position, 1)}>↓</Button>
              </li>)}
            </ol>}
          </div>)}
          <Button type="button" variant="outline" size="sm" onClick={() => setSections((rows) => [...rows, { title: "", fields: [] }])}>{t("settings.itemTypes.addSection")}</Button>
          {sections.length > 0 && <section aria-label={t("settings.itemTypes.preview")} className="space-y-2 rounded border border-dashed p-3">
            <p className="font-medium">{t("settings.itemTypes.preview")}</p>
            {sections.map((section, index) => <div key={index} className="rounded bg-muted/40 p-2">
              <p className="font-medium">{section.title.trim() || t("settings.itemTypes.untitled")}</p>
              <ul className="list-disc pl-5">{section.fields.map((block) => <li key={block}>{blockLabel(block)}</li>)}</ul>
            </div>)}
          </section>}
        </fieldset>
        <FormError message={error} />
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>{t("settings.itemTypes.cancel")}</Button>
          <Button type="submit" disabled={busy}>{t("settings.itemTypes.save")}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>;
}
