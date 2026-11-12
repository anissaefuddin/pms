import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";

export const Route = createFileRoute("/_app/settings/users/types")({ component: UserTypes });

function UserTypes() {
  const { t } = useTranslation();
  const query = useQuery({ queryKey: ["user-types"], queryFn: () => api<{
    roles: Record<string, string[]>;
    user_types: Record<string, string[]>;
  }>("/api/v1/users/types") });
  return <section className="space-y-3">
    <h2 className="text-xl font-semibold">{t("settings.users.types")}</h2>
    {query.isPending && <p role="status">{t("items.loading")}</p>}
    {query.isError && <p role="alert">{t("items.loadError")}</p>}
    {query.data && <table className="w-full text-left text-sm">
      <thead><tr><th scope="col">{t("settings.users.type")}</th><th scope="col">{t("settings.users.capabilities")}</th></tr></thead>
      <tbody>{Object.entries(query.data.user_types).map(([type, capabilities]) => <tr key={type} className="border-t">
        <th scope="row" className="p-2">{type}</th><td className="p-2">{capabilities.join(", ")}</td>
      </tr>)}</tbody>
    </table>}
  </section>;
}
