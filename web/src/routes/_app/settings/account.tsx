import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
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
    {account.logo_light_url ? <img src={account.logo_light_url} alt="" className="size-14 rounded-xl object-contain" />
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
  return <div role="tabpanel" className="space-y-6"><WorkspacePreview account={account} /><WorkspaceForm account={account} fields="branding" /></div>;
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
    </SettingsSection> : <SettingsSection title={t("settings.account.branding")} description={t("settings.account.brandingHint")}>
      <div className="grid gap-4 sm:grid-cols-2">
        <label className="block space-y-1 text-sm"><span>{t("settings.account.icon")}</span><Input name="icon" defaultValue={account.icon} maxLength={8} placeholder="L" /></label>
        <label className="block space-y-1 text-sm"><span>{t("settings.account.brandColor")}</span><Input name="brand_color" defaultValue={account.brand_color} pattern="^#[0-9A-Fa-f]{6}$" placeholder={t("settings.account.brandColorPlaceholder")} /></label>
        <label className="block space-y-1 text-sm sm:col-span-2"><span>{t("settings.account.logoLight")}</span><Input type="url" name="logo_light_url" defaultValue={account.logo_light_url} placeholder={t("settings.account.logoPlaceholder")} /></label>
        <label className="block space-y-1 text-sm sm:col-span-2"><span>{t("settings.account.logoDark")}</span><Input type="url" name="logo_dark_url" defaultValue={account.logo_dark_url} placeholder={t("settings.account.logoPlaceholder")} /></label>
      </div>
    </SettingsSection>}
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
