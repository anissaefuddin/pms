import { useMemo, useState } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import { blueprintDetailKey, blueprintKey, previewBlueprint, saveBlueprint, useBlueprintDetail, type BlueprintNode, type BlueprintPreviewItem, type BlueprintTree } from "@/lib/blueprints";
import type { CustomField } from "@/lib/custom-fields";
import type { User } from "@/lib/spaces";
import { can } from "@/lib/spaces";
import { useSpaces } from "@/lib/space-queries";
import { FormError } from "@/components/auth/form-error";
import { ItemUserPicker, type PersonOption } from "@/components/item-user-picker";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/blueprints_/$blueprintId")({ component: BlueprintEditorPage });

type Mode = "start" | "finish";
type Workflow = { id: string; name: string; is_default: boolean; statuses: { id: string; name: string }[] };
type Dependency = BlueprintTree["dependencies"][number];
const ITEM_TYPES = ["project", "folder", "task"] as const;
const IMPORTANCE = ["high", "normal", "low"] as const;
const DEPENDENCY_TYPES = ["FS", "SS", "FF", "SF"] as const;
// Custom fields the editor can change; other kinds are shown read-only and keep their stored value.
const EDITABLE_FIELDS = ["text", "number", "date", "checkbox", "single_select"] as const;
const field = "h-8 rounded-md border bg-background px-2 text-sm disabled:opacity-60";

/** Removing a node keeps the tree valid: children move up one index, and dependencies on it go. */
function removeNode(tree: BlueprintTree, index: number): BlueprintTree {
  const remap = (i: number) => (i > index ? i - 1 : i);
  return {
    nodes: tree.nodes.filter((_, i) => i !== index).map((n) => ({ ...n, parent: n.parent < 0 ? -1 : remap(n.parent) })),
    dependencies: tree.dependencies
      .filter((d) => d.predecessor !== index && d.successor !== index)
      .map((d) => ({ ...d, predecessor: remap(d.predecessor), successor: remap(d.successor) })),
  };
}

function BlueprintEditorPage() {
  const { t } = useTranslation();
  const { blueprintId } = Route.useParams();
  const detail = useBlueprintDetail(blueprintId);
  const spaces = useSpaces();
  const space = (spaces.data ?? []).find((s) => s.id === detail.data?.space_id);
  const canManage = !!space && can(space, "manage_blueprint");
  const canUse = !!space && can(space, "use_blueprint");
  // A new key remounts the editor with fresh data. It changes only on an explicit reload, never on a background refetch,
  // so unsaved edits are not dropped silently.
  const [generation, setGeneration] = useState(0);

  if (detail.isPending) return <main className="p-8"><p role="status">{t("items.loading")}</p></main>;
  if (detail.isError || !detail.data) return <main className="p-8"><p role="alert">{t("blueprints.editor.notFound")}</p></main>;
  return <Editor key={`${detail.data.id}-${generation}`} blueprint={detail.data} canManage={canManage} canUse={canUse}
    onReload={async () => { await detail.refetch(); setGeneration((g) => g + 1); }} />;
}

function Editor({ blueprint, canManage, canUse, onReload }: {
  blueprint: NonNullable<ReturnType<typeof useBlueprintDetail>["data"]>;
  canManage: boolean; canUse: boolean; onReload: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  // `saved` is what the server holds as of our last read or save; "dirty" means the draft differs from it.
  const [saved, setSaved] = useState(() => ({ name: blueprint.name, tree: structuredClone(blueprint.tree) }));
  const original = saved.tree;
  const [name, setName] = useState(blueprint.name);
  const [tree, setTree] = useState<BlueprintTree>(() => structuredClone(blueprint.tree));
  const [version, setVersion] = useState(blueprint.version);
  const [error, setError] = useState("");
  const [conflict, setConflict] = useState(false);
  const [busy, setBusy] = useState(false);
  const [mode, setMode] = useState<Mode>("start");
  const [date, setDate] = useState("");
  const [preview, setPreview] = useState<BlueprintPreviewItem[] | null>(null);

  // Lookups for the detail panels: statuses of the default workflow, the Space's custom fields, and assignee names.
  const workflows = useQuery({ queryKey: ["workflows"], enabled: canManage, queryFn: () => api<{ items: Workflow[] }>("/api/v1/workflows") });
  const statuses = useMemo(() => {
    const list = workflows.data?.items ?? [];
    return (list.find((w) => w.is_default) ?? list[0])?.statuses ?? [];
  }, [workflows.data]);
  const fields = useQuery({
    queryKey: ["space-custom-fields", blueprint.space_id], enabled: canManage,
    queryFn: () => api<{ items: CustomField[] }>(`/api/v1/spaces/${blueprint.space_id}/custom-fields`),
  });
  const assigneeIds = useMemo(() => [...new Set(tree.nodes.flatMap((n) => n.assignee_ids ?? []))], [tree]);
  const people = useQueries({
    queries: assigneeIds.map((id) => ({ queryKey: ["user-name", id], queryFn: () => api<User>(`/api/v1/users/${id}`), retry: false })),
  });
  const names = useMemo(() => {
    const map = new Map<string, string>();
    people.forEach((q, i) => { if (q.data) map.set(assigneeIds[i], `${q.data.first_name} ${q.data.last_name}`.trim()); });
    return map;
  }, [people, assigneeIds]);
  const personOf = (id: string): PersonOption => ({ id, name: names.get(id) ?? id });

  const dirty = name !== saved.name || JSON.stringify(tree) !== JSON.stringify(original);
  const childCount = useMemo(() => tree.nodes.map((_, i) => tree.nodes.filter((n) => n.parent === i).length), [tree]);
  const editableFields = (fields.data?.items ?? []).filter((f) => !f.archived_at);

  function patchNode(index: number, change: Partial<BlueprintNode>) {
    setTree((current) => ({ ...current, nodes: current.nodes.map((n, i) => (i === index ? { ...n, ...change } : n)) }));
  }

  function setField(index: number, key: string, value: unknown) {
    setTree((current) => ({
      ...current,
      nodes: current.nodes.map((n, i) => {
        if (i !== index) return n;
        const values: Record<string, unknown> = { ...(n.fields ?? {}) };
        if (value === null || value === "" || value === undefined) delete values[key]; else values[key] = value;
        return { ...n, fields: values };
      }),
    }));
  }

  function patchDependency(index: number, change: Partial<Dependency>) {
    setTree((current) => ({ ...current, dependencies: current.dependencies.map((d, i) => (i === index ? { ...d, ...change } : d)) }));
  }

  function addChild(parent: number) {
    setTree((current) => ({
      ...current,
      nodes: [...current.nodes, { parent, type: "task", title: t("blueprints.editor.newItem"), importance: "normal", assignee_ids: [], is_milestone: false }],
    }));
  }

  function addDependency() {
    setTree((current) => {
      const count = current.nodes.length;
      if (count < 2) return current;
      return { ...current, dependencies: [...current.dependencies, { predecessor: 0, successor: 1, type: "FS", lag: 0 }] };
    });
  }

  const dependencyProblem = (d: Dependency) => (d.predecessor === d.successor ? t("blueprints.editor.selfDependency") : "");

  async function save() {
    setBusy(true); setError(""); setConflict(false);
    try {
      const result = await saveBlueprint(blueprint.id, { version, name: name.trim(), tree });
      setVersion(result.version);
      setSaved({ name: name.trim(), tree: structuredClone(tree) });
      setName(name.trim());
      await qc.invalidateQueries({ queryKey: blueprintKey(blueprint.space_id) });
      await qc.invalidateQueries({ queryKey: blueprintDetailKey(blueprint.id) });
      toast.success(t("blueprints.editor.saved"));
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) setConflict(true);
      setError(err instanceof ApiError ? err.message : t("blueprints.editor.saveError"));
    } finally {
      setBusy(false);
    }
  }

  async function runPreview() {
    setError(""); setPreview(null);
    try {
      const body = mode === "start" ? { start_date: date } : { finish_date: date };
      setPreview((await previewBlueprint(blueprint.id, body)).items);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("blueprints.editor.previewError"));
    }
  }

  const blockingProblem = tree.dependencies.map(dependencyProblem).find(Boolean) ?? "";

  return <main className="space-y-6 p-4 sm:p-8">
    <header className="space-y-2 border-b pb-5">
      <Link to="/blueprints" className="text-sm text-muted-foreground underline">{t("blueprints.editor.back")}</Link>
      <h1 className="workspace-title">{t("blueprints.editor.title")}</h1>
      <p className="text-sm text-muted-foreground">{t(`blueprints.type.${blueprint.item_type}`)} · {t("blueprints.count", { count: tree.nodes.length })} · {t("blueprints.editor.version", { version })}</p>
      {!canManage && <p role="status" className="text-sm">{t("blueprints.editor.readOnly")}</p>}
    </header>

    <section aria-labelledby="bp-name" className="space-y-3">
      <h2 id="bp-name" className="text-base font-semibold">{t("blueprints.editor.details")}</h2>
      <label className="block max-w-md space-y-1"><span>{t("blueprints.name")}</span>
        <Input name="name" maxLength={120} value={name} disabled={!canManage} onChange={(e) => setName(e.target.value)} />
      </label>
    </section>

    <section aria-labelledby="bp-items" className="space-y-3">
      <h2 id="bp-items" className="text-base font-semibold">{t("blueprints.editor.items")}</h2>
      <ol className="space-y-2">
        {tree.nodes.map((node, i) => <li key={i} className="space-y-2 rounded-lg border bg-card p-3" style={{ marginLeft: node.parent >= 0 ? "1.5rem" : 0 }}>
          <div className="grid gap-2 sm:grid-cols-[minmax(0,2fr)_auto_auto_auto_auto_auto]">
            <label className="space-y-1"><span className="sr-only">{t("blueprints.editor.itemTitle")}</span>
              <Input value={node.title} maxLength={500} disabled={!canManage} onChange={(e) => patchNode(i, { title: e.target.value })} />
            </label>
            <label className="space-y-1"><span className="sr-only">{t("blueprints.kind")}</span>
              <select className={field} value={node.type} disabled={!canManage || i === 0} onChange={(e) => patchNode(i, { type: e.target.value as BlueprintNode["type"] })}>
                {ITEM_TYPES.map((type) => <option key={type} value={type}>{t(`blueprints.type.${type}`)}</option>)}
              </select>
            </label>
            <label className="space-y-1"><span className="sr-only">{t("blueprints.editor.importance")}</span>
              <select className={field} value={node.importance} disabled={!canManage} onChange={(e) => patchNode(i, { importance: e.target.value as BlueprintNode["importance"] })}>
                {IMPORTANCE.map((level) => <option key={level} value={level}>{t(`blueprints.editor.importanceLevel.${level}`)}</option>)}
              </select>
            </label>
            <label className="space-y-1"><span className="text-xs text-muted-foreground">{t("blueprints.editor.startDay")}</span>
              <input type="number" min={0} className={`${field} w-24`} value={node.start_offset ?? ""} disabled={!canManage}
                onChange={(e) => patchNode(i, { start_offset: e.target.value === "" ? null : Number(e.target.value) })} />
            </label>
            <label className="space-y-1"><span className="text-xs text-muted-foreground">{t("blueprints.editor.dueDay")}</span>
              <input type="number" min={0} className={`${field} w-24`} value={node.due_offset ?? ""} disabled={!canManage}
                onChange={(e) => patchNode(i, { due_offset: e.target.value === "" ? null : Number(e.target.value) })} />
            </label>
            <div className="flex items-end gap-2">
              {canManage && <Button variant="outline" onClick={() => addChild(i)}>{t("blueprints.editor.addChild")}</Button>}
              {canManage && i > 0 && childCount[i] === 0 && <Button variant="outline" onClick={() => setTree((current) => removeNode(current, i))}>{t("blueprints.editor.remove")}</Button>}
            </div>
          </div>
          <details className="text-sm">
            <summary className="cursor-pointer text-muted-foreground">{t("blueprints.editor.detailsOf")}</summary>
            <div className="mt-2 grid gap-3 sm:grid-cols-2">
              <label className="space-y-1"><span>{t("blueprints.editor.status")}</span>
                <select className={field} value={node.status_id ?? ""} disabled={!canManage}
                  onChange={(e) => patchNode(i, { status_id: e.target.value || null })}>
                  <option value="">{t("blueprints.editor.noStatus")}</option>
                  {statuses.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
                </select>
              </label>
              <div className="space-y-1">
                <ItemUserPicker label={t("blueprints.editor.assignees")} multiple disabled={!canManage}
                  selected={(node.assignee_ids ?? []).map(personOf)}
                  onChange={(picked) => patchNode(i, { assignee_ids: picked.map((p) => p.id) })} />
              </div>
              {editableFields.map((f) => {
                const value = (node.fields ?? {})[f.id];
                if (!(EDITABLE_FIELDS as readonly string[]).includes(f.type)) {
                  return <p key={f.id} className="text-muted-foreground sm:col-span-2">{f.name}: {t("blueprints.editor.readOnlyField")}</p>;
                }
                return <label key={f.id} className="space-y-1"><span>{f.name}</span>
                  {f.type === "checkbox" ? <input type="checkbox" checked={value === true} disabled={!canManage} onChange={(e) => setField(i, f.id, e.target.checked)} />
                    : f.type === "single_select" ? <select className={field} value={typeof value === "string" ? value : ""} disabled={!canManage} onChange={(e) => setField(i, f.id, e.target.value || null)}>
                      <option value="">—</option>
                      {f.options.map((o) => <option key={o.id} value={o.id}>{o.label}</option>)}
                    </select>
                    : <input type={f.type === "number" ? "number" : f.type === "date" ? "date" : "text"} className={field}
                      value={typeof value === "number" || typeof value === "string" ? String(value) : ""} disabled={!canManage}
                      onChange={(e) => setField(i, f.id, f.type === "number" ? (e.target.value === "" ? null : Number(e.target.value)) : e.target.value)} />}
                </label>;
              })}
            </div>
          </details>
        </li>)}
      </ol>

      <div className="space-y-3">
        <h3 className="text-sm font-semibold">{t("blueprints.editor.dependencies")}</h3>
        {tree.dependencies.length === 0 && <p className="text-sm text-muted-foreground">{t("blueprints.editor.noDependencies")}</p>}
        {tree.dependencies.map((d, i) => <div key={i} className="flex flex-wrap items-end gap-2">
          <label className="space-y-1"><span className="text-xs text-muted-foreground">{t("blueprints.editor.predecessor")}</span>
            <select className={field} value={d.predecessor} disabled={!canManage} onChange={(e) => patchDependency(i, { predecessor: Number(e.target.value) })}>
              {tree.nodes.map((n, j) => <option key={j} value={j}>{n.title}</option>)}
            </select>
          </label>
          <label className="space-y-1"><span className="text-xs text-muted-foreground">{t("blueprints.editor.successor")}</span>
            <select className={field} value={d.successor} disabled={!canManage} onChange={(e) => patchDependency(i, { successor: Number(e.target.value) })}>
              {tree.nodes.map((n, j) => <option key={j} value={j}>{n.title}</option>)}
            </select>
          </label>
          <label className="space-y-1"><span className="text-xs text-muted-foreground">{t("blueprints.editor.dependencyType")}</span>
            <select className={field} value={d.type} disabled={!canManage} onChange={(e) => patchDependency(i, { type: e.target.value as Dependency["type"] })}>
              {DEPENDENCY_TYPES.map((kind) => <option key={kind} value={kind}>{kind}</option>)}
            </select>
          </label>
          <label className="space-y-1"><span className="text-xs text-muted-foreground">{t("blueprints.editor.lag")}</span>
            <input type="number" className={`${field} w-20`} value={d.lag} disabled={!canManage} onChange={(e) => patchDependency(i, { lag: Number(e.target.value || 0) })} />
          </label>
          {canManage && <Button variant="outline" onClick={() => setTree((current) => ({ ...current, dependencies: current.dependencies.filter((_, k) => k !== i) }))}>{t("blueprints.editor.remove")}</Button>}
          {dependencyProblem(d) && <p role="alert" className="text-sm">{dependencyProblem(d)}</p>}
        </div>)}
        {canManage && <Button variant="outline" disabled={tree.nodes.length < 2} onClick={addDependency}>{t("blueprints.editor.addDependency")}</Button>}
      </div>

      {canManage && <div className="flex flex-wrap items-center gap-3">
        <Button disabled={busy || !dirty || !name.trim() || !!blockingProblem} onClick={() => void save()}>{t("blueprints.editor.save")}</Button>
        {conflict && <Button variant="outline" onClick={() => void onReload()}>{t("blueprints.editor.reload")}</Button>}
      </div>}
      <FormError message={error} />
      {conflict && <p role="status">{t("blueprints.editor.conflict")}</p>}
      {dirty && <p className="text-sm text-muted-foreground">{t("blueprints.editor.unsaved")}</p>}
    </section>

    {canUse && <section aria-labelledby="bp-preview" className="space-y-3">
      <h2 id="bp-preview" className="text-base font-semibold">{t("blueprints.editor.previewTitle")}</h2>
      <p className="text-sm text-muted-foreground">{t("blueprints.editor.previewNote")}</p>
      <div className="flex flex-wrap items-end gap-3">
        <label className="space-y-1"><span>{t("blueprints.editor.previewBy")}</span>
          <select aria-label={t("blueprints.editor.previewBy")} className={field} value={mode} onChange={(e) => setMode(e.target.value as Mode)}>
            <option value="start">{t("blueprints.editor.byStart")}</option>
            <option value="finish">{t("blueprints.editor.byFinish")}</option>
          </select>
        </label>
        <label className="space-y-1"><span>{mode === "start" ? t("blueprints.editor.byStart") : t("blueprints.editor.byFinish")}</span>
          <input type="date" aria-label={mode === "start" ? t("blueprints.editor.byStart") : t("blueprints.editor.byFinish")} className={field} value={date} onChange={(e) => setDate(e.target.value)} />
        </label>
        <Button variant="outline" disabled={!date || dirty} onClick={() => void runPreview()}>{t("blueprints.editor.preview")}</Button>
      </div>
      {preview && <table className="w-full max-w-3xl text-sm" aria-label={t("blueprints.editor.previewTitle")}>
        <thead><tr className="text-left"><th>{t("blueprints.editor.itemTitle")}</th><th>{t("blueprints.editor.startDate")}</th><th>{t("blueprints.editor.dueDate")}</th></tr></thead>
        <tbody>{preview.map((row, i) => <tr key={i} className="border-t">
          <td className="py-1" style={{ paddingLeft: row.parent >= 0 ? "1.5rem" : 0 }}>{row.title}</td>
          <td>{row.start_date ?? "—"}</td><td>{row.due_date ?? "—"}</td>
        </tr>)}</tbody>
      </table>}
    </section>}
  </main>;
}
