import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { FormManagePanel } from "@/components/form-manage-panel";

export const Route = createFileRoute("/_app/forms")({ component: ManageForms });

// Kept for existing links; the same list is the Settings > Request forms page.
function ManageForms() {
  const { t } = useTranslation();
  return <main className="space-y-5 p-8">
    <header className="border-b pb-5">
      <h1 className="workspace-title">{t("forms.manage.title")}</h1>
      <p className="text-sm text-muted-foreground">{t("forms.manage.description")}</p>
    </header>
    <FormManagePanel />
  </main>;
}
