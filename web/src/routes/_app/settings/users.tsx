import { createFileRoute, Outlet } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { useProfile } from "@/lib/profile";

export const Route = createFileRoute("/_app/settings/users")({ component: AdminUsers, notFoundComponent: AdminNotFound });

function AdminUsers() {
  const me = useProfile();
  if (!me?.capabilities.manage_users) return <AdminNotFound />;
  return <Outlet />;
}

function AdminNotFound() {
  const { t } = useTranslation();
  return <h2 className="text-xl font-semibold">{t("settings.users.notFound")}</h2>;
}
