import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { CurrentUser } from "@/lib/api";
import { useSpaces } from "@/lib/space-queries";

export const Route = createFileRoute("/_app/")({ component: Workspace });

function Workspace() {
  const { t } = useTranslation();
  const me = useQueryClient().getQueryData<CurrentUser>(["me"]);
  const spaces = useSpaces();
  const recent = me && localStorage.getItem(`pms:last-space:${me.id}`);
  const destination = spaces.data?.find((space) => space.id === recent)
    ?? spaces.data?.find((space) => space.id === me?.personal_space_id);
  if (destination) return <Navigate to="/spaces/$spaceId" params={{ spaceId: destination.id }} replace />;
  return (
    <main className="p-8">
      <div className="rounded-xl border border-dashed bg-card p-12 text-center">
        <p className="text-sm font-medium text-primary">{t("workspace.home.eyebrow")}</p>
        <h1 className="mt-2 text-3xl font-semibold">{t("workspace.home.title")}</h1>
        <p className="mx-auto mt-3 max-w-lg text-muted-foreground">{t("workspace.home.description")}</p>
      </div>
    </main>
  );
}
