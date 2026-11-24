import { useRef, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, type CurrentUser } from "@/lib/api";
import { spaceKeys, type Space } from "@/lib/spaces";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

const steps = 4;
const viewKinds = ["table", "board", "gantt", "calendar"] as const;
const post = (path: string, body?: unknown, method = "POST") => api<{ id: string }>(path, { method, body: body === undefined ? undefined : JSON.stringify(body) });

/** First-run wizard for the administrator (AUTH-07): shown until it is finished or skipped, once per workspace. */
export function OnboardingWizard({ me }: { me: CurrentUser }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const client = useQueryClient();
  const [step, setStep] = useState(0);
  const [org, setOrg] = useState(me.account_name);
  const [project, setProject] = useState("");
  const [tasks, setTasks] = useState(["", "", ""]);
  const [view, setView] = useState<(typeof viewKinds)[number]>("table");
  const [emails, setEmails] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  // What already exists on the server, so "try again" after a failure resumes instead of duplicating.
  const made = useRef<{ space?: string; project?: string; tasks: number }>({ tasks: 0 });

  async function finish(open?: string) {
    await post("/api/v1/onboarding/complete");
    // DR-04: close the wizard from what we just saved instead of waiting for (or trusting) a refetch of /me.
    client.setQueryData<CurrentUser>(["me"], (old) => old && { ...old, onboarding_pending: false });
    await client.invalidateQueries({ queryKey: spaceKeys.all });
    if (open) await navigate({ to: "/tasks/$itemId", params: { itemId: open } });
  }
  async function guarded(action: () => Promise<void>) {
    setBusy(true);
    setError("");
    try { await action(); } catch { setError(t("workspace.onboarding.error")); } finally { setBusy(false); }
  }
  const build = () => guarded(async () => {
    await post("/api/v1/account", { name: org.trim() }, "PATCH");
    made.current.space ??= (await api<Space>("/api/v1/spaces", { method: "POST", body: JSON.stringify({ name: org.trim() }) })).id;
    const space = made.current.space;
    made.current.project ??= (await post("/api/v1/items", { type: "project", title: project.trim(), default_view_type: view, owner_id: me.id, locations: [{ space_id: space }] })).id;
    const titles = tasks.map((title) => title.trim()).filter(Boolean);
    for (; made.current.tasks < titles.length; made.current.tasks++) {
      await post("/api/v1/items", { type: "task", title: titles[made.current.tasks], locations: [{ space_id: space, parent_item_id: made.current.project }] });
    }
    setStep(3);
  });
  const invite = () => guarded(async () => {
    const failed: string[] = [];
    for (const email of emails.split(/[\s,;]+/).filter(Boolean)) {
      try { await post("/api/v1/users/invitations", { email, user_type: "regular" }); } catch { failed.push(email); }
    }
    if (failed.length) { setEmails(failed.join("\n")); setError(t("workspace.onboarding.invite.failed", { emails: failed.join(", ") })); return; }
    await finish(made.current.project);
  });

  const valid = [org.trim(), project.trim(), true, true][step];
  return (
    <Dialog open onOpenChange={() => undefined /* leaving is explicit: Skip setup */}>
      <DialogContent>
        <DialogTitle className="text-lg font-semibold">{t("workspace.onboarding.title")}</DialogTitle>
        <p className="text-sm text-muted-foreground">{t("workspace.onboarding.step", { current: step + 1, total: steps })}</p>
        {step === 0 && <label className="block space-y-1 text-sm"><span>{t("workspace.onboarding.org.hint")}</span>
          <span className="block font-medium">{t("workspace.onboarding.org.label")}</span>
          <Input value={org} maxLength={120} onChange={(event) => setOrg(event.target.value)} autoFocus /></label>}
        {step === 1 && <div className="space-y-2 text-sm"><p>{t("workspace.onboarding.project.hint")}</p>
          <label className="block space-y-1"><span className="font-medium">{t("workspace.onboarding.project.label")}</span>
            <Input value={project} maxLength={500} onChange={(event) => setProject(event.target.value)} autoFocus /></label>
          {tasks.map((title, i) => <label key={i} className="block space-y-1"><span>{t("workspace.onboarding.project.task", { number: i + 1 })}</span>
            <Input value={title} maxLength={500} onChange={(event) => setTasks((rows) => rows.map((row, j) => (j === i ? event.target.value : row)))} /></label>)}
        </div>}
        {step === 2 && <fieldset className="space-y-2 text-sm"><legend>{t("workspace.onboarding.view.hint")}</legend>
          {viewKinds.map((kind) => <label key={kind} className="flex items-center gap-2">
            <input type="radio" name="view" checked={view === kind} onChange={() => setView(kind)} />{t(`workspace.onboarding.view.${kind}`)}</label>)}
        </fieldset>}
        {step === 3 && <label className="block space-y-1 text-sm"><span>{t("workspace.onboarding.invite.hint")}</span>
          <span className="block font-medium">{t("workspace.onboarding.invite.label")}</span>
          <textarea className="w-full rounded-lg border border-input bg-transparent px-2.5 py-1.5" rows={4} value={emails} onChange={(event) => setEmails(event.target.value)} /></label>}
        <FormError message={error} />
        <DialogFooter>
          <Button variant="ghost" disabled={busy} onClick={() => void guarded(() => finish())}>{t("workspace.onboarding.skip")}</Button>
          {step > 0 && step < 3 && <Button variant="outline" disabled={busy} onClick={() => setStep(step - 1)}>{t("workspace.onboarding.back")}</Button>}
          {step < 2 && <Button disabled={!valid || busy} onClick={() => setStep(step + 1)}>{t("workspace.onboarding.next")}</Button>}
          {step === 2 && <Button disabled={busy} onClick={() => void build()}>{t("workspace.onboarding.view.create")}</Button>}
          {step === 3 && <Button disabled={busy} onClick={() => void invite()}>{t("workspace.onboarding.invite.finish")}</Button>}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
