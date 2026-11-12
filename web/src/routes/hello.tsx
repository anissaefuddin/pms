import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { checkHealth } from "@/lib/api";
import { AuthCard } from "@/components/auth/auth-card";

export const Route = createFileRoute("/hello")({ component: Hello });

function Hello() {
  const { t } = useTranslation();
  const health = useQuery({ queryKey: ["healthz"], queryFn: checkHealth });
  return (
    <AuthCard title={t("common.health.title")}>
      <p role="status">{health.isPending ? t("common.health.checking") : health.isError ? t("common.health.unavailable") : t("common.health.running")}</p>
    </AuthCard>
  );
}
