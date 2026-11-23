import { useState, type FormEvent } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { Mail, KeyRound, ShieldCheck } from "lucide-react";
import { api } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { FormError } from "@/components/auth/form-error";
import { TextField } from "@/components/auth/text-field";
import { Button } from "@/components/ui/button";
import { SsoIdentities } from "@/components/sso-identities";
import { TwoFactorSettings } from "@/components/two-factor-settings";
import { Dialog, DialogContent, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { SettingsRow, SettingsSection } from "@/components/settings/settings-ui";
import { AuthorizedApps } from "@/components/AuthorizedApps";

export const Route = createFileRoute("/_app/settings/security")({ component: Security });

// Sign-in and security as rows: each item states what it is and offers the action on the right.
function Security() {
  const { t } = useTranslation();
  const me = useProfile();
  const [open, setOpen] = useState(false);
  return (
    <div className="space-y-8">
      <SettingsSection title={t("settings.security.authentication")} description={t("settings.security.description")}>
        <div>
          <SettingsRow icon={<Mail />} label={t("settings.security.email")} hint={t("settings.security.emailHint")}>
            <span className="text-sm text-muted-foreground">{me?.email}</span>
          </SettingsRow>
          <SettingsRow icon={<KeyRound />} label={t("settings.security.password")} hint={t("settings.security.passwordHint")}>
            <Dialog open={open} onOpenChange={setOpen}>
              <DialogTrigger render={<Button variant="outline" />}>{t("settings.security.changePassword")}</DialogTrigger>
              <DialogContent>
                <DialogTitle className="text-lg font-semibold">{t("settings.security.changePassword")}</DialogTitle>
                <ChangePassword onDone={() => setOpen(false)} />
              </DialogContent>
            </Dialog>
          </SettingsRow>
          <SettingsRow icon={<ShieldCheck />} label={t("settings.security.twoFactor")} hint={t("settings.security.twoFactorHint")}>
            <a href="#two-factor" className="text-sm underline">{t("settings.security.manage")}</a>
          </SettingsRow>
        </div>
      </SettingsSection>
      <div id="two-factor" className="scroll-mt-4">
        <TwoFactorSettings />
      </div>
      <SsoIdentities />
      <AuthorizedApps />
    </div>
  );
}

function ChangePassword({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const next = String(data.get("password") ?? "");
    if (next.length < 8) return setError(t("auth.password.tooShort"));
    if (next !== data.get("repeat")) return setError(t("auth.reset.passwordMismatch"));
    setError("");
    try {
      await api("/api/v1/auth/password/change", { method: "POST", body: JSON.stringify({ current_password: data.get("currentPassword"), new_password: next }) });
      toast.success(t("settings.security.passwordChanged"));
      onDone();
    } catch (e) {
      setError((e as Error).message);
    }
  }

  return (
    <form className="space-y-3" noValidate onSubmit={submit}>
      <TextField label={t("auth.fields.currentPassword")} name="currentPassword" type="password" autoComplete="current-password" required />
      <TextField label={t("auth.fields.newPassword")} hint={t("auth.fields.passwordRule")} name="password" type="password" autoComplete="new-password" required />
      <TextField label={t("auth.fields.repeatPassword")} name="repeat" type="password" autoComplete="new-password" required />
      <FormError message={error} />
      <Button type="submit">{t("settings.security.changePassword")}</Button>
    </form>
  );
}
