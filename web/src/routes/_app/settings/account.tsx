import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, type CurrentUser } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import type { components } from "@/api/schema";
import { FormError } from "@/components/auth/form-error";
import { SettingsSection } from "@/components/settings/settings-ui";

export const Route = createFileRoute("/_app/settings/account")({ component: AccountRoute });

type Account = components["schemas"]["Account"];

// Account overview: membership, seat capacity and the administrators, each in its own block.
function AccountRoute() {
  const { t } = useTranslation();
  const me = useProfile() as CurrentUser | undefined;
  const account = useQuery({ queryKey: ["account"], queryFn: () => api<Account>("/api/v1/account") });
  if (account.isError) return <FormError message={t("settings.account.loadError")} />;
  if (!account.data || !me) return <p role="status">{t("items.loading")}</p>;
  const { data } = account;
  const { seats } = data;
  const percent = seats.max > 0 ? Math.min(100, Math.round((seats.used / seats.max) * 100)) : 0;
  return <div className="max-w-3xl space-y-8">
    <section className="flex items-start gap-4">
      <span aria-hidden="true" className="grid size-14 shrink-0 place-items-center rounded-xl bg-primary text-lg font-semibold text-primary-foreground">{data.name.slice(0, 1).toUpperCase()}</span>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-lg font-semibold">{data.name}</p>
          <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium">{t(`settings.account.roles.${data.role}`, { defaultValue: data.role })}</span>
        </div>
        <p className="text-sm text-muted-foreground">{t("settings.account.companyWorkspace")}</p>
      </div>
    </section>

    <dl className="grid gap-4 border-y py-4 text-sm sm:grid-cols-2">
      <div><dt className="text-muted-foreground">{t("settings.account.joined")}</dt><dd className="font-medium">{formatDate(data.joined_at, me)}</dd></div>
      <div><dt className="text-muted-foreground">{t("settings.account.created")}</dt><dd className="font-medium">{formatDate(data.created_at, me)}</dd></div>
    </dl>

    <SettingsSection title={t("settings.account.capacity")}>
      <div className="flex items-baseline justify-between">
        <p className="text-2xl font-semibold">{seats.used} <span className="text-sm font-normal text-muted-foreground">/ {seats.max} {t("settings.account.seatsUsed")}</span></p>
        <p className="text-sm font-medium">{percent}%</p>
      </div>
      <div role="progressbar" aria-label={t("settings.account.seatsUsed")} aria-valuemin={0} aria-valuemax={seats.max} aria-valuenow={seats.used} className="h-2 overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full bg-primary" style={{ width: `${percent}%` }} />
      </div>
      <dl className="grid grid-cols-3 gap-4 text-sm">
        <div><dt className="text-muted-foreground">{t("settings.account.active")}</dt><dd className="text-lg font-semibold">{seats.used}</dd></div>
        <div><dt className="text-muted-foreground">{t("settings.account.invited")}</dt><dd className="text-lg font-semibold">{seats.pending}</dd></div>
        <div><dt className="text-muted-foreground">{t("settings.account.available")}</dt><dd className="text-lg font-semibold">{seats.available}</dd></div>
      </dl>
    </SettingsSection>

    <SettingsSection title={t("settings.account.admins")} description={t("settings.account.adminsCount", { count: data.administrators.length })}>
      <ul className="divide-y rounded-lg border">
        {data.administrators.map((admin) => <li key={admin.id} className="flex items-center gap-3 px-3 py-2.5">
          <span aria-hidden="true" className="grid size-8 shrink-0 place-items-center rounded-full bg-muted text-xs font-semibold">{initialsOf(admin.name)}</span>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">{admin.name}</p>
            <a className="truncate text-xs text-muted-foreground underline" href={`mailto:${admin.email}`}>{admin.email}</a>
          </div>
          <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium">{t("settings.account.adminBadge")}</span>
        </li>)}
      </ul>
    </SettingsSection>
  </div>;
}

function initialsOf(name: string) {
  return name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]).join("").toUpperCase();
}
