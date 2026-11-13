import { useState, type FormEvent } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { FormError } from "@/components/auth/form-error";
import { TextField } from "@/components/auth/text-field";
import { Button } from "@/components/ui/button";
import { TwoFactorSettings } from "@/components/two-factor-settings";
import { Dialog, DialogContent, DialogTitle, DialogTrigger } from "@/components/ui/dialog";

export const Route = createFileRoute("/_app/settings/security")({ component: Security });

function Security() {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  return (
    <section className="max-w-xl">
      <h2 className="text-lg font-semibold">{t("settings.security.title")}</h2>
      <p className="mt-1 text-sm text-muted-foreground">{t("settings.security.description")}</p>
      <div className="mt-6">
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button variant="outline" />}>{t("settings.security.changePassword")}</DialogTrigger>
          <DialogContent>
            <DialogTitle className="text-lg font-semibold">{t("settings.security.changePassword")}</DialogTitle>
            <ChangePassword onDone={() => setOpen(false)} />
          </DialogContent>
        </Dialog>
      </div>
      <TwoFactorSettings />
    </section>
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
