import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { useProfile } from "@/lib/profile";
import { ConnectorsTab } from "@/components/ConnectorsTab";

export const Route = createFileRoute("/_app/settings/apps")({ component: Apps });

// Minimal shell for S11.3.4. S11.3.1 owns the final Apps & Integrations page; keep ConnectorsTab as the Connectors tab there.
function Apps() {
  const { t } = useTranslation();
  const me = useProfile();
  if (me?.user_type !== "admin") return null;
  return <section className="space-y-5">
    <h2 className="text-lg font-semibold">{t("settings.apps.title")}</h2>
    <div role="tablist" aria-label={t("settings.apps.title")}>
      <button role="tab" aria-selected="true" className="border-b-2 px-3 py-1 text-sm font-medium">{t("settings.apps.connectors.title")}</button>
    </div>
    <div role="tabpanel"><ConnectorsTab /></div>
  </section>;
}
