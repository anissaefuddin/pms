import { useState, type FormEvent } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { AuthCard } from "@/components/auth/auth-card";
import { FormError } from "@/components/auth/form-error";
import { TextField } from "@/components/auth/text-field";
import { Button } from "@/components/ui/button";

export const Route = createFileRoute("/forgot-password")({ component: ForgotPassword });

function ForgotPassword() {
  const { t } = useTranslation();
  const [done, setDone] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const data = new FormData(event.currentTarget);
    try {
      await api("/api/v1/auth/password/forgot", { method: "POST", body: JSON.stringify({ email: data.get("email") }) });
      setDone(true);
    } catch (e) {
      setError((e as Error).message);
    }
  }

  return (
    <AuthCard title={t("auth.forgot.title")}>
      {done ? (
        <p className="text-sm text-muted-foreground" role="status">{t("auth.forgot.confirmation")}</p>
      ) : (
        <form className="space-y-4" onSubmit={submit}>
          <TextField label={t("auth.fields.email")} name="email" type="email" autoComplete="email" required autoFocus />
          <FormError message={error} />
          <Button type="submit" className="w-full">{t("auth.forgot.submit")}</Button>
        </form>
      )}
      <Link to="/login" className="mt-5 block text-center text-sm text-primary hover:underline">{t("auth.login.back")}</Link>
    </AuthCard>
  );
}
