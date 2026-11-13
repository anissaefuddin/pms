import { useEffect, useRef, useState } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ApiError, api } from "@/lib/api";
import { answerErrors, buildSubmission, type Answers, type Files, type FormDefinition } from "@/lib/forms";
import { FormRenderer } from "@/components/form-renderer";
import { Button, buttonVariants } from "@/components/ui/button";

export const Route = createFileRoute("/_app/requests/$formId")({ component: Fill });

type Fill = { id: string; name: string; description: string; definition: FormDefinition };
const BANNERS = ["form_unavailable", "file_type_blocked", "file_too_large"];

function Fill() {
  const { t } = useTranslation();
  const { formId } = Route.useParams();
  const form = useQuery({ queryKey: ["forms", "fill", formId], retry: false, queryFn: () => api<Fill>(`/api/v1/forms/${formId}/definition`) });
  const [answers, setAnswers] = useState<Answers>({});
  const [serverErrors, setServerErrors] = useState<Record<string, string>>();
  const [banner, setBanner] = useState("");
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState<string>();
  const [round, setRound] = useState(0);
  const bannerRef = useRef<HTMLParagraphElement>(null);
  useEffect(() => { if (banner) bannerRef.current?.focus(); }, [banner]);

  async function submit(files: Files) {
    if (!form.data || busy) return;
    setBusy(true); setBanner(""); setServerErrors(undefined);
    try {
      const result = await api<{ title: string }>(`/api/v1/forms/${formId}/submissions`, buildSubmission(form.data.definition, answers, files));
      setSent(result.title);
    } catch (err) {
      if (!(err instanceof ApiError)) throw err;
      const { byKey, rest } = answerErrors(err.fields);
      if (Object.keys(byKey).length) setServerErrors(byKey);
      if (err.code && BANNERS.includes(err.code)) setBanner(t(`forms.requests.error.${err.code}`));
      else if (!Object.keys(byKey).length || rest.length) setBanner(rest[0] ?? t("forms.requests.error.generic"));
    } finally { setBusy(false); }
  }

  const back = <Link to="/requests" className="text-sm text-muted-foreground underline">{t("forms.requests.back")}</Link>;
  if (form.isPending) return <main className="p-8"><p role="status">{t("forms.requests.loading")}</p></main>;
  if (form.isError) {
    const missing = form.error instanceof ApiError && form.error.status === 404;
    return <main className="space-y-3 p-8">{back}
      <h1 className="workspace-title">{t(missing ? "forms.requests.notFound.title" : "forms.requests.loadError")}</h1>
      {missing && <p className="text-muted-foreground">{t("forms.requests.notFound.description")}</p>}</main>;
  }
  if (sent !== undefined) return <main className="space-y-4 p-8">
    <h1 className="workspace-title" tabIndex={-1} ref={(el) => el?.focus()}>{t("forms.requests.sent")}</h1>
    <p>{t("forms.requests.sentDescription", { title: sent })}</p>
    <div className="flex gap-2">
      <Button onClick={() => { setSent(undefined); setAnswers({}); setRound(round + 1); }}>{t("forms.requests.another")}</Button>
      <Link to="/requests" className={buttonVariants({ variant: "outline" })}>{t("forms.requests.back")}</Link>
      <Link to="/requests/mine" className={buttonVariants({ variant: "outline" })}>{t("forms.mine.link")}</Link></div>
  </main>;
  return <main className="max-w-2xl space-y-5 p-8">
    {back}
    <header><h1 className="workspace-title">{form.data.name}</h1>
      {form.data.description && <p className="text-sm text-muted-foreground">{form.data.description}</p>}</header>
    {banner && <p ref={bannerRef} tabIndex={-1} role="alert" className="rounded-lg border border-destructive p-3 text-sm font-medium">{banner}</p>}
    <FormRenderer key={round} headingLevel={2} definition={form.data.definition} answers={answers} onAnswersChange={setAnswers}
      onSubmit={(files) => void submit(files)} submitting={busy} serverErrors={serverErrors} />
  </main>;
}
