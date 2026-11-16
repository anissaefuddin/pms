import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { itemKeys, listItems } from "@/lib/items";
import { useSpaces } from "@/lib/space-queries";
import type { FormDraft, FormPage } from "@/lib/forms";
import type { Workflow } from "@/lib/status-colors";
import { FormError } from "@/components/auth/form-error";
import { ItemUserPicker } from "@/components/item-user-picker";
import { Input } from "@/components/ui/input";
import { FormRoutingSettings } from "@/components/form-routing-settings";

const select = "block w-full rounded-lg border border-input bg-background px-2 py-1.5 text-sm";

type Props = { draft: FormDraft; pages: FormPage[]; errors: Record<string, string>; onChange: (patch: Partial<FormDraft>) => void };

export function FormResponseSettings({ draft, pages, errors, onChange }: Props) {
  const { t } = useTranslation();
  const spaces = useSpaces();
  const choices = spaces.data?.filter((s) => s.kind === "shared" && s.my_capabilities?.includes("create")) ?? [];
  const parents = useQuery({
    queryKey: itemKeys.root(draft.space_id), enabled: !!draft.space_id,
    queryFn: () => listItems(`/api/v1/spaces/${draft.space_id}/items`),
  });
  const workflows = useQuery({ queryKey: ["workflows"], queryFn: () => api<{ items: Workflow[] }>("/api/v1/workflows") });
  const statuses = (workflows.data?.items.find((w) => w.is_default) ?? workflows.data?.items[0])?.statuses ?? [];
  // shortcut: parents are limited to the Space root; browsing deeper folders can come with the submission story.
  const parentItems = parents.data?.filter((item) => item.type !== "task" && item.my_capabilities.includes("create")) ?? [];
  const titles = pages.flatMap((p) => p.elements).filter((e) => e.kind === "single_line");
  const err = (id: string) => errors[id] ?? "";
  const bad = (id: string) => !!errors[id];

  return <section aria-labelledby="fr-title" className="space-y-4 rounded-xl border p-4">
    <h2 id="fr-title" className="text-lg font-semibold">{t("forms.response.title")}</h2>
    <div className="grid gap-4 sm:grid-cols-2">
      <div className="space-y-1">
        <label className="block text-sm" htmlFor="fr-type">{t("forms.response.itemType")}</label>
        <select id="fr-type" className={select} value={draft.item_type} aria-invalid={bad("response.item_type")} onChange={(e) => onChange({ item_type: e.target.value as FormDraft["item_type"] })}>
          <option value="task">{t("forms.response.task")}</option><option value="project">{t("forms.response.project")}</option></select>
        <FormError message={err("response.item_type")} />
      </div>
      <div className="space-y-1">
        <label className="block text-sm" htmlFor="fr-space">{t("forms.response.space")}</label>
        <select id="fr-space" className={select} value={draft.space_id} aria-invalid={bad("response.location.space_id")}
          onChange={(e) => onChange({ space_id: e.target.value, parent_item_id: "" })}>
          {!choices.some((s) => s.id === draft.space_id) && <option value={draft.space_id}>{t("forms.response.unknownSpace")}</option>}
          {choices.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}</select>
        <FormError message={err("response.location.space_id")} />
      </div>
      <div className="space-y-1">
        <label className="block text-sm" htmlFor="fr-parent">{t("forms.response.parent")}</label>
        <select id="fr-parent" className={select} value={draft.parent_item_id} aria-invalid={bad("response.location.parent_item_id")} onChange={(e) => onChange({ parent_item_id: e.target.value })}>
          <option value="">{t("forms.response.parentNone")}</option>
          {draft.parent_item_id && !parentItems.some((i) => i.id === draft.parent_item_id) && <option value={draft.parent_item_id}>{t("forms.response.unknownParent")}</option>}
          {parentItems.map((i) => <option key={i.id} value={i.id}>{i.title}</option>)}</select>
        <FormError message={err("response.location.parent_item_id")} />
      </div>
      <div className="space-y-1">
        <label className="block text-sm" htmlFor="fr-status">{t("forms.response.status")}</label>
        <select id="fr-status" className={select} value={draft.status_id} aria-invalid={bad("response.status_id")} onChange={(e) => onChange({ status_id: e.target.value })}>
          <option value="">{t("forms.response.statusDefault")}</option>
          {statuses.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}</select>
        <FormError message={err("response.status_id")} />
      </div>
      <div className="space-y-1">
        <label className="block text-sm" htmlFor="fr-title-q">{t("forms.response.titleQuestion")}</label>
        <select id="fr-title-q" className={select} value={draft.title_question} aria-invalid={bad("response.title_question")} onChange={(e) => onChange({ title_question: e.target.value })}>
          <option value="">{t("forms.response.titleNone")}</option>
          {titles.map((e) => <option key={e.key} value={e.key}>{e.label || e.key}</option>)}</select>
        <FormError message={err("response.title_question")} />
      </div>
    </div>
    <div>
      <ItemUserPicker label={t("forms.response.assignees")} multiple maxSelected={20} selected={draft.assignees} onChange={(assignees) => onChange({ assignees })} />
      <FormError message={err("response.assignee_ids")} />
    </div>
    <div className="space-y-3">
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={draft.approval} onChange={(e) => onChange({ approval: e.target.checked })} />{t("forms.response.approval")}</label>
      {draft.approval && <div className="space-y-3 pl-6">
        <ItemUserPicker label={t("forms.response.approvers")} multiple maxSelected={20} selected={draft.approvers} onChange={(approvers) => onChange({ approvers })} />
        <FormError message={err("response.approval.approver_ids")} />
        <label className="block space-y-1 text-sm"><span>{t("forms.response.approvalDescription")}</span>
          <Input value={draft.approval_description} aria-invalid={bad("response.approval.description")} onChange={(e) => onChange({ approval_description: e.target.value })} /></label>
        <FormError message={err("response.approval.description")} />
      </div>}
    </div>
    <FormRoutingSettings draft={draft} pages={pages} errors={errors} onChange={onChange} />
  </section>;
}
