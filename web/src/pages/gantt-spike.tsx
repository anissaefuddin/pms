import { Gantt, Willow } from "@svar-ui/react-gantt";
import "@svar-ui/react-gantt/style.css";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";

export default function GanttSpike() {
  const { t } = useTranslation();
  const tasks = Array.from({ length: 200 }, (_, index) => ({
    id: index + 1,
    text: t("gantt.spike.task", { number: index + 1 }),
    start: new Date(2026, 10, 23 + (index % 10) * 4),
    duration: 2,
    type: "task",
  }));
  const links = tasks
    .filter(({ id }) => id % 10 !== 0)
    .map(({ id }) => ({ id, source: id, target: id + 1, type: "e2s" as const }));
  return (
    <main className="flex h-screen flex-col gap-4 p-6">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wider text-primary">{t("gantt.spike.eyebrow")}</p>
          <h1 className="text-xl font-semibold">{t("gantt.spike.title")}</h1>
        </div>
        <Link to="/settings" className="text-sm text-primary">{t("gantt.spike.back")}</Link>
      </div>
      <div className="min-h-0 flex-1 overflow-hidden rounded-lg border [&_.wx-theme]:h-full">
        <Willow><Gantt tasks={tasks} links={links} /></Willow>
      </div>
    </main>
  );
}
