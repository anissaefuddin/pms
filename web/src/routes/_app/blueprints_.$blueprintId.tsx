import { useMemo, useState } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError } from "@/lib/api";
import { blueprintDetailKey, blueprintKey, previewBlueprint, saveBlueprint, useBlueprintDetail, type BlueprintNode, type BlueprintPreviewItem, type BlueprintTree } from "@/lib/blueprints";
import { can } from "@/lib/spaces";
import { useSpaces } from "@/lib/space-queries";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/blueprints_/$blueprintId")({ component: BlueprintEditorPage });

type Mode = "start" | "finish";
const ITEM_TYPES = ["project", "folder", "task"] as const;
const IMPORTANCE = ["high", "normal", "low"] as const;
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
  const qc = useQueryClient();
  const detail = useBlueprintDetail(blueprintId);
  const spaces = useSpaces();
  const space = (spaces.data ?? []).find((s) => s.id === detail.data?.space_id);
  const canManage = !!space && can(space, "manage_blueprint");
  const canUse = !!space && can(space, "use_blueprint");

  if (detail.isPending) return <main className="p-8"><p role="status">{t("items.loading")}</p></main>;
  if (detail.isError || !detail.data) return <main className="p-8"><p role="alert">{t("blueprints.editor.notFound")}</p></main>;
  return <Editor key={`${detail.data.id}-${detail.data.version}`} blueprint={detail.data} canManage={canManage} canUse={canUse}
    onChanged={() => void qc.invalidateQueries({ queryKey: blueprintDetailKey(blueprintId) })} />;
}

function Editor({ blueprint, canManage, canUse, onChanged }: {
  blueprint: NonNullable<ReturnType<typeof useBlueprintDetail>["data"]>;
  canManage: boolean; canUse: boolean; onChanged: () => void;
}) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const original = blueprint.tree;
  const [name, setName] = useState(blueprint.name);
  const [tree, setTree] = useState<BlueprintTree>(() => structuredClone(original));
  const [version, setVersion] = useState(blueprint.version);
  const [error, setError] = useState("");
  const [conflict, setConflict] = useState(false);
  const [busy, setBusy] = useState(false);
  const [mode, setMode] = useState<Mode>("start");
  const [date, setDate] = useState("");
  const [preview, setPreview] = useState<BlueprintPreviewItem[] | null>(null);

  const dirty = name !== blueprint.name || JSON.stringify(tree) !== JSON.stringify(original);
  const childCount = useMemo(() => tree.nodes.map((_, i) => tree.nodes.filter((n) => n.parent === i).length), [tree]);

  function patchNode(index: number, change: Partial<BlueprintNode>) {
    setTree((current) => ({ ...current, nodes: current.nodes.map((n, i) => (i === index ? { ...n, ...change } : n)) }));
  }

  function addChild(parent: number) {
    setTree((current) => ({
      ...current,
      nodes: [...current.nodes, { parent, type: "task", title: t("blueprints.editor.newItem"), importance: "normal", assignee_ids: [], is_milestone: false }],
    }));
  }

  async function save() {
    setBusy(true); setError(""); setConflict(false);
    try {
      const saved = await saveBlueprint(blueprint.id, { version, name: name.trim(), tree });
      setVersion(saved.version);
      await qc.invalidateQueries({ queryKey: blueprintKey(blueprint.space_id) });
      onChanged();
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
        {tree.nodes.map((node, i) => <li key={i} className="grid gap-2 rounded-lg border bg-card p-3 sm:grid-cols-[minmax(0,2fr)_auto_auto_auto_auto_auto]"
          style={{ marginLeft: node.parent >= 0 ? "1.5rem" : 0 }}>
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
        </li>)}
      </ol>
      {canManage && <div className="flex flex-wrap items-center gap-3">
        <Button disabled={busy || !dirty || !name.trim()} onClick={() => void save()}>{t("blueprints.editor.save")}</Button>
        {conflict && <Button variant="outline" onClick={onChanged}>{t("blueprints.editor.reload")}</Button>}
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
