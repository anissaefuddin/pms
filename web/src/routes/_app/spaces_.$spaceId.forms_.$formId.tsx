import { useEffect, useRef, useState } from "react";
import { Link, createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import { buildInput, draftFromForm, emptyDraft, formKeys, mapErrors, type Answers, type Form, type FormDraft, type FormPage } from "@/lib/forms";
import { useSpace } from "@/lib/space-queries";
import { can } from "@/lib/spaces";
import { AccessForm } from "@/components/form-access-dialog";
import { FormEmailSettings } from "@/components/form-routing-settings";
import { FormError } from "@/components/auth/form-error";
import { AdminStatus } from "@/components/admin/admin-ui";
import { FormQuestionsEditor } from "@/components/form-questions-editor";
import { FormRenderer } from "@/components/form-renderer";
import { FormResponseSettings } from "@/components/form-response-settings";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/spaces_/$spaceId/forms_/$formId")({ component: FormBuilderRoute });

function FormBuilderRoute() {
  const { t } = useTranslation();
  const { spaceId, formId } = Route.useParams();
  const space = useSpace(spaceId);
  const isNew = formId === "new";
  const [nonce, setNonce] = useState(0);
  const form = useQuery({
    queryKey: formKeys.one(formId), enabled: !isNew, retry: false, staleTime: Infinity,
    queryFn: () => api<Form>(`/api/v1/forms/${formId}`),
  });
  const gate = (text: string) => <main className="p-8"><p role="alert">{text}</p>
    <Link to="/spaces/$spaceId/forms" params={{ spaceId }} className="mt-3 inline-block underline">{t("forms.builder.back")}</Link></main>;
  if (space.data && !can(space.data, "edit")) return gate(t("forms.builder.forbidden"));
  if (form.error instanceof ApiError && form.error.status === 403) return gate(t("forms.builder.forbidden"));
  if (form.isError) return gate(t("forms.builder.loadError"));
  if (space.isPending || (!isNew && form.isPending)) return <main className="p-8"><p role="status">{t("forms.builder.loading")}</p></main>;
  // keyed on the version (and a reload counter) so a save or "Reload" re-seeds the draft from the server copy
  return <Builder key={form.data ? `${form.data.id}:${form.data.version}:${nonce}` : "new"} spaceId={spaceId} form={form.data}
    onReload={async () => { await form.refetch(); setNonce((n) => n + 1); }} />;
}

/** Fills in names for the person chips (the API stores ids only). */
async function nameUsers(people: { id: string; name: string }[]) {
  return Promise.all(people.map(async (p) => {
    try {
      const u = await api<{ first_name: string; last_name: string }>(`/api/v1/users/${p.id}`);
      return { id: p.id, name: `${u.first_name} ${u.last_name}`.trim() };
    } catch { return p; }
  }));
}

function Builder({ spaceId, form, onReload }: { spaceId: string; form?: Form; onReload: () => Promise<void> }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [draft, setDraft] = useState<FormDraft>(() => form ? draftFromForm(form) : emptyDraft(spaceId));
  // keys saved on the server never change; only elements added in this session follow their label
  const lockedKeys = useRef(new Set(form?.definition.pages.flatMap((p) => p.elements.map((e) => e.key)) ?? []));
  const [answers, setAnswers] = useState<Answers>({});
  const [errors, setErrors] = useState<ReturnType<typeof mapErrors>>({ byControl: {}, summary: [] });
  const [conflict, setConflict] = useState(false);
  const [busy, setBusy] = useState(false);
  const patch = (next: Partial<FormDraft>) => setDraft((d) => ({ ...d, ...next }));

  useEffect(() => {
    if (!form) return;
    void Promise.all([nameUsers(draft.assignees), nameUsers(draft.approvers)]).then(([assignees, approvers]) =>
      setDraft((d) => ({ ...d, assignees, approvers })));
    // once per loaded version
  }, []);

  function onPages(pages: FormPage[], rename?: [string, string]) {
    setDraft((d) => ({ ...d, pages, title_question: rename && d.title_question === rename[0] ? rename[1] : d.title_question }));
  }

  // Save keeps the form's status. Publish saves first, then makes the saved version live for requesters.
  async function save(publish = false) {
    if (busy) return;
    setConflict(false);
    if (!draft.name.trim()) { setErrors({ byControl: { name: t("forms.builder.nameRequired") }, summary: [] }); return; }
    setBusy(true); setErrors({ byControl: {}, summary: [] });
    try {
      const body = JSON.stringify(buildInput(draft));
      let saved = form
        ? await api<Form>(`/api/v1/forms/${form.id}`, { method: "PUT", body, headers: { "If-Match": `"${form.version}"` } })
        : await api<Form>(`/api/v1/spaces/${spaceId}/forms`, { method: "POST", body });
      if (publish) saved = await api<Form>(`/api/v1/forms/${saved.id}/publish`, { method: "POST" });
      toast.success(t(publish ? "forms.builder.published" : "forms.builder.saved"));
      qc.setQueryData(formKeys.one(saved.id), saved);
      await qc.invalidateQueries({ queryKey: formKeys.list(spaceId) });
      if (!form) await navigate({ to: "/spaces/$spaceId/forms/$formId", params: { spaceId, formId: saved.id }, replace: true });
    } catch (err) {
      if (err instanceof ApiError && err.status === 409 && err.code !== "name_taken") setConflict(true);
      else if (err instanceof ApiError && err.code === "name_taken") setErrors({ byControl: { name: t("forms.builder.nameTaken") }, summary: [] });
      else if (err instanceof ApiError && err.fields.length) {
        // A field the form has no control for (such as the question count on publish) is shown as a summary line.
        const mapped = mapErrors(err.fields);
        setErrors(mapped.summary.length || Object.keys(mapped.byControl).length ? mapped : { byControl: {}, summary: [err.message] });
      }
      else setErrors({ byControl: {}, summary: [(err as Error).message] });
    } finally { setBusy(false); }
  }

  async function setPublished(publish: boolean) {
    if (!form || busy) return;
    setBusy(true); setErrors({ byControl: {}, summary: [] });
    try {
      const saved = await api<Form>(`/api/v1/forms/${form.id}/${publish ? "publish" : "unpublish"}`, { method: "POST" });
      toast.success(t(publish ? "forms.builder.published" : "forms.builder.unpublished"));
      qc.setQueryData(formKeys.one(saved.id), saved);
      await qc.invalidateQueries({ queryKey: formKeys.list(spaceId) });
    } catch (err) { setErrors({ byControl: {}, summary: [(err as Error).message] }); }
    finally { setBusy(false); }
  }

  const definition = { pages: draft.pages };
  const published = form?.status === "published";
  const nameError = errors.byControl.name ?? "";
  const descError = errors.byControl.description ?? "";
  const hasQuestions = draft.pages.some((p) => p.elements.length > 0);
  const sections = [
    { id: "sec-questions", label: t("forms.builder.sections.questions") },
    { id: "sec-submission", label: t("forms.builder.sections.submission") },
    { id: "sec-access", label: t("forms.builder.sections.access") },
    { id: "sec-confirmation", label: t("forms.builder.sections.confirmation") },
  ];
  return <main className="space-y-5 p-8">
    <header className="space-y-3 border-b pb-5">
      <Link to="/spaces/$spaceId/forms" params={{ spaceId }} className="text-sm text-muted-foreground underline">{t("forms.builder.back")}</Link>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="workspace-title">{t(form ? "forms.builder.titleEdit" : "forms.builder.titleNew")}</h1>
          <AdminStatus tone={published ? "success" : "neutral"} label={t(published ? "forms.builder.statusPublished" : "forms.builder.statusDraft")} />
        </div>
        <div className="flex flex-wrap gap-2">
          <Button type="button" variant="outline" disabled={busy} onClick={() => void save()}>{busy ? t("forms.builder.saving") : t("forms.builder.save")}</Button>
          {published
            ? <Button type="button" variant="outline" disabled={busy} onClick={() => void setPublished(false)}>{t("forms.builder.unpublish")}</Button>
            : <Button type="button" disabled={busy} onClick={() => void save(true)}>{t("forms.builder.publish")}</Button>}
        </div>
      </div>
    </header>
    {errors.summary.length > 0 && <div role="alert" className="rounded-lg border border-destructive p-3 text-sm text-destructive">
      <p className="font-medium">{t("forms.builder.errors")}</p>
      <ul className="list-disc pl-5">{errors.summary.map((m, i) => <li key={i}>{m}</li>)}</ul></div>}
    {conflict && <div role="alert" className="flex flex-wrap items-center gap-3 rounded-lg border border-destructive p-3 text-sm text-destructive">
      <span>{t("forms.builder.conflict")}</span>
      <Button type="button" size="sm" variant="outline" onClick={() => void onReload()}>{t("forms.builder.reload")}</Button></div>}
    {/* Full-page editor: section list on the left, the four sections in the middle, the live preview on the right. */}
    <div className="grid gap-6 xl:grid-cols-[11rem_minmax(0,1fr)_minmax(0,22rem)]">
      <nav aria-label={t("forms.builder.sectionsLabel")} className="flex flex-wrap gap-1 text-sm xl:sticky xl:top-4 xl:block xl:space-y-1 xl:self-start">
        {sections.map((section) => <a key={section.id} href={`#${section.id}`} className="block rounded-md px-2 py-1.5 text-muted-foreground hover:bg-muted hover:text-foreground">{section.label}</a>)}
      </nav>
      <div className="min-w-0 space-y-8">
        {/* The questions editor below has the section title, so this wrapper adds no landmark of its own. */}
        <div id="sec-questions" className="scroll-mt-4 space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="block space-y-1 text-sm"><span>{t("forms.builder.name")}</span>
              <Input value={draft.name} maxLength={120} aria-invalid={!!nameError} onChange={(e) => patch({ name: e.target.value })} /></label>
            <label className="block space-y-1 text-sm"><span>{t("forms.builder.description")}</span>
              <Input value={draft.description} aria-invalid={!!descError} onChange={(e) => patch({ description: e.target.value })} /></label>
            <FormError message={nameError} /><FormError message={descError} />
          </div>
          <FormQuestionsEditor pages={draft.pages} lockedKeys={lockedKeys.current} errors={errors.byControl} onChange={onPages} />
        </div>
        <div id="sec-submission" className="scroll-mt-4 space-y-4">
          <h2 id="sec-submission-title" className="text-lg font-semibold">{t("forms.builder.sections.submission")}</h2>
          <p className="text-sm text-muted-foreground">{t("forms.builder.submissionHint")}</p>
          <FormResponseSettings draft={draft} pages={draft.pages} errors={errors.byControl} onChange={patch} />
        </div>
        <div id="sec-access" className="scroll-mt-4 space-y-4">
          <h2 id="sec-access-title" className="text-lg font-semibold">{t("forms.builder.sections.access")}</h2>
          {form
            ? <div className="rounded-xl border p-4"><AccessForm key={`${form.id}:${form.access}`} form={form} /></div>
            : <p className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">{t("forms.builder.accessAfterSave")}</p>}
        </div>
        <div id="sec-confirmation" className="scroll-mt-4 space-y-4">
          <h2 id="sec-confirmation-title" className="text-lg font-semibold">{t("forms.builder.sections.confirmation")}</h2>
          <FormEmailSettings draft={draft} errors={errors.byControl} onChange={patch} />
        </div>
      </div>
      <section aria-labelledby="fp-title" className="min-w-0 space-y-3 self-start rounded-xl border p-4 xl:sticky xl:top-4">
        <h2 id="fp-title" className="text-lg font-semibold">{t("forms.preview.title")}</h2>
        {draft.name && <h3 className="text-base font-semibold">{draft.name}</h3>}
        {draft.description && <p className="text-sm text-muted-foreground">{draft.description}</p>}
        {hasQuestions ? <FormRenderer definition={definition} answers={answers} onAnswersChange={setAnswers} preview />
          : <p className="text-sm text-muted-foreground">{t("forms.preview.empty")}</p>}
      </section>
    </div>
  </main>;
}
