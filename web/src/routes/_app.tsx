import { useEffect } from "react";
import { Outlet, createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, type CurrentUser } from "@/lib/api";
import { Sidebar } from "@/components/sidebar";
import { Topbar } from "@/components/topbar";

export const Route = createFileRoute("/_app")({ component: AppShell });

function AppShell() {
  const { t } = useTranslation();
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<CurrentUser>("/api/v1/me") });
  useEffect(() => {
    if (me.isError) {
      const here = location.pathname + location.search;
      location.replace(here === "/" ? "/login" : `/login?next=${encodeURIComponent(here)}`);
    }
  }, [me.isError]);
  if (!me.data) return <div className="grid min-h-screen place-items-center" role="status">{t("workspace.home.loading")}</div>;
  return (
    <div className="flex min-h-screen">
      <Sidebar account={`${me.data.first_name} ${me.data.last_name}`.trim()} />
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar />
        <div className="min-w-0 flex-1"><Outlet /></div>
      </div>
    </div>
  );
}
