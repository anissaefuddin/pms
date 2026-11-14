import { useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { ApiError, api, type CurrentUser } from "@/lib/api";
import { safeNext } from "@/lib/next";
import { AuthCard } from "@/components/auth/auth-card";
import { FormError } from "@/components/auth/form-error";
import { TextField } from "@/components/auth/text-field";
import { Button } from "@/components/ui/button";

export const Route = createFileRoute("/login")({ component: Login });

type Challenge = components["schemas"]["TwoFactorChallenge"];

function Login() {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  // Only the challenge survives the password step; the password stays in the (unmounted) form.
  const [challenge, setChallenge] = useState("");
  const expired = new URLSearchParams(location.search).get("reason") === "expired";
  const ssoError = new URLSearchParams(location.search).get("sso_error");
  // Its own cache key: the account page reads the same endpoint but keeps only the list.
  const providers = useQuery({ queryKey: ["login-options"], retry: false, staleTime: 60_000, queryFn: () => api<{ items: { kind: string; name: string }[]; otp_login: boolean }>("/api/v1/auth/sso/providers") });
  // Signing in with an emailed code: "email" asks for the address, "code" for the six digits that were sent.
  const [otp, setOtp] = useState<"" | "email" | "code">("");
  const [otpEmail, setOtpEmail] = useState("");

  function failure(e: unknown) {
    const status = e instanceof ApiError ? e.status : 0;
    if (status === 429) return e instanceof ApiError && e.retryAfter ? t("auth.login.retryAfter", { count: e.retryAfter }) : t("auth.login.rateLimited");
    return (e as Error).message;
  }
  const done = () => location.replace(safeNext(new URLSearchParams(location.search).get("next")));

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    setNotice("");
    const data = new FormData(event.currentTarget);
    try {
      const result = await api<CurrentUser | Challenge>("/api/v1/auth/login", { method: "POST", body: JSON.stringify({ email: data.get("email"), password: data.get("password") }) });
      if (result && "two_factor_required" in result) setChallenge(result.challenge);
      else done();
    } catch (e) {
      setError(e instanceof ApiError && e.status === 401 ? t("auth.login.invalid") : failure(e));
    } finally {
      setBusy(false);
    }
  }

  async function submitCode(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const code = String(new FormData(event.currentTarget).get("code") ?? "");
    try {
      await api("/api/v1/auth/login/2fa", { method: "POST", body: JSON.stringify({ challenge, code }) });
      done();
    } catch (e) {
      if (e instanceof ApiError && e.status === 401 && e.code === "challenge_expired") {
        setChallenge("");
        setNotice(t("auth.twoFactor.challengeExpired"));
      } else setError(e instanceof ApiError && e.status === 401 ? t("auth.twoFactor.invalid") : failure(e));
    } finally {
      setBusy(false);
    }
  }

  async function requestCode(event?: FormEvent<HTMLFormElement>, address = otpEmail) {
    event?.preventDefault();
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await api("/api/v1/auth/otp/request", { method: "POST", body: JSON.stringify({ email: address }) });
      setOtpEmail(address);
      setOtp("code");
      if (!event) setNotice(t("auth.otp.resent"));
    } catch (e) {
      setError(failure(e));
    } finally {
      setBusy(false);
    }
  }

  async function submitOtp(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    setNotice("");
    const code = String(new FormData(event.currentTarget).get("code") ?? "").trim();
    try {
      const result = await api<CurrentUser | Challenge>("/api/v1/auth/otp/verify", { method: "POST", body: JSON.stringify({ email: otpEmail, code }) });
      if (result && "two_factor_required" in result) { setOtp(""); setChallenge(result.challenge); }
      else done();
    } catch (e) {
      setError(e instanceof ApiError && e.status === 401 ? t("auth.otp.invalid") : failure(e));
    } finally {
      setBusy(false);
    }
  }

  function back() {
    setChallenge("");
    setError("");
  }

  if (challenge) {
    return (
      <AuthCard title={t("auth.twoFactor.title")}>
        <form className="space-y-4" onSubmit={submitCode}>
          <TextField label={t("auth.twoFactor.code")} hint={t("auth.twoFactor.codeHint")} name="code" type="text" autoComplete="one-time-code"
            autoCapitalize="characters" spellCheck={false} maxLength={32} required autoFocus />
          <FormError message={error} />
          <Button type="submit" className="w-full" disabled={busy}>{busy ? t("auth.twoFactor.verifying") : t("auth.twoFactor.submit")}</Button>
          <button type="button" className="block w-full text-center text-sm text-primary hover:underline" onClick={back}>{t("auth.twoFactor.otherAccount")}</button>
        </form>
      </AuthCard>
    );
  }

  if (otp === "email") {
    return (
      <AuthCard title={t("auth.otp.title")}>
        <form className="space-y-4" onSubmit={(e) => void requestCode(e, String(new FormData(e.currentTarget).get("email") ?? ""))}>
          <p className="text-sm text-muted-foreground">{t("auth.otp.intro")}</p>
          <TextField label={t("auth.fields.email")} name="email" type="email" autoComplete="email" defaultValue={otpEmail} required autoFocus />
          <FormError message={error} />
          <Button type="submit" className="w-full" disabled={busy}>{busy ? t("auth.otp.sending") : t("auth.otp.send")}</Button>
          <button type="button" className="block w-full text-center text-sm text-primary hover:underline" onClick={() => { setOtp(""); setError(""); }}>{t("auth.login.back")}</button>
        </form>
      </AuthCard>
    );
  }
  if (otp === "code") {
    return (
      <AuthCard title={t("auth.otp.codeTitle")}>
        <form className="space-y-4" onSubmit={submitOtp}>
          <p className="text-sm text-muted-foreground">{t("auth.otp.sentTo", { email: otpEmail })}</p>
          {notice && <p role="status" className="rounded border border-warning bg-warning-subtle p-3 text-sm">{notice}</p>}
          <TextField label={t("auth.otp.code")} hint={t("auth.otp.codeHint")} name="code" type="text" inputMode="numeric" pattern="[0-9]{6}" autoComplete="one-time-code"
            spellCheck={false} maxLength={6} required autoFocus />
          <FormError message={error} />
          <Button type="submit" className="w-full" disabled={busy}>{busy ? t("auth.otp.verifying") : t("auth.otp.submit")}</Button>
          <button type="button" className="block w-full text-center text-sm text-primary hover:underline" disabled={busy} onClick={() => void requestCode()}>{t("auth.otp.resend")}</button>
          <button type="button" className="block w-full text-center text-sm text-primary hover:underline" onClick={() => { setOtp("email"); setError(""); setNotice(""); }}>{t("auth.otp.otherEmail")}</button>
        </form>
      </AuthCard>
    );
  }

  return (
    <AuthCard title={t("auth.login.title")}>
      {expired && <p role="status" className="mb-4 rounded border border-warning bg-warning-subtle p-3 text-sm">{t("auth.login.expired")}</p>}
      {notice && <p role="status" className="mb-4 rounded border border-warning bg-warning-subtle p-3 text-sm">{notice}</p>}
      {ssoError && <p role="alert" className="mb-4 rounded border border-destructive p-3 text-sm">{t(`auth.sso.error.${ssoError}`, { defaultValue: t("auth.sso.error.failed") })}</p>}
      {!!providers.data?.items.length && <div className="mb-4 space-y-2">
        {providers.data.items.map((provider) => <a key={provider.kind} href={`/api/v1/auth/sso/${provider.kind}/start?next=${encodeURIComponent(safeNext(new URLSearchParams(location.search).get("next")))}`}
          className="block rounded-lg border border-input px-3 py-2 text-center text-sm font-medium hover:bg-muted">{t("auth.sso.signInWith", { name: provider.name })}</a>)}
        <p className="text-center text-xs text-muted-foreground">{t("auth.sso.or")}</p>
      </div>}
      <form className="space-y-4" onSubmit={submit}>
        <TextField label={t("auth.fields.email")} name="email" type="email" autoComplete="email" required autoFocus />
        <TextField label={t("auth.fields.password")} name="password" type="password" autoComplete="current-password" required />
        <FormError message={error} />
        <Button type="submit" className="w-full" disabled={busy}>{busy ? t("auth.login.submitting") : t("auth.login.submit")}</Button>
        <Link to="/forgot-password" className="block text-center text-sm text-primary hover:underline">{t("auth.forgot.link")}</Link>
        {providers.data?.otp_login && <button type="button" className="block w-full text-center text-sm text-primary hover:underline" onClick={() => { setOtp("email"); setError(""); }}>{t("auth.otp.link")}</button>}
      </form>
    </AuthCard>
  );
}
