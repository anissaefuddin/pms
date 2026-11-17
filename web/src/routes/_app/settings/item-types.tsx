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

export const Route = createFileRoute("/_app/settings/item-types")({ component: ItemTypesRoute });

const kinds = ["task", "project", "folder"] as const;
const levels = ["high", "normal", "low"] as const;
const selectClass = "block w-full rounded border bg-background px-2 py-1 text-sm";
type Section = { title: string; fields: string[] };

function ItemTypesRoute() {
  const { t } = useTranslation();
  const admin = useProfile()?.user_type === "admin";
  const types = useItemTypes();
  const spaces = useSpaces();
  const [editing, setEditing] = useState<ItemTypeDef | "new">();
  const [workspaceFields, setWorkspaceFields] = useState(false);
  if (!types.data) return null;
  const spaceName = (id?: string | null) => (id ? spaces.data?.find((space) => space.id === id)?.name : undefined) ?? t("settings.itemTypes.scopeWorkspace");
  const builtIn = types.data.filter((type) => type.is_builtin);
  const custom = types.data.filter((type) => !type.is_builtin);
  // Each row: kind icon, name, kind and key as secondary text, and an Active/Inactive status.
  const list = (rows: ItemTypeDef[]) => <ul className="divide-y rounded-lg border">
    {rows.map((type) => <li key={type.id} className="flex flex-wrap items-center gap-3 p-3">
      <span aria-hidden="true" className="grid size-8 shrink-0 place-items-center rounded-md bg-muted text-muted-foreground"><Layers className="size-4" /></span>
      <div className="min-w-0 flex-1">
        <p className="font-medium">{type.name}</p>
        <p className="text-sm text-muted-foreground">{t("settings.itemTypes.basedOn", { base: t(`items.${type.base}`) })} · <span className="font-mono text-xs">{type.key}</span> · {spaceName(type.space_id)}</p>
      </div>
      <AdminStatus tone={type.active ? "success" : "neutral"} label={type.active ? t("settings.itemTypes.statusActive") : t("settings.itemTypes.inactive")} />
      {admin && <Button variant="outline" size="sm" aria-label={`${t("settings.itemTypes.edit")}: ${type.name}`} onClick={() => setEditing(type)}>{t("settings.itemTypes.edit")}</Button>}
    </li>)}
  </ul>;
  return <div className="space-y-6">
    <header className="flex flex-wrap items-start justify-between gap-3">
      <p className="max-w-2xl text-sm text-muted-foreground">{admin ? t("settings.itemTypes.description") : t("settings.itemTypes.readOnly")}</p>
      {admin && <div className="flex flex-wrap gap-2">
        <Button variant="outline" onClick={() => setWorkspaceFields(true)}>{t("settings.itemTypes.manageWorkspaceFields")}</Button>
        <Button onClick={() => setEditing("new")}>{t("settings.itemTypes.create")}</Button>
      </div>}
    </header>
    <section aria-labelledby="item-types-builtin" className="space-y-2">
      <h2 id="item-types-builtin" className="text-sm font-medium">{t("settings.itemTypes.groupBuiltIn")}</h2>
      {list(builtIn)}
    </section>
    <section aria-labelledby="item-types-custom" className="space-y-2">
      <h2 id="item-types-custom" className="text-sm font-medium">{t("settings.itemTypes.groupCustom")}</h2>
      {custom.length ? list(custom) : <p className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">{t("settings.itemTypes.create")}</p>}
    </section>
    {admin && <CustomFieldsDialog open={workspaceFields} onOpenChange={setWorkspaceFields} spaceId={null} />}
    {editing && <TypeDialog type={editing === "new" ? undefined : editing} all={types.data} spaces={(spaces.data ?? []).filter((space) => can(space, "edit"))}
      spaceName={spaceName} onClose={() => setEditing(undefined)} />}
  </div>;
}

function TypeDialog({ type, all, spaces, spaceName, onClose }: { type?: ItemTypeDef; all: ItemTypeDef[]; spaces: { id: string; name: string }[]; spaceName: (id?: string | null) => string; onClose: () => void }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [base, setBase] = useState<ItemTypeDef["base"]>(type?.base ?? "task");
  const [children, setChildren] = useState<string[]>(type?.allowed_children ?? []);
  const [active, setActive] = useState(type?.active ?? true);
  const [scope, setScope] = useState("");
  // Default workflow applies to projects and folders only: a task takes its workflow from its parent.
  const [workflowId, setWorkflowId] = useState(type?.default_workflow_id ?? "");
  const workflows = useQuery({ queryKey: ["workflows"], queryFn: () => api<{ items: { id: string; name: string }[] }>("/api/v1/workflows") });
  // Custom fields the type carries: Workspace types take Workspace fields only; Space types also see their Space's fields.
  const workspaceFields = useQuery({ queryKey: ["custom-fields", "workspace"], enabled: !!type, queryFn: () => api<{ items: { id: string; name: string; archived_at?: string | null }[] }>("/api/v1/custom-fields") });
  const spaceFields = useQuery({ queryKey: ["custom-fields", type?.space_id ?? "none"], enabled: !!type?.space_id,
    queryFn: () => api<{ items: { id: string; name: string; archived_at?: string | null }[] }>(`/api/v1/spaces/${type?.space_id}/custom-fields`) });
  const assigned = useQuery({ queryKey: ["item-type-fields", type?.id], enabled: !!type,
    queryFn: () => api<{ items: { field_id: string }[] }>(`/api/v1/item-types/${type?.id}/fields`) });
  const [picked, setPicked] = useState<string[]>();
  const assignedIds = assigned.data?.items.map((f) => f.field_id) ?? [];
  const chosen = picked ?? assignedIds;
  const candidates = [...(workspaceFields.data?.items ?? []), ...(type?.space_id ? spaceFields.data?.items ?? [] : [])].filter((f) => !f.archived_at);
  const [inherit, setInherit] = useState(Array.isArray(type?.defaults?.inherit) && (type?.defaults?.inherit as string[]).includes("importance"));
  const [sections, setSections] = useState<Section[]>(((type?.layout as { sections?: Section[] } | undefined)?.sections) ?? []);
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
      layout: sections.length ? { sections: sections.filter((section) => section.title.trim()) } : {} };
    if (!type) Object.assign(body, { key: text("key"), base, space_id: scope || null });
    else body.active = active;
    setBusy(true); setError("");
    try {
      await api(type ? `/api/v1/item-types/${type.id}` : "/api/v1/item-types", { method: type ? "PATCH" : "POST", headers: type ? { "If-Match": `"${type.version}"` } : undefined, body: JSON.stringify(body) });
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
        <label className="block space-y-1 text-sm"><span>{t("settings.itemTypes.name")}</span>
          <Input name="name" defaultValue={type?.name ?? ""} maxLength={80} required /></label>
        {!type && <>
          <label className="block space-y-1 text-sm"><span>{t("settings.itemTypes.key")}</span>
            <Input name="key" pattern="[a-z][a-z0-9_]{1,39}" maxLength={40} required /></label>
          <label className="block space-y-1 text-sm"><span>{t("settings.itemTypes.base")}</span>
            <select className={selectClass} value={base} onChange={(event) => setBase(event.target.value as ItemTypeDef["base"])}>
              {kinds.map((kind) => <option key={kind} value={kind}>{t(`items.${kind}`)}</option>)}</select></label>
          {/* CIT-D01: a Workspace type works in every Space; a Space type works only in that Space. Fixed once created. */}
          <label className="block space-y-1 text-sm"><span>{t("settings.itemTypes.scope")}</span>
            <select className={selectClass} value={scope} onChange={(event) => setScope(event.target.value)}>
              <option value="">{t("settings.itemTypes.scopeWorkspace")}</option>
              {spaces.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}</select></label>
        </>}
        {type && <p className="text-sm text-muted-foreground">{t("settings.itemTypes.scope")}: {spaceName(type.space_id)}</p>}
        <label className="block space-y-1 text-sm"><span>{t("settings.itemTypes.icon")}</span>
          <Input name="icon" defaultValue={type?.icon ?? ""} maxLength={40} pattern="[a-z0-9-]*" /></label>
        {type && !type.is_builtin && <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={active} onChange={(event) => setActive(event.target.checked)} />{t("settings.itemTypes.active")}</label>}
        <fieldset className="space-y-1 text-sm"><legend className="font-medium">{t("settings.itemTypes.children")}</legend>
          <p className="text-muted-foreground">{t("settings.itemTypes.childrenHint")}</p>
          {eligible.map((other) => <label key={other.id} className="flex items-center gap-2">
            <input type="checkbox" checked={children.includes(other.id)} onChange={() => setChildren((rows) => rows.includes(other.id) ? rows.filter((x) => x !== other.id) : [...rows, other.id])} />{other.name}</label>)}
        </fieldset>
        <fieldset className="space-y-1 text-sm"><legend className="font-medium">{t("settings.itemTypes.fields")}</legend>
          {!type ? <p className="text-muted-foreground">{t("settings.itemTypes.fieldsSaveFirst")}</p> : <>
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
            <select name="importance" className={selectClass} defaultValue={String(type?.defaults?.importance ?? "")}>
              <option value="">—</option>{levels.map((level) => <option key={level} value={level}>{t(`items.importance.${level}`)}</option>)}</select></label>
          <label className="space-y-1"><span>{t("settings.itemTypes.duration")}</span>
            <Input name="duration_days" type="number" min={0} defaultValue={String(type?.defaults?.duration_days ?? "")} /></label>
          <label className="space-y-1"><span>{t("settings.itemTypes.effort")}</span>
            <Input name="effort_minutes" type="number" min={0} defaultValue={String(type?.defaults?.effort_minutes ?? "")} /></label>
          <label className="flex items-center gap-2 sm:col-span-3"><input type="checkbox" checked={inherit} onChange={(event) => setInherit(event.target.checked)} />{t("settings.itemTypes.inherit")}</label>
        </fieldset>
        <fieldset className="space-y-2 text-sm"><legend className="font-medium">{t("settings.itemTypes.layout")}</legend>
          <p className="text-muted-foreground">{t("settings.itemTypes.layoutHint")}</p>
          {sections.map((section, index) => <div key={index} className="space-y-1 rounded border p-2">
            <div className="flex items-end gap-2">
              <label className="min-w-0 flex-1 space-y-1"><span>{t("settings.itemTypes.sectionTitle", { number: index + 1 })}</span>
                <Input value={section.title} maxLength={60} onChange={(event) => updateSection(index, { title: event.target.value })} /></label>
              <Button type="button" variant="ghost" size="sm" aria-label={t("settings.itemTypes.removeSection", { number: index + 1 })}
                onClick={() => setSections((rows) => rows.filter((_, i) => i !== index))}>×</Button>
            </div>
            <div className="flex flex-wrap gap-3">{LAYOUT_BLOCKS.map((block) => {
              const usedElsewhere = sections.some((other, i) => i !== index && other.fields.includes(block));
              return <label key={block} className="flex items-center gap-1"><input type="checkbox" disabled={usedElsewhere} checked={section.fields.includes(block)} onChange={() => toggleBlock(index, block)} />{t(`settings.itemTypes.block.${block}`)}</label>;
            })}</div>
          </div>)}
          <Button type="button" variant="outline" size="sm" onClick={() => setSections((rows) => [...rows, { title: "", fields: [] }])}>{t("settings.itemTypes.addSection")}</Button>
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
