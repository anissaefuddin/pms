import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { PermissionMatrix } from "@/components/permission-matrix";

export const Route = createFileRoute("/_app/settings/users/types")({ component: UserTypes });

function UserTypes() {
  const { t } = useTranslation();
  const query = useQuery({ queryKey: ["user-types"], queryFn: () => api<{
    roles: Record<string, string[]>;
    user_types: Record<string, string[]>;
  }>("/api/v1/users/types") });
  const concepts = ["userType", "customRole", "group", "jobRole"] as const;
  return <section className="space-y-6">
    <section aria-labelledby="concepts-title" className="space-y-3">
      <h3 id="concepts-title" className="text-base font-semibold">{t("settings.users.concepts.title")}</h3>
      <dl className="grid gap-3 sm:grid-cols-2">
        {concepts.map((key) => <div key={key} className="rounded-lg border p-3">
          <dt className="text-sm font-medium">{t(`settings.users.concepts.${key}`)}</dt>
          <dd className="mt-1 text-sm text-muted-foreground">{t(`settings.users.concepts.${key}Desc`)}</dd>
        </div>)}
      </dl>
    </section>
    <h3 className="text-base font-semibold">{t("settings.users.types")}</h3>
    {query.isPending && <p role="status">{t("items.loading")}</p>}
    {query.isError && <p role="alert">{t("items.loadError")}</p>}
    {query.data && (() => {
      const types = Object.entries(query.data.user_types);
      const capabilities = [...new Set(types.flatMap(([, caps]) => caps))].sort();
      return <PermissionMatrix caption={t("settings.users.types")}
        columns={types.map(([type]) => ({ id: type, label: type }))}
        rows={capabilities.map((cap) => ({ id: cap, label: t(`roles.capability.${cap}`, { defaultValue: cap }) }))}
        allowed={(type, cap) => (query.data.user_types[type] ?? []).includes(cap)} />;
    })()}
  </section>;
}
