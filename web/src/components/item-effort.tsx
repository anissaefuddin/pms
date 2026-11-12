import { useEffect, useId, useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import type { Item, ItemUpdate } from "@/lib/items";
import type { WorkSchedule } from "@/lib/work-schedule";

type Allocation = NonNullable<Item["assignees"][number]["allocation"]>;
type AllocationDraft = { unit: Allocation["unit"]; value: string };

const maxValue = 2_147_483_647;
const maxHours = (maxValue + 0.49) / 60;

function hours(minutes?: number | null) {
  return minutes == null ? "" : String(Number((minutes / 60).toFixed(2)));
}

function minutes(value: string) {
  return value === "" ? null : Math.round(Number(value) * 60);
}

export function ItemEffort({ item, editable, disabled, onEdit, onSave }: {
  item: Item;
  editable: boolean;
  disabled: boolean;
  onEdit: () => void;
  onSave: (update: ItemUpdate) => Promise<boolean>;
}) {
  const { t } = useTranslation();
  const heading = useId();
  const [effortDraft, setEffortDraft] = useState<string>();
  const [allocations, setAllocations] = useState<Record<string, AllocationDraft>>({});
  const schedule = useQuery({
    queryKey: ["work-schedule"],
    enabled: item.assignees.length > 0,
    queryFn: async () => {
      let etag = "";
      const data = await api<WorkSchedule>("/api/v1/settings/work-schedule", undefined, (headers) => {
        etag = headers.get("ETag") ?? "";
      });
      return { data, etag };
    },
  });
  useEffect(() => {
    const ids = new Set(item.assignees.map(({ id }) => id));
    setAllocations((drafts) => {
      const current = Object.entries(drafts).filter(([id]) => ids.has(id));
      return current.length === Object.keys(drafts).length ? drafts : Object.fromEntries(current);
    });
  }, [item.assignees]);

  const effort = effortDraft ?? hours(item.effort_minutes);
  const rows = item.assignees.map((person) => {
    const saved = person.allocation;
    const unit = saved?.unit ?? "percent";
    const value = unit === "minutes_per_day" ? hours(saved?.value) : String(saved?.value ?? "");
    const draft = allocations[person.id] ?? { unit, value };
    const allocation = draft.value === "" ? null : {
      unit: draft.unit,
      value: draft.unit === "percent" ? Number(draft.value) : Math.round(Number(draft.value) * 60),
    };
    return { person, draft, allocation };
  });
  let totalPercent = 0;
  let totalMinutes = 0;
  for (const { allocation } of rows) {
    if (allocation?.unit === "percent") totalPercent += allocation.value;
    if (allocation?.unit === "minutes_per_day") totalMinutes += allocation.value;
  }
  const dailyMinutes = (schedule.data?.data.daily_hours ?? 0) * 60;
  const overAllocated = totalPercent > 100 || (dailyMinutes > 0 && totalMinutes * 100 > (100 - totalPercent) * dailyMinutes);

  function setAllocation(id: string, draft: AllocationDraft) {
    onEdit();
    setAllocations((current) => ({ ...current, [id]: draft }));
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const submitter = (event.nativeEvent as SubmitEvent).submitter;
    const update: ItemUpdate = {};
    const effortMinutes = minutes(effort);
    if (effortMinutes !== (item.effort_minutes ?? null)) update.effort_minutes = effortMinutes;
    const changed = rows.filter(({ person: { allocation: saved }, allocation }) =>
      saved?.unit !== allocation?.unit || saved?.value !== allocation?.value);
    if (changed.length) {
      update.assignee_allocations = changed.map(({ person, allocation }) => ({ assignee_id: person.id, allocation }));
    }
    if (submitter?.getAttribute("name") === "recalculate_duration") update.recalculate_duration = true;
    if (Object.keys(update).length && await onSave(update)) {
      setEffortDraft(undefined);
      setAllocations({});
    }
  }

  return <section aria-labelledby={heading} className="space-y-2">
    <h3 id={heading} className="font-medium">{t("items.effortAllocation")}</h3>
    <form onSubmit={submit}>
      <fieldset disabled={!editable || disabled} className="space-y-2">
        <label className="block max-w-xs space-y-1">
          <span>{t("items.effortHours")}</span>
          <Input type="number" min={0} max={maxHours} step="any" value={effort}
            onChange={(event) => { onEdit(); setEffortDraft(event.target.value); }} />
        </label>
        {rows.map(({ person, draft }) => <div key={person.id} className="grid grid-cols-[minmax(0,1fr)_auto] items-end gap-2">
          <label className="min-w-0 space-y-1">
            <span className="break-words">{person.name}</span>
            <Input aria-label={t("items.allocationFor", { name: person.name })}
              type="number" min={0} max={draft.unit === "percent" ? maxValue : maxHours}
              step={draft.unit === "percent" ? 1 : "any"} value={draft.value}
              onChange={(event) => setAllocation(person.id, { ...draft, value: event.target.value })} />
          </label>
          <select aria-label={t("items.allocationUnitFor", { name: person.name })}
            className="h-8 rounded-lg border border-input bg-background px-2.5 text-sm" value={draft.unit}
            onChange={(event) => setAllocation(person.id, { ...draft, unit: event.target.value as Allocation["unit"] })}>
            <option value="percent">{t("items.allocationPercent")}</option>
            <option value="minutes_per_day">{t("items.allocationHours")}</option>
          </select>
        </div>)}
        {overAllocated && <p role="status" className="rounded border border-warning bg-warning-subtle p-2 text-sm">
          {t("items.overAllocated")}
        </p>}
        {item.duration_days != null && <p className="text-sm">{t("items.durationDays", { count: item.duration_days })}</p>}
        {editable && <div className="flex flex-wrap gap-2">
          <Button type="submit" variant="outline">{t("items.saveEffort")}</Button>
          {!item.is_milestone && <Button type="submit" name="recalculate_duration" variant="outline" disabled={effort === ""}>
            {t("items.recalculateDuration")}
          </Button>}
        </div>}
      </fieldset>
    </form>
  </section>;
}
