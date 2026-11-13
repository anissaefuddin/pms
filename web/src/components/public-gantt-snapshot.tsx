/* eslint-disable i18next/no-literal-string */
import { useMemo } from "react";
import { Gantt, Willow, WillowDark, type ITask } from "@svar-ui/react-gantt";
import "@svar-ui/react-gantt/style.css";

export type PublicGantt = {
  project_id: string;
  project_title: string;
  items: Array<{
    id: string;
    title: string;
    parent_id?: string | null;
    start_date?: string | null;
    due_date?: string | null;
    progress_percent: number;
    assignees?: string[];
  }>;
};

const day = (value?: string | null) => value ? new Date(`${value}T00:00:00`) : undefined;

export function PublicGanttSnapshot({ snapshot }: { snapshot: PublicGantt }) {
  const dark = document.documentElement.classList.contains("dark");
  const Theme = dark ? WillowDark : Willow;
  const tasks = useMemo<ITask[]>(() => snapshot.items.map((item) => {
    const start = day(item.start_date ?? item.due_date);
    const end = day(item.due_date ?? item.start_date);
    if (end) end.setDate(end.getDate() + 1);
    return {
      id: item.id,
      text: item.title,
      parent: item.parent_id === snapshot.project_id ? 0 : item.parent_id ?? 0,
      open: true,
      progress: item.progress_percent,
      ...(start && end ? { start, end } : { unscheduled: true }),
      assignees: item.assignees?.join(", ") ?? "",
    };
  }), [snapshot]);
  return <main className="min-h-screen bg-background p-4 text-foreground sm:p-8">
    <header className="mb-4">
      <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Shared Gantt snapshot</p>
      <h1 className="text-2xl font-semibold">{snapshot.project_title}</h1>
    </header>
    <div className="h-[calc(100vh-8rem)] overflow-hidden rounded-lg border bg-card" aria-label="Read-only Gantt chart">
      <Theme><Gantt tasks={tasks} links={[]} readonly={true} columns={[
        { id: "text", header: "Task", width: 260 },
        ...(snapshot.items.some((item) => item.assignees?.length) ? [{ id: "assignees", header: "Assignees", width: 180 }] : []),
      ]} scales={[{ unit: "month", step: 1, format: "%F %Y" }, { unit: "day", step: 1, format: "%d" }]} /></Theme>
    </div>
  </main>;
}
