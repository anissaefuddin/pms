import { useEffect, useRef, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ApiError, api } from "@/lib/api";
import { answerErrors, buildSubmission, type Answers, type Files, type FormDefinition } from "@/lib/forms";
import { FormRenderer } from "@/components/form-renderer";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

// Outside `_app`: this page works without a session and never calls an endpoint that needs one.
export const Route = createFileRoute("/f/$token")({ component: PublicForm });

type Fill = { name: string; description: string; definition: FormDefinition };
const BANNERS = ["file_type_blocked", "file_too_large"];
const looksLikeEmail = (value: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);

function PublicForm() {
  const { t } = useTranslation();
  const { token } = Route.useParams();
  const form = useQuery({ queryKey: ["public-form", token], retry: false, queryFn: () => api<Fill>(`/api/v1/public/forms/${encodeURIComponent(token)}`) });
  const [answers, setAnswers] = useState<Answers>({});
  const [who, setWho] = useState({ name: "", email: "", website: "" });
  const [serverErrors, setServerErrors] = useState<Record<string, string>>();
  const [banner, setBanner] = useState("");
  const [busy, setBusy] = useState(false);
  const [sentTo, setSentTo] = useState<string>();
  const [round, setRound] = useState(0);
  const bannerRef = useRef<HTMLParagraphElement>(null);
  useEffect(() => { if (banner) bannerRef.current?.focus(); }, [banner]);

  async function submit(files: Files) {
    if (!form.data || busy) return;
    const name = who.name.trim(), email = who.email.trim();
    if (!name || !looksLikeEmail(email)) { setBanner(t("forms.public.needDetails")); return; }
    setBusy(true); setBanner(""); setServerErrors(undefined);
    try {
      await api(`/api/v1/public/forms/${encodeURIComponent(token)}/submissions`, buildSubmission(form.data.definition, answers, files, { ...who, name, email }));
      setSentTo(email);
    } catch (err) {
      if (!(err instanceof ApiError)) throw err;
      const { byKey, rest } = answerErrors(err.fields);
      if (Object.keys(byKey).length) setServerErrors(byKey);
      if (err.status === 429) setBanner(t("forms.public.rateLimited"));
      else if (err.status === 404) void form.refetch();
      else if (err.code && BANNERS.includes(err.code)) setBanner(t(`forms.requests.error.${err.code}`));
      else if (!Object.keys(byKey).length || rest.length) setBanner(rest[0] ?? t("forms.public.error"));
    } finally { setBusy(false); }
  }

  if (form.isPending) return <main className="p-8"><p role="status">{t("forms.public.loading")}</p></main>;
  if (form.isError) {
    const missing = form.error instanceof ApiError && form.error.status === 404;
    return <main className="mx-auto max-w-2xl space-y-3 p-8">
      <h1 className="text-2xl font-semibold">{t(missing ? "forms.public.notFound.title" : "forms.public.loadError")}</h1>
      {missing ? <p className="text-muted-foreground">{t("forms.public.notFound.description")}</p>
        : <Button variant="outline" onClick={() => void form.refetch()}>{t("forms.public.retry")}</Button>}
    </main>;
  }
  if (sentTo !== undefined) return <main className="mx-auto max-w-2xl space-y-4 p-8">
    <h1 className="text-2xl font-semibold" tabIndex={-1} ref={(el) => el?.focus()}>{t("forms.public.sent")}</h1>
    <p>{t("forms.public.sentDescription", { email: sentTo })}</p>
    <Button onClick={() => { setSentTo(undefined); setAnswers({}); setRound(round + 1); }}>{t("forms.public.another")}</Button>
  </main>;
  return <main className="mx-auto max-w-2xl space-y-5 p-8">
    <header><h1 className="text-2xl font-semibold">{form.data.name}</h1>
      {form.data.description && <p className="text-sm text-muted-foreground">{form.data.description}</p>}</header>
    {banner && <p ref={bannerRef} tabIndex={-1} role="alert" className="rounded-lg border border-destructive p-3 text-sm font-medium">{banner}</p>}
    <fieldset className="space-y-3 rounded-lg border p-4">
      <legend className="px-1 text-sm font-medium">{t("forms.public.about")}</legend>
      <label className="block space-y-1 text-sm"><span>{t("forms.public.name")}</span>
        <Input required maxLength={120} autoComplete="name" value={who.name} onChange={(e) => setWho({ ...who, name: e.target.value })} /></label>
      <label className="block space-y-1 text-sm"><span>{t("forms.public.email")}</span>
        <Input required type="email" maxLength={254} autoComplete="email" value={who.email} aria-describedby="public-email-help"
          onChange={(e) => setWho({ ...who, email: e.target.value })} />
        <span id="public-email-help" className="block text-xs text-muted-foreground">{t("forms.public.emailHelp")}</span></label>
      {/* Honeypot: hidden from people and assistive tech, so only a script that fills every field trips it. */}
      <div aria-hidden="true" className="absolute -left-[9999px] h-0 w-0 overflow-hidden">
        <label>{t("forms.public.honeypot")}<input tabIndex={-1} autoComplete="off" value={who.website} onChange={(e) => setWho({ ...who, website: e.target.value })} /></label>
      </div>
    </fieldset>
    <FormRenderer key={round} headingLevel={2} definition={form.data.definition} answers={answers} onAnswersChange={setAnswers}
      onSubmit={(files) => void submit(files)} submitting={busy} serverErrors={serverErrors} />
  </main>;
}
