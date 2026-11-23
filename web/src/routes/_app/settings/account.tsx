import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef, useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ImageUp, Trash2 } from "lucide-react";
import { api, type CurrentUser } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import type { components } from "@/api/schema";
import { FormError } from "@/components/auth/form-error";
import { SettingsSection } from "@/components/settings/settings-ui";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/settings/account")({ component: AccountRoute });

type Account = components["schemas"]["Account"];
type Tab = "general" | "branding" | "members";
const tabs: Tab[] = ["general", "branding", "members"];
const imageTypes = ["image/png", "image/jpeg", "image/webp"];
const brandColors = ["#0F766E", "#2563EB", "#7C3AED", "#EA580C"];

function AccountRoute() {
  const { t } = useTranslation();
  const me = useProfile() as CurrentUser | undefined;
  const [tab, setTab] = useState<Tab>("general");
  const account = useQuery({ queryKey: ["account"], queryFn: () => api<Account>("/api/v1/account") });
  if (account.isError) return <FormError message={t("settings.account.loadError")} />;
  if (!account.data || !me) return <p role="status">{t("items.loading")}</p>;
  return <div className="max-w-4xl space-y-6">
    <p className="text-sm text-muted-foreground">{t("settings.account.description")}</p>
    <div role="tablist" aria-label={t("settings.account.tabsLabel")} className="grid grid-cols-3 rounded-lg bg-muted p-1">
      {tabs.map((value) => <button key={value} role="tab" aria-selected={tab === value}
        onClick={() => setTab(value)} className={`rounded-md px-3 py-2 text-sm font-medium ${tab === value ? "bg-background shadow-sm" : "text-muted-foreground hover:text-foreground"}`}>
        {t(`settings.account.tabs.${value}`)}
      </button>)}
    </div>
    {tab === "general" && <GeneralTab account={account.data} me={me} />}
    {tab === "branding" && <BrandingTab account={account.data} />}
    {tab === "members" && <MembersTab account={account.data} />}
  </div>;
}

function WorkspacePreview({ account }: { account: Account }) {
  const { t } = useTranslation();
  const mark = account.icon || account.name.slice(0, 1).toUpperCase();
  return <section className="flex items-center gap-4 rounded-lg border p-4">
    {account.logo_light_version > 0 ? <img src={`/api/v1/account/branding/logo_light?v=${account.logo_light_version}`} alt="" className="size-14 rounded-xl object-contain" />
      : <span aria-hidden="true" style={account.brand_color ? { backgroundColor: account.brand_color } : undefined}
        className="grid size-14 shrink-0 place-items-center rounded-xl bg-primary text-lg font-semibold text-primary-foreground">{mark}</span>}
    <div className="min-w-0"><p className="truncate text-lg font-semibold">{account.name}</p>
      <p className="text-sm text-muted-foreground">{account.description || t("settings.account.companyWorkspace")}</p></div>
  </section>;
}

function GeneralTab({ account, me }: { account: Account; me: CurrentUser }) {
  const { t } = useTranslation();
  return <div role="tabpanel" className="space-y-6">
    <WorkspacePreview account={account} />
    <WorkspaceForm account={account} fields="general" />
    <dl className="border-t pt-4 text-sm"><dt className="text-muted-foreground">{t("settings.account.created")}</dt><dd className="font-medium">{formatDate(account.created_at, me)}</dd></dl>
  </div>;
}

function BrandingTab({ account }: { account: Account }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [color, setColor] = useState(account.brand_color || brandColors[0]);
  const [busy, setBusy] = useState("");
  async function updateAsset(kind: string, file?: File) {
    if (!file) return;
    if (!imageTypes.includes(file.type)) return void toast.error(t("settings.account.imageType"));
    if (file.size > 2 << 20) return void toast.error(t("settings.account.imageSize"));
    setBusy(kind);
    try {
      const updated = await api<Account>(`/api/v1/account/branding/${kind}`, { method: "PUT", headers: { "Content-Type": file.type }, body: file });
      client.setQueryData(["account"], updated); toast.success(t("settings.account.saved"));
    } catch (err) { toast.error((err as Error).message); } finally { setBusy(""); }
  }
  async function removeAsset(kind: string) {
    setBusy(kind);
    try {
      const updated = await api<Account>(`/api/v1/account/branding/${kind}`, { method: "DELETE" });
      client.setQueryData(["account"], updated); toast.success(t("settings.account.saved"));
    } catch (err) { toast.error((err as Error).message); } finally { setBusy(""); }
  }
  async function saveColor(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy("color");
    try {
      const updated = await api<Account>("/api/v1/account", { method: "PATCH", body: JSON.stringify({ brand_color: color }) });
      client.setQueryData(["account"], updated); toast.success(t("settings.account.saved"));
    } catch (err) { toast.error((err as Error).message); } finally { setBusy(""); }
  }
  return <div role="tabpanel" className="space-y-6">
    <SettingsSection title={t("settings.account.branding")} description={t("settings.account.brandingHint")}>
      <div className="grid gap-5 md:grid-cols-2">
        <AssetUpload title={t("settings.account.workspaceIcon")} hint={t("settings.account.iconUploadHint")} kind="icon" version={account.icon_image_version}
          previewClass="size-20 rounded-xl" busy={busy === "icon"} onUpload={updateAsset} onRemove={removeAsset} fallback={<span style={{ backgroundColor: color }} className="grid size-20 place-items-center rounded-xl text-xl font-semibold text-white">{account.name.slice(0, 1).toUpperCase()}</span>} />
        <AssetUpload title={t("settings.account.workspaceLogoLight")} hint={t("settings.account.logoLightHint")} kind="logo_light" version={account.logo_light_version}
          previewClass="h-20 max-w-full object-contain" busy={busy === "logo_light"} onUpload={updateAsset} onRemove={removeAsset} fallback={<span className="text-sm font-semibold">{account.name}</span>} />
        <AssetUpload title={t("settings.account.workspaceLogoDark")} hint={t("settings.account.logoDarkHint")} kind="logo_dark" version={account.logo_dark_version}
          previewClass="h-20 max-w-full object-contain" previewDark busy={busy === "logo_dark"} onUpload={updateAsset} onRemove={removeAsset} fallback={<span className="text-sm font-semibold text-white">{account.name}</span>} />
      </div>
    </SettingsSection>
    <form onSubmit={saveColor} className="space-y-4 border-t pt-6">
      <div><h2 className="text-base font-semibold">{t("settings.account.brandColor")}</h2><p className="text-sm text-muted-foreground">{t("settings.account.brandColorHint")}</p></div>
      <div className="flex flex-wrap items-center gap-3">
        {brandColors.map((preset) => <button key={preset} type="button" aria-label={preset} onClick={() => setColor(preset)} style={{ backgroundColor: preset }} className={`size-9 rounded-lg ${color.toLowerCase() === preset.toLowerCase() ? "ring-2 ring-ring ring-offset-2" : ""}`} />)}
        <label className="relative grid size-9 cursor-pointer place-items-center overflow-hidden rounded-lg border" title={t("settings.account.chooseColor")}>
          <input type="color" value={color} onChange={(event) => setColor(event.target.value)} className="absolute inset-[-8px] size-14 cursor-pointer border-0" />
          <span className="sr-only">{t("settings.account.chooseColor")}</span>
        </label>
        <Input value={color} onChange={(event) => setColor(event.target.value)} pattern="^#[0-9A-Fa-f]{6}$" aria-label={t("settings.account.brandColor")} className="w-28 font-mono uppercase" />
        <Button disabled={busy === "color"}>{t("settings.account.saveColor")}</Button>
      </div>
    </form>
  </div>;
}

function AssetUpload({ title, hint, kind, version, previewClass, previewDark = false, busy, onUpload, onRemove, fallback }: {
  title: string; hint: string; kind: string; version: number; previewClass: string; previewDark?: boolean; busy: boolean;
  onUpload: (kind: string, file?: File) => void; onRemove: (kind: string) => void; fallback: ReactNode;
}) {
  const { t } = useTranslation();
  const input = useRef<HTMLInputElement>(null);
  return <section className="space-y-3">
    <div><h3 className="text-sm font-medium">{title}</h3><p className="text-xs text-muted-foreground">{hint}</p></div>
    <div className={`flex min-h-28 items-center justify-center rounded-lg border p-4 ${previewDark ? "bg-slate-950" : "bg-muted/20"}`}>
      {version > 0 ? <img src={`/api/v1/account/branding/${kind}?v=${version}`} alt="" className={previewClass} /> : fallback}
    </div>
    <input ref={input} type="file" accept={imageTypes.join(",")} className="sr-only" onChange={(event) => { void onUpload(kind, event.target.files?.[0]); event.target.value = ""; }} />
    <div className="flex gap-2"><Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => input.current?.click()}><ImageUp aria-hidden="true" className="size-4" />{t("settings.account.upload")}</Button>
      {version > 0 && <Button type="button" variant="ghost" size="sm" disabled={busy} onClick={() => void onRemove(kind)}><Trash2 aria-hidden="true" className="size-4" />{t("settings.account.remove")}</Button>}</div>
  </section>;
}

function WorkspaceForm({ account, fields }: { account: Account; fields: "general" | "branding" }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true); setError("");
    const values = Object.fromEntries(new FormData(event.currentTarget));
    try {
      const updated = await api<Account>("/api/v1/account", { method: "PATCH", body: JSON.stringify(values) });
      client.setQueryData(["account"], updated);
      client.setQueryData<CurrentUser>(["me"], (current) => current ? { ...current, account_name: updated.name } : current);
      toast.success(t("settings.account.saved"));
    } catch (err) { setError((err as Error).message); } finally { setBusy(false); }
  }
  return <form onSubmit={save} className="space-y-4">
    {fields === "general" ? <SettingsSection title={t("settings.account.identity")} description={t("settings.account.identityHint")}>
      <label className="block space-y-1 text-sm"><span>{t("settings.account.name")}</span><Input name="name" defaultValue={account.name} maxLength={120} required /></label>
      <label className="block space-y-1 text-sm"><span>{t("settings.account.workspaceDescription")}</span>
        <textarea name="description" defaultValue={account.description} maxLength={500} rows={3} className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring" /></label>
    </SettingsSection> : null}
    <FormError message={error} /><div className="flex justify-end"><Button disabled={busy}>{t("settings.account.save")}</Button></div>
  </form>;
}

function MembersTab({ account }: { account: Account }) {
  const { t } = useTranslation();
  const { seats } = account;
  const percent = seats.max > 0 ? Math.min(100, Math.round((seats.used / seats.max) * 100)) : 0;
  return <div role="tabpanel" className="space-y-6">
    <SettingsSection title={t("settings.account.capacity")} description={t("settings.account.membersHint")}>
      <div className="flex items-baseline justify-between"><p className="text-2xl font-semibold">{seats.used} <span className="text-sm font-normal text-muted-foreground">/ {seats.max} {t("settings.account.seatsUsed")}</span></p><p className="text-sm font-medium">{percent}%</p></div>
      <div role="progressbar" aria-label={t("settings.account.seatsUsed")} aria-valuemin={0} aria-valuemax={seats.max} aria-valuenow={seats.used} className="h-2 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-primary" style={{ width: `${percent}%` }} /></div>
      <dl className="grid grid-cols-3 gap-4 text-sm"><Stat label={t("settings.account.active")} value={seats.used} /><Stat label={t("settings.account.invited")} value={seats.pending} /><Stat label={t("settings.account.available")} value={seats.available} /></dl>
    </SettingsSection>
    <SettingsSection title={t("settings.account.admins")} description={t("settings.account.adminsCount", { count: account.administrators.length })}>
      <ul className="divide-y rounded-lg border">{account.administrators.map((admin) => <li key={admin.id} className="flex items-center gap-3 px-3 py-2.5"><span aria-hidden="true" className="grid size-8 shrink-0 place-items-center rounded-full bg-muted text-xs font-semibold">{initialsOf(admin.name)}</span><div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{admin.name}</p><a className="truncate text-xs text-muted-foreground underline" href={`mailto:${admin.email}`}>{admin.email}</a></div><span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium">{t("settings.account.adminBadge")}</span></li>)}</ul>
      <Link to="/settings/users" className="inline-flex h-9 items-center justify-center rounded-md border bg-background px-4 text-sm font-medium hover:bg-accent">{t("settings.account.manageUsers")}</Link>
    </SettingsSection>
  </div>;
}

function Stat({ label, value }: { label: string; value: number }) { return <div><dt className="text-muted-foreground">{label}</dt><dd className="text-lg font-semibold">{value}</dd></div>; }
function initialsOf(name: string) { return name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]).join("").toUpperCase(); }
