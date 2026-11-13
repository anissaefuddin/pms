import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, type CurrentUser } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import type { components } from "@/api/schema";
import { FormError } from "@/components/auth/form-error";

export const Route = createFileRoute("/_app/settings/account")({ component: AccountRoute });

type Account = components["schemas"]["Account"];

function AccountRoute() {
  const { t } = useTranslation();
  const me = useProfile();
  const account = useQuery({ queryKey: ["account"], queryFn: () => api<Account>("/api/v1/account") });
  if (account.isError) return <FormError message={t("settings.account.loadError")} />;
  if (!account.data) return <p role="status">{t("items.loading")}</p>;
  const { data } = account;
  const rows: [string, string][] = [
    [t("settings.account.name"), data.name],
    [t("settings.account.role"), data.role],
    [t("settings.account.joined"), formatDate(data.joined_at, me as CurrentUser | undefined)],
    [t("settings.account.created"), formatDate(data.created_at, me as CurrentUser | undefined)],
  ];
  return <section className="max-w-3xl space-y-4">
    <h2 className="text-lg font-semibold">{t("settings.account.title")}</h2>
    <dl className="grid gap-3 sm:grid-cols-[12rem_minmax(0,1fr)] sm:gap-2 text-sm">
      {rows.map(([label, value]) => <div key={label} className="sm:contents"><dt className="text-muted-foreground">{label}</dt><dd className="break-words">{value}</dd></div>)}
    </dl>
    <p className="text-sm">{t("settings.users.seats", { ...data.seats })}</p>
    <div>
      <h3 className="mb-1 font-medium">{t("settings.account.admins")}</h3>
      <ul className="space-y-1 break-words text-sm">{data.administrators.map((admin) =>
        <li key={admin.id}>{admin.name} · <a className="underline" href={`mailto:${admin.email}`}>{admin.email}</a></li>)}</ul>
    </div>
  </section>;
}
