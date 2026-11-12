import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { useQueries } from "@tanstack/react-query";
import { Gantt, Willow, WillowDark, type IApi, type ITask } from "@svar-ui/react-gantt";
import "@svar-ui/react-gantt/style.css";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { formatDate, useProfile } from "@/lib/profile";
import { itemKeys, listItems, type Item } from "@/lib/items";

type Scale = "day" | "week" | "month";
const scaleOptions: Scale[] = ["day", "week", "month"];

/** Dates are calendar days (YYYY-MM-DD). SVAR gets local midnights so no time zone can shift a day. */
export const toDay = (value?: string | null) => value ? new Date(+value.slice(0, 4), +value.slice(5, 7) - 1, +value.slice(8, 10)) : undefined;
export const fromDay = (date: Date) => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
const addDays = (date: Date, days: number) => new Date(date.getFullYear(), date.getMonth(), date.getDate() + days);

export function ganttTask(item: Item, parent?: string): ITask {
  const start = toDay(item.start_date) ?? toDay(item.due_date);
  const finish = toDay(item.due_date) ?? start;
  return {
    id: item.id, text: item.title, parent: parent ?? 0, open: true, progress: item.progress_percent ?? 0,
    type: item.is_milestone ? "milestone" : item.child_count ? "summary" : "task",
    ...(start && finish ? { start, end: addDays(finish, 1) } : { unscheduled: true }), // SVAR's end is exclusive
  };
}

export function ItemGantt({ items, spaceId, parentId, readonly, onOpen, onPatch }: {
  items: Item[];
  spaceId: string;
  parentId?: string;
  readonly: boolean;
  onOpen: (item: Item) => void;
  onPatch: (item: Item, update: { start_date: string | null; due_date: string | null }) => Promise<void>;
}) {
  const { t, i18n } = useTranslation();
  const profile = useProfile();
  const [scale, setScale] = useState<Scale>("day");
  const [selected, setSelected] = useState<string>();
  const api = useRef<IApi>(undefined);
  // Every level is fetched with the same cache keys as Table, so SSE updates and PATCH responses reach the chart.
  const [parents, setParents] = useState<string[]>([]);
  const branches = useQueries({
    queries: parents.map((id) => ({ queryKey: itemKeys.children(id), queryFn: () => listItems(`/api/v1/items/${id}/children`) })),
    combine: (results) => results.map((result) => result.data),
  });
  const { tasks, byId } = useMemo(() => {
    const children = new Map(parents.map((id, index) => [id, branches[index] ?? []]));
    const out: ITask[] = [];
    const lookup = new Map<string, Item>();
    function visit(list: Item[], parent: string | undefined, ancestors: Set<string>) {
      for (const item of list) {
        if (ancestors.has(item.id) || lookup.has(item.id)) continue;
        lookup.set(item.id, item);
        out.push(ganttTask(item, parent));
        visit((children.get(item.id) ?? []).filter((child) => child.locations.some((loc) => loc.space_id === spaceId && loc.parent_item_id === item.id)),
          item.id, new Set([...ancestors, item.id]));
      }
    }
    visit(items, parentId, new Set());
    return { tasks: out, byId: lookup };
  }, [items, parents, branches, spaceId, parentId]);
  useEffect(() => {
    const wanted = [...byId.values()].filter((item) => item.child_count).map((item) => item.id).filter((id) => !parents.includes(id));
    if (wanted.length) setParents((list) => [...list, ...wanted]);
  }, [byId, parents]);

  const ref = useRef({ byId, readonly, onPatch, t });
  ref.current = { byId, readonly, onPatch, t };
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const locale = i18n.language;
  const scales = useMemo(() => {
    const label = (options: Intl.DateTimeFormatOptions) => (date: Date) => new Intl.DateTimeFormat(locale, options).format(date);
    return scale === "day"
      ? [{ unit: "month", step: 1, format: label({ month: "long", year: "numeric" }) }, { unit: "day", step: 1, format: label({ day: "numeric" }) }]
      : scale === "week"
        ? [{ unit: "month", step: 1, format: label({ month: "long", year: "numeric" }) }, { unit: "week", step: 1, format: label({ day: "numeric", month: "short" }) }]
        : [{ unit: "year", step: 1, format: label({ year: "numeric" }) }, { unit: "month", step: 1, format: label({ month: "short" }) }];
  }, [scale, locale]);

  /** Both drag/resize and the keyboard end here: nothing changes on screen until the server answers. */
  function change(id: string, start: Date, finish: Date) {
    const { byId, readonly, onPatch, t } = ref.current;
    const item = byId.get(id);
    if (!item) return;
    if (readonly || !item.my_capabilities.includes("edit")) {
      toast.error(t("views.gantt.noEdit", { name: item.title }));
      return;
    }
    const next = { start_date: fromDay(start), due_date: fromDay(finish) };
    if (next.start_date !== item.start_date || next.due_date !== item.due_date) void onPatch(item, next);
  }
  function init(instance: IApi) {
    api.current = instance;
    // SVAR reports the edge it moved (`start` and/or `end`) and a whole-day `diff`; the dates themselves come from our item.
    instance.intercept("update-task", (event: { id: string | number; diff?: number; task?: ITask; inProgress?: boolean }) => {
      if (event.inProgress) return true; // live preview while dragging
      const item = ref.current.byId.get(String(event.id));
      const start = toDay(item?.start_date) ?? toDay(item?.due_date);
      const finish = toDay(item?.due_date) ?? start;
      const { diff = 0, task } = event;
      if (item && start && finish && diff && task) {
        const nextStart = task.start ? addDays(start, diff) : start;
        const nextFinish = item.is_milestone ? nextStart : task.end ? addDays(finish, diff) : finish;
        if (nextFinish >= nextStart) change(item.id, nextStart, nextFinish);
      }
      return false;
    });
    instance.on("select-task", (event: { id: string | number }) => setSelected(String(event.id)));
  }
  function keyboard(event: KeyboardEvent<HTMLDivElement>) {
    const ids = tasks.map((task) => String(task.id));
    const at = selected ? ids.indexOf(selected) : -1;
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const next = ids[Math.min(ids.length - 1, Math.max(0, at + (event.key === "ArrowDown" ? 1 : -1)))];
      if (next) { setSelected(next); api.current?.exec("select-task", { id: next }); }
    } else if ((event.key === "ArrowLeft" || event.key === "ArrowRight") && selected) {
      event.preventDefault();
      const item = byId.get(selected);
      const start = toDay(item?.start_date) ?? toDay(item?.due_date);
      const finish = toDay(item?.due_date) ?? start;
      if (!item || !start || !finish) return;
      const delta = event.key === "ArrowRight" ? 1 : -1;
      if (event.shiftKey) {
        if (!item.is_milestone && addDays(finish, delta) >= start) change(item.id, start, addDays(finish, delta));
      } else change(item.id, addDays(start, delta), addDays(finish, delta));
    } else if (event.key === "Enter" && selected && byId.get(selected)) onOpen(byId.get(selected)!);
  }

  const current = selected ? byId.get(selected) : undefined;
  const Theme = document.documentElement.classList.contains("dark") ? WillowDark : Willow;
  return <div className="space-y-2">
    <div className="flex flex-wrap items-center gap-2">
      <div role="group" aria-label={t("views.gantt.scale")} className="flex gap-1">
        {scaleOptions.map((value) => <Button key={value} size="sm" variant={scale === value ? "default" : "outline"}
          aria-pressed={scale === value} onClick={() => setScale(value)}>{t(`views.gantt.${value}`)}</Button>)}
      </div>
      <p className="text-sm text-muted-foreground">{readonly ? t("views.gantt.readonly") : t("views.gantt.keys")}</p>
    </div>
    <div role="application" aria-label={t("views.gantt.label")} tabIndex={0} onKeyDown={keyboard}
      className="gantt-chart h-[60vh] min-h-72 overflow-hidden rounded border focus-visible:outline-ring [&_.wx-theme]:h-full">
      <Theme>
        <Gantt tasks={tasks} links={[]} readonly={readonly} init={init} scales={scales} cellWidth={scale === "day" ? 36 : scale === "week" ? 80 : 90}
          highlightTime={(date, unit) => unit === "day" && date.getTime() === today.getTime() ? "gantt-today" : ""} schedule={{ auto: false }} unscheduledTasks
          columns={[{ id: "text", header: t("views.gantt.name"), flexgrow: 1, width: 240 }]} />
      </Theme>
    </div>
    <p role="status" className="sr-only">{current ? (current.start_date || current.due_date
      ? t("views.gantt.selected", { name: current.title, start: formatDate(current.start_date ?? current.due_date, profile), end: formatDate(current.due_date ?? current.start_date, profile) })
      : t("views.gantt.unscheduled", { name: current.title })) : ""}</p>
  </div>;
}

