import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { Item, ItemUpdate } from "@/lib/items";

const methods = ["task_count", "weighted_duration", "average_percent"] as const;
const riskColors = { green: "bg-success", yellow: "bg-warning", red: "bg-danger" };

export function ProjectProgress({ item }: { item: Item }) {
  const { t } = useTranslation();
  if (item.type !== "project" || item.progress_percent == null || !item.progress_risk) return null;
  // Three separate reads: a bar, the percentage, and the risk as its own badge (not one run-on label).
  const value = Math.max(0, Math.min(100, item.progress_percent));
  return <span className="inline-flex items-center gap-2 tabular-nums" title={t("items.progress.methodValue", { method: t(`items.progress.methods.${item.progress_method ?? "task_count"}`) })}>
    <span role="meter" aria-label={t("items.progress.percent", { value })} aria-valuemin={0} aria-valuemax={100} aria-valuenow={value}
      className="relative block h-1.5 w-14 shrink-0 overflow-hidden rounded-full bg-muted">
      <span aria-hidden="true" className="absolute inset-y-0 start-0 rounded-full bg-primary" style={{ width: `${value}%` }} />
    </span>
    <span className="w-9 text-end">{t("items.progress.percent", { value })}</span>
    {value > 0 && <span className="inline-flex shrink-0 items-center gap-1 whitespace-nowrap rounded-full border px-2 py-0.5 text-xs">
      <span aria-hidden="true" className={`size-2 rounded-full ${riskColors[item.progress_risk]}`} />
      {t(`items.progress.risk.${item.progress_risk}`)}
    </span>}
  </span>;
}

export function ItemProgress({ item, editable, disabled, onEdit, onSave }: {
  item: Item;
  editable: boolean;
  disabled: boolean;
  onEdit: () => void;
  onSave: (update: ItemUpdate) => Promise<boolean>;
}) {
  const { t } = useTranslation();
  const heading = useId();
  const hint = useId();
  const [draft, setDraft] = useState<string>();
  if (item.type === "folder") return null;

  if (item.type === "project") return <section aria-labelledby={heading} className="space-y-2">
    <h3 id={heading} className="font-semibold">{t("items.progress.project")}</h3>
    <ProjectProgress item={item} />
    <label className="block space-y-1">
      <span>{t("items.progress.method")}</span>
      <select className="block w-full rounded border bg-background p-2" value={item.progress_method ?? "task_count"}
        disabled={!editable || disabled} onChange={(event) => {
          const method = methods.find((method) => method === event.target.value);
          if (method) void onSave({ progress_method: method });
        }}>
        {methods.map((method) => <option key={method} value={method}>{t(`items.progress.methods.${method}`)}</option>)}
      </select>
    </label>
  </section>;

  const completed = item.status?.group === "completed";
  return <section aria-labelledby={heading} className="space-y-2">
    <h3 id={heading} className="font-semibold">{t("items.progress.task")}</h3>
    {completed && <p>{t("items.progress.complete", { value: 100 })}</p>}
    {editable ? <form className="space-y-1" onSubmit={async (event) => {
      event.preventDefault();
      if (draft === undefined) return;
      if (await onSave({ progress_percent: draft === "" ? null : Number(draft) })) setDraft(undefined);
    }}>
      <div className="flex items-end gap-2">
        <label className="min-w-0 flex-1 space-y-1">
          <span>{t("items.progress.manual")}</span>
          <Input type="number" min={0} max={100} step={1} disabled={disabled} aria-describedby={hint}
            value={draft ?? item.progress_percent ?? ""} onChange={(event) => {
              onEdit();
              setDraft(event.target.value);
            }} />
        </label>
        <Button type="submit" variant="outline" disabled={disabled || draft === undefined}>{t("items.progress.save")}</Button>
      </div>
      <p id={hint} className="text-sm text-muted-foreground">{t("items.progress.clear")}</p>
    </form> : !completed && <p>{t("items.progress.percent", { value: item.progress_percent ?? 0 })}</p>}
  </section>;
}
