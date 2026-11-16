import { Link, createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { settingsGroups } from "@/components/settings-sidebar";
import { useProfile } from "@/lib/profile";

export const Route = createFileRoute("/_app/settings/design-guide")({ component: DesignGuide });

// Read-only: lists the settings groups from the sidebar so the structure is visible in one place.
function DesignGuide() {
  const { t } = useTranslation();
  const groups = settingsGroups(useProfile(), t);
  return <section className="max-w-3xl space-y-6">
    <header className="space-y-1">
      <h2 className="text-lg font-semibold">{t("settings.designGuide.title")}</h2>
      <p className="text-sm text-muted-foreground">{t("settings.designGuide.description")}</p>
    </header>
    {groups.map((group) => <section key={group.title} className="space-y-2">
      <h3 className="font-medium">{group.title}</h3>
      <ul className="list-disc space-y-1 ps-5 text-sm">
        {group.items.map((item) => <li key={item.to + (item.hash ?? "")}>
          <Link to={item.to} params={item.params} hash={item.hash} className="underline">{item.label}</Link></li>)}
      </ul>
    </section>)}
  </section>;
}
