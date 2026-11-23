import { createFileRoute } from "@tanstack/react-router";
import { CustomFieldRegistry } from "@/components/custom-field-registry";
import { useProfile } from "@/lib/profile";
import { useTranslation } from "react-i18next";

export const Route = createFileRoute("/_app/settings/custom-fields")({ component: CustomFieldsSettings });

// Work configuration > Custom fields. Administrators and Space owners manage fields here; no Personal Space detour.
function CustomFieldsSettings() {
  const { t } = useTranslation();
  const admin = useProfile()?.user_type === "admin";
  if (!admin) return <p role="alert">{t("settings.adminOnly")}</p>;
  return <CustomFieldRegistry />;
}
