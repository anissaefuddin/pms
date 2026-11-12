import { Suspense, lazy } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

// Lazy on purpose: SVAR Gantt is large and must stay out of the main bundle.
const GanttSpike = lazy(() => import("@/pages/gantt-spike"));

export const Route = createFileRoute("/_app/spikes/gantt")({ component: GanttSpikeRoute });

function GanttSpikeRoute() {
  const { t } = useTranslation();
  return <Suspense fallback={<div className="p-8">{t("gantt.spike.loading")}</div>}><GanttSpike /></Suspense>;
}
