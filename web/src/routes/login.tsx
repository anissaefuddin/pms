import { useState, type FormEvent } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ApiError, api } from "@/lib/api";
import { safeNext } from "@/lib/next";
import { AuthCard } from "@/components/auth/auth-card";
import { FormError } from "@/components/auth/form-error";
import { TextField } from "@/components/auth/text-field";
import { Button } from "@/components/ui/button";

export const Route = createFileRoute("/login")({ component: Login });

function Login() {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const expired = new URLSearchParams(location.search).get("reason") === "expired";

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const data = new FormData(event.currentTarget);
    try {
      await api("/api/v1/auth/login", { method: "POST", body: JSON.stringify({ email: data.get("email"), password: data.get("password") }) });
      location.replace(safeNext(new URLSearchParams(location.search).get("next")));
    } catch (e) {
      const status = e instanceof ApiError ? e.status : 0;
      setError(status === 401 ? t("auth.login.invalid") : status === 429 ? (e instanceof ApiError && e.retryAfter ? t("auth.login.retryAfter", { count: e.retryAfter }) : t("auth.login.rateLimited")) : (e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthCard title={t("auth.login.title")}>
      {expired && <p role="status" className="mb-4 rounded border border-warning bg-warning-subtle p-3 text-sm">{t("auth.login.expired")}</p>}
      <form className="space-y-4" onSubmit={submit}>
        <TextField label={t("auth.fields.email")} name="email" type="email" autoComplete="email" required autoFocus />
        <TextField label={t("auth.fields.password")} name="password" type="password" autoComplete="current-password" required />
        <FormError message={error} />
        <Button type="submit" className="w-full" disabled={busy}>{busy ? t("auth.login.submitting") : t("auth.login.submit")}</Button>
        <Link to="/forgot-password" className="block text-center text-sm text-primary hover:underline">{t("auth.forgot.link")}</Link>
      </form>
    </AuthCard>
  );
}
