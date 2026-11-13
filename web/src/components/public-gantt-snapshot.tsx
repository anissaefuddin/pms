/* eslint-disable i18next/no-literal-string */
import { useEffect, useMemo, useRef } from "react";
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
    progress_percent: number | null;
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
      progress: item.progress_percent ?? 0,
      ...(start && end ? { start, end } : { unscheduled: true }),
      assignees: item.assignees?.join(", ") ?? "",
    };
  }), [snapshot]);
  const region = useRef<HTMLDivElement>(null);
  // Same grid semantics fix as the editable chart: SVAR reports row indices and aria-expanded that axe rejects.
  useEffect(() => {
    const node = region.current;
    if (!node) return;
    const parents = new Set(snapshot.items.map((item) => item.parent_id).filter(Boolean) as string[]);
    const fix = () => {
      for (const grid of node.querySelectorAll(".wx-table-box")) {
        if (grid.getAttribute("role") !== "treegrid") grid.setAttribute("role", "treegrid");
        grid.removeAttribute("aria-rowcount");
        for (const row of grid.querySelectorAll("[role=row][data-id]")) {
          row.removeAttribute("aria-rowindex");
          if (!parents.has(row.getAttribute("data-id")?.replace(/^:/, "") ?? "")) row.removeAttribute("aria-expanded");
        }
      }
    };
    fix();
    const observer = new MutationObserver(fix);
    observer.observe(node, { subtree: true, childList: true, attributes: true, attributeFilter: ["data-id", "aria-rowindex", "aria-rowcount", "aria-expanded"] });
    return () => observer.disconnect();
  }, [snapshot]);
  return <main className="min-h-screen bg-background p-4 text-foreground sm:p-8">
    <header className="mb-4">
      <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Shared Gantt snapshot</p>
      <h1 className="text-2xl font-semibold">{snapshot.project_title}</h1>
    </header>
    <div ref={region} className="gantt-chart h-[calc(100vh-8rem)] overflow-hidden rounded-lg border bg-card [&_.wx-theme]:h-full" aria-label="Read-only Gantt chart">
      <Theme><Gantt tasks={tasks} links={[]} readonly={true} columns={[
        { id: "text", header: "Task", width: 260 },
        ...(snapshot.items.some((item) => item.assignees?.length) ? [{ id: "assignees", header: "Assignees", width: 180 }] : []),
      ]} scales={[{ unit: "month", step: 1, format: "%F %Y" }, { unit: "day", step: 1, format: "%d" }]} /></Theme>
    </div>
  </main>;
}
