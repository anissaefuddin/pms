import { useEffect, useMemo, useState, type FormEvent } from "react";
import qrcode from "qrcode-generator";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import type { components } from "@/api/schema";
import { ApiError, api } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import { FormError } from "@/components/auth/form-error";
import { TextField } from "@/components/auth/text-field";
import { Button, buttonVariants } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

type Status = components["schemas"]["TwoFactorStatus"];
type Setup = { secret: string; otpauth_uri: string };
const statusKey = ["me", "2fa"];

function message(t: TFunction, e: unknown, wrong: string) {
  if (!(e instanceof ApiError)) return (e as Error).message;
  if (e.status === 429) return e.retryAfter ? t("auth.login.retryAfter", { count: e.retryAfter }) : t("auth.login.rateLimited");
  if (e.status === 401 || e.status === 422) return wrong;
  return e.message;
}

export function TwoFactorSettings() {
  const { t } = useTranslation();
  const profile = useProfile();
  const client = useQueryClient();
  const status = useQuery({ queryKey: statusKey, queryFn: () => api<Status>("/api/v1/me/2fa") });
  const [setup, setSetup] = useState<Setup | null>(null);
  const [codes, setCodes] = useState<string[] | null>(null);
  const [justEnabled, setJustEnabled] = useState(false);
  const [dialog, setDialog] = useState<"renew" | "disable" | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function start() {
    setError("");
    try { setSetup(await api<Setup>("/api/v1/me/2fa/setup", { method: "POST" })); }
    catch (e) { setError(message(t, e, "")); }
  }

  async function enable(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const result = await api<{ backup_codes: string[] }>("/api/v1/me/2fa/enable", { method: "POST", body: JSON.stringify({ code: new FormData(event.currentTarget).get("code") }) });
      setSetup(null);
      setCodes(result.backup_codes);
      setJustEnabled(true);
      await client.invalidateQueries({ queryKey: statusKey });
    } catch (e) {
      setError(message(t, e, t("settings.twoFactor.wrongCode")));
    } finally {
      setBusy(false);
    }
  }

  const data = status.data;
  let body;
  if (status.isError) body = <FormError message={(status.error as Error).message} />;
  else if (!data) body = <p className="text-sm text-muted-foreground">{t("common.feedback.loading")}</p>;
  else if (codes) {
    body = (
      <BackupCodes codes={codes} enabled={justEnabled} onDone={() => { setCodes(null); setJustEnabled(false); }} />
    );
  } else if (!data.configured) body = <p className="text-sm text-muted-foreground">{t(profile?.user_type === "admin" ? "settings.twoFactor.notConfiguredAdmin" : "settings.twoFactor.notConfigured")}</p>;
  else if (setup) {
    body = (
      <div className="space-y-4">
        <p className="text-sm">{t("settings.twoFactor.scan")}</p>
        <QrCode value={setup.otpauth_uri} />
        <div className="space-y-1.5">
          <p className="text-sm">{t("settings.twoFactor.manual")}</p>
          <div className="flex items-center gap-2">
            <code className="rounded border bg-muted px-2 py-1 font-mono text-sm break-all" data-testid="two-factor-secret">{setup.secret}</code>
            <Button type="button" variant="outline" size="sm" onClick={() => copy(t, setup.secret, t("settings.twoFactor.secretCopied"))}>{t("settings.twoFactor.copySecret")}</Button>
          </div>
        </div>
        <form className="space-y-3" noValidate onSubmit={enable}>
          <TextField label={t("settings.twoFactor.confirmCode")} hint={t("settings.twoFactor.confirmHint")} name="code" type="text" inputMode="numeric" autoComplete="one-time-code" maxLength={16} required />
          <FormError message={error} />
          <div className="flex gap-2">
            <Button type="submit" disabled={busy}>{t("settings.twoFactor.enable")}</Button>
            <Button type="button" variant="outline" onClick={() => { setSetup(null); setError(""); }}>{t("settings.twoFactor.cancel")}</Button>
          </div>
        </form>
      </div>
    );
  } else if (data.enabled) {
    const low = data.backup_codes_remaining <= 3;
    body = (
      <div className="space-y-3">
        <p className="text-sm">{t("settings.twoFactor.onSince", { date: formatDate(data.enabled_at, profile) })}</p>
        <p className={low ? "rounded border border-warning bg-warning-subtle p-3 text-sm" : "text-sm"}>
          {t("settings.twoFactor.remaining", { count: data.backup_codes_remaining })}{low && ` ${t("settings.twoFactor.lowCodes")}`}
        </p>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={() => setDialog("renew")}>{t("settings.twoFactor.renew")}</Button>
          <Button variant="outline" onClick={() => setDialog("disable")}>{t("settings.twoFactor.disable")}</Button>
        </div>
        <ReauthDialog kind={dialog} onClose={() => setDialog(null)} onDone={async (result) => {
          setDialog(null);
          if (result) setCodes(result.backup_codes);
          else toast.success(t("settings.twoFactor.disabled"));
          await client.invalidateQueries({ queryKey: statusKey });
        }} />
      </div>
    );
  } else {
    body = (
      <div className="space-y-3">
        <p className="text-sm text-muted-foreground">{t("settings.twoFactor.off")}</p>
        <FormError message={error} />
        <Button variant="outline" onClick={start}>{t("settings.twoFactor.setUp")}</Button>
      </div>
    );
  }

  return (
    <section className="mt-10" aria-labelledby="two-factor-title">
      <h3 id="two-factor-title" className="text-base font-semibold">{t("settings.twoFactor.title")}</h3>
      <p className="mt-1 mb-4 text-sm text-muted-foreground">{t("settings.twoFactor.description")}</p>
      {body}
    </section>
  );
}

async function copy(t: TFunction, text: string, done: string) {
  try { await navigator.clipboard.writeText(text); toast.success(done); }
  catch { toast.error(t("settings.twoFactor.copyFailed")); }
}

function BackupCodes({ codes, enabled, onDone }: { codes: string[]; enabled: boolean; onDone: () => void }) {
  const { t } = useTranslation();
  const text = codes.join("\n");
  const [href, setHref] = useState("");
  useEffect(() => {
    const url = URL.createObjectURL(new Blob([text + "\n"], { type: "text/plain" }));
    setHref(url);
    return () => URL.revokeObjectURL(url);
  }, [text]);
  return (
    <div className="space-y-3">
      {enabled && <p role="status" className="text-sm">{t("settings.twoFactor.enabled")}</p>}
      <p className="text-sm">{t("settings.twoFactor.codesOnce")}</p>
      <ul aria-label={t("settings.twoFactor.codesLabel")} className="grid grid-cols-2 gap-1 rounded border bg-muted p-3 font-mono text-sm">
        {codes.map((code) => <li key={code}>{code}</li>)}
      </ul>
      <div className="flex flex-wrap gap-2">
        <Button variant="outline" onClick={() => copy(t, text, t("settings.twoFactor.codesCopied"))}>{t("settings.twoFactor.copyCodes")}</Button>
        <a className={buttonVariants({ variant: "outline" })} href={href} download="pms-backup-codes.txt">{t("settings.twoFactor.downloadCodes")}</a>
        <Button onClick={onDone}>{t("settings.twoFactor.savedCodes")}</Button>
      </div>
    </div>
  );
}

/** Draws the otpauth link as an SVG; the secret text next to it is the non-visual alternative. */
function QrCode({ value }: { value: string }) {
  const { t } = useTranslation();
  const { size, path } = useMemo(() => {
    const qr = qrcode(0, "M");
    qr.addData(value);
    qr.make();
    const count = qr.getModuleCount();
    let d = "";
    for (let row = 0; row < count; row++) for (let col = 0; col < count; col++) if (qr.isDark(row, col)) d += `M${col + 4} ${row + 4}h1v1h-1z`;
    return { size: count + 8, path: d };
  }, [value]);
  // Always dark on white (also in dark mode) so phone cameras can read it.
  return (
    <svg role="img" aria-label={t("settings.twoFactor.qrLabel")} viewBox={`0 0 ${size} ${size}`} className="size-48 rounded" shapeRendering="crispEdges">
      <rect width={size} height={size} fill="#fff" />
      <path d={path} fill="#000" />
    </svg>
  );
}

function ReauthDialog({ kind, onClose, onDone }: { kind: "renew" | "disable" | null; onClose: () => void; onDone: (result?: { backup_codes: string[] }) => void }) {
  const { t } = useTranslation();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const renew = kind === "renew";
  const close = () => { setError(""); onClose(); };

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    setBusy(true);
    setError("");
    try {
      const result = await api<{ backup_codes: string[] } | undefined>(renew ? "/api/v1/me/2fa/backup-codes" : "/api/v1/me/2fa/disable", {
        method: "POST", body: JSON.stringify({ password: data.get("password"), code: data.get("code") }),
      });
      setError("");
      onDone(result);
    } catch (e) {
      setError(message(t, e, t("settings.twoFactor.wrongReauth")));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={!!kind} onOpenChange={(open) => { if (!open) close(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{renew ? t("settings.twoFactor.renew") : t("settings.twoFactor.disable")}</DialogTitle>
          <DialogDescription>{renew ? t("settings.twoFactor.renewDescription") : t("settings.twoFactor.disableDescription")}</DialogDescription>
        </DialogHeader>
        <form className="space-y-3" noValidate onSubmit={submit}>
          <TextField label={t("auth.fields.currentPassword")} name="password" type="password" autoComplete="current-password" required />
          <TextField label={t("auth.twoFactor.code")} hint={t("auth.twoFactor.codeHint")} name="code" type="text" autoComplete="one-time-code" spellCheck={false} maxLength={32} required />
          <FormError message={error} />
          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>{t("settings.twoFactor.cancel")}</Button>
            <Button type="submit" variant={renew ? "default" : "destructive"} disabled={busy}>{renew ? t("settings.twoFactor.renewSubmit") : t("settings.twoFactor.disableSubmit")}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
