import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { useBlueprints } from "@/lib/blueprints";
import { useItemTypes } from "@/lib/item-types";
import { emptyEmail, type FormDraft, type FormPage } from "@/lib/forms";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

const select = "block w-full rounded-lg border border-input bg-background px-2 py-1.5 text-sm";
type Field = { id: string; name: string; type: string };
const mappable = ["text", "number", "date", "single_select", "multi_select"];
const emailInputs = [["sender_name", 80], ["logo_url", 500], ["greeting", 200], ["closing", 500]] as const;

type Props = { draft: FormDraft; pages: FormPage[]; errors: Record<string, string>; onChange: (patch: Partial<FormDraft>) => void };

/** FORM-09: which item type a submission creates (by answer) and which custom fields the answers fill. */
export function FormRoutingSettings({ draft, pages, errors, onChange }: Props) {
  const { t } = useTranslation();
  const types = (useItemTypes().data ?? []).filter((def) => !def.is_builtin && def.active && def.base === draft.item_type);
  const fields = useQuery({ queryKey: ["custom-fields", draft.space_id], enabled: !!draft.space_id, retry: false,
    queryFn: async () => (await api<{ items: Field[] }>(`/api/v1/spaces/${draft.space_id}/custom-fields`)).items });
  const blueprints = useBlueprints(draft.space_id, !!draft.space_id).data ?? [];
  const usable = (fields.data ?? []).filter((field) => mappable.includes(field.type));
  const questions = pages.flatMap((p) => p.elements).filter((e) => e.kind !== "section" && e.kind !== "attachment");
  const choices = questions.filter((e) => e.kind === "single_choice");
  const err = (id: string) => errors[id] ?? "";
  if (!types.length && !usable.length) return null;
  return <section aria-labelledby="fr-route" className="space-y-3">
    <h3 id="fr-route" className="font-medium">{t("forms.routing.title")}</h3>
    {types.length > 0 && <div className="space-y-3">
      <label className="block space-y-1 text-sm"><span>{t("forms.routing.type")}</span>
        <select className={select} value={draft.type_key} onChange={(e) => onChange({ type_key: e.target.value })}>
          <option value="">{t("forms.routing.builtin")}</option>
          {types.map((def) => <option key={def.key} value={def.key}>{def.name}</option>)}</select></label>
      <FormError message={err("response.type_key")} />
      {draft.routes.map((route, index) => {
        const source = choices.find((q) => q.key === route.question);
        const patch = (next: Partial<typeof route>) => onChange({ routes: draft.routes.map((r, i) => i === index ? { ...r, ...next } : r) });
        return <div key={index} className="flex flex-wrap items-end gap-2 text-sm">
          <label className="space-y-1"><span>{t("forms.routing.when", { number: index + 1 })}</span>
            <select className={select} value={route.question} onChange={(e) => patch({ question: e.target.value, value: "" })}>
              <option value="">—</option>{choices.map((q) => <option key={q.key} value={q.key}>{q.label || q.key}</option>)}</select></label>
          <label className="space-y-1"><span>{t("forms.routing.is", { number: index + 1 })}</span>
            <select className={select} value={route.value} onChange={(e) => patch({ value: e.target.value })}>
              <option value="">—</option>{(source?.options ?? []).filter(Boolean).map((o) => <option key={o} value={o}>{o}</option>)}</select></label>
          <label className="space-y-1"><span>{t("forms.routing.create", { number: index + 1 })}</span>
            <select className={select} value={route.type_key} onChange={(e) => patch({ type_key: e.target.value })}>
              <option value="">—</option>{types.map((def) => <option key={def.key} value={def.key}>{def.name}</option>)}</select></label>
          <label className="space-y-1"><span>{t("forms.routing.blueprint", { number: index + 1 })}</span>
            <select className={select} value={route.blueprint_id} onChange={(e) => patch({ blueprint_id: e.target.value })}>
              <option value="">{t("forms.routing.noBlueprint")}</option>{blueprints.map((b) => <option key={b.id} value={b.id}>{b.name}</option>)}</select></label>
          <p className="basis-full text-muted-foreground">{t("forms.routing.summary", { number: index + 1, question: source?.label || route.question || "—", value: route.value || "—",
            type: types.find((d) => d.key === route.type_key)?.name ?? "—", blueprint: blueprints.find((b) => b.id === route.blueprint_id)?.name ?? t("forms.routing.noBlueprint") })}</p>
          <Button type="button" variant="ghost" size="sm" aria-label={t("forms.routing.remove", { number: index + 1 })}
            onClick={() => onChange({ routes: draft.routes.filter((_, i) => i !== index) })}>×</Button>
        </div>;
      })}
      <FormError message={err("response.routes")} />
      <Button type="button" variant="outline" size="sm" disabled={draft.routes.length >= 10 || !choices.length}
        onClick={() => onChange({ routes: [...draft.routes, { question: "", value: "", type_key: "", blueprint_id: "" }] })}>{t("forms.routing.addRoute")}</Button>
    </div>}
    {usable.length > 0 && <fieldset className="space-y-2 text-sm"><legend className="font-medium">{t("forms.routing.fields")}</legend>
      <p className="text-muted-foreground">{t("forms.routing.fieldsHint")}</p>
      {questions.map((q) => <label key={q.key} className="flex flex-wrap items-center gap-2">
        <span className="min-w-40">{q.label || q.key}</span>
        <select className={`${select} max-w-60`} aria-label={t("forms.routing.fillField", { label: q.label || q.key })} value={draft.field_map[q.key] ?? ""}
          onChange={(e) => onChange({ field_map: { ...draft.field_map, [q.key]: e.target.value } })}>
          <option value="">{t("forms.routing.noField")}</option>{usable.map((field) => <option key={field.id} value={field.id}>{field.name}</option>)}</select>
      </label>)}
      <FormError message={err("response.field_map")} />
    </fieldset>}
  </section>;
}

/** FORM-04: the wording of the confirmation email for public requesters, with a preview. React escapes every value shown. */
export function FormEmailSettings({ draft, errors, onChange }: Omit<Props, "pages">) {
  const { t } = useTranslation();
  const email = draft.email;
  const set = (patch: Partial<typeof emptyEmail>) => onChange({ email: { ...email, ...patch } });
  const sample = (text: string) => text.replaceAll("{name}", "Sam Visitor").replaceAll("{title}", "Broken login").replaceAll("{form}", draft.name || t("forms.email.formName"));
  const input = (key: keyof typeof emptyEmail, max: number) => <label className="block space-y-1 text-sm"><span>{t(`forms.email.${key}`)}</span>
    <Input value={email[key]} maxLength={max} aria-invalid={!!errors[`response.email.${key}`]} onChange={(e) => set({ [key]: e.target.value })} />
    <FormError message={errors[`response.email.${key}`] ?? ""} /></label>;
  return <section aria-labelledby="fr-email" className="space-y-3">
    <h3 id="fr-email" className="font-medium">{t("forms.email.title")}</h3>
    <p className="text-sm text-muted-foreground">{t("forms.email.hint")}</p>
    <div className="grid gap-3 sm:grid-cols-2">{emailInputs.map(([key, max]) => <div key={key}>{input(key, max)}</div>)}</div>
    <label className="block space-y-1 text-sm"><span>{t("forms.email.message")}</span>
      <textarea className="block w-full rounded-lg border border-input bg-transparent px-2.5 py-1.5 text-sm" rows={3} maxLength={2000} value={email.message}
        aria-invalid={!!errors["response.email.message"]} onChange={(e) => set({ message: e.target.value })} /></label>
    <FormError message={errors["response.email.message"] ?? ""} />
    <div role="region" aria-label={t("forms.email.preview")} className="space-y-2 rounded-lg border bg-card p-3 text-sm">
      <p className="text-xs text-muted-foreground">{t("forms.email.from", { name: email.sender_name || t("forms.email.defaultSender") })}</p>
      {/^https:\/\//.test(email.logo_url) && <img src={email.logo_url} alt="" className="max-h-12" />}
      {email.greeting && <p>{sample(email.greeting)}</p>}
      <p className="whitespace-pre-line">{email.message ? sample(email.message) : t("forms.email.defaultMessage")}</p>
      {email.closing && <p className="whitespace-pre-line">{sample(email.closing)}</p>}
    </div>
  </section>;
}
