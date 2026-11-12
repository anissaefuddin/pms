import { useState, type FormEvent } from "react";
import { Link, createFileRoute, useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { ApiError, api } from "@/lib/api";
import { AuthCard } from "@/components/auth/auth-card";
import { FormError } from "@/components/auth/form-error";
import { PasswordStrength } from "@/components/auth/password-strength";
import { TextField } from "@/components/auth/text-field";
import { Button } from "@/components/ui/button";

export const Route = createFileRoute("/reset-password")({ component: ResetPassword });

function ResetPassword() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const token = new URLSearchParams(location.search).get("token") ?? "";
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [invalid, setInvalid] = useState(!token);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    if (password.length < 8) return setError(t("auth.password.tooShort"));
    if (password !== data.get("repeat")) return setError(t("auth.reset.passwordMismatch"));
    setError("");
    try {
      await api("/api/v1/auth/password/reset", { method: "POST", body: JSON.stringify({ token, password }) });
      toast.success(t("auth.reset.success"));
      navigate({ to: "/login" });
    } catch (e) {
      if (e instanceof ApiError && e.status === 410) setInvalid(true);
      else setError((e as Error).message);
    }
  }

  return (
    <AuthCard title={t("auth.reset.title")}>
      {invalid ? (
        <div className="space-y-4">
          <p className="text-sm text-destructive" role="alert">{t("auth.reset.invalidToken")}</p>
          <Link to="/forgot-password" className="block text-center text-sm text-primary hover:underline">{t("auth.reset.requestNew")}</Link>
        </div>
      ) : (
        <form className="space-y-4" noValidate onSubmit={submit}>
          <TextField label={t("auth.fields.newPassword")} hint={t("auth.fields.passwordRule")} name="password" type="password" autoComplete="new-password" required autoFocus value={password} onChange={(e) => setPassword(e.target.value)} />
          <PasswordStrength value={password} />
          <TextField label={t("auth.fields.repeatPassword")} name="repeat" type="password" autoComplete="new-password" required />
          <FormError message={error} />
          <Button type="submit" className="w-full">{t("auth.reset.submit")}</Button>
        </form>
      )}
    </AuthCard>
  );
}
