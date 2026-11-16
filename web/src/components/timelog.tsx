import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Filter } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { api } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import { formatMinutes, type TimeEntryPage } from "@/lib/time-entries";

const pageSize = 50;
const dateFilters = ["from", "to", "category"] as const;
const columns = ["user", "date", "duration", "category", "comment", "billing", "locked"];
const selectClass = "h-8 rounded border bg-background px-2";
const sorts = { "-date": "newest", date: "oldest", duration: "shortest", "-duration": "longest", user: "byUser" };

export function Timelog({ project }: { project: string }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const [offset, setOffset] = useState(0);
  const [filter, setFilter] = useState({ from: "", to: "", category: "", billing_type: "", locked: "", sort: "-date" });
  const params = new URLSearchParams({ ...filter, limit: String(pageSize), offset: String(offset) });
  const result = useQuery({
    queryKey: ["time-entries", project, params.toString()],
    queryFn: ({ signal }) => api<TimeEntryPage>(`/api/v1/items/${project}/time-entries?${params}`, { signal }),
  });
  // Filters are active when any of them differs from its default; then "no entries" means "no match", not "none logged yet".
  const filtersActive = !!(filter.from || filter.to || filter.category || filter.billing_type || filter.locked);
  function change(key: keyof typeof filter, value: string) {
    setOffset(0);
    setFilter({ ...filter, [key]: value });
  }
  return <section aria-label={t("time.timelog")} className="min-w-0 space-y-3">
    <div role="toolbar" aria-label={t("views.toolbar")} className="flex flex-wrap items-center gap-2">
      <Popover>
        <PopoverTrigger render={<Button variant={filter.from || filter.to || filter.category || filter.billing_type || filter.locked ? "secondary" : "outline"} size="sm" />}><Filter />{t("views.chip.filter")}</PopoverTrigger>
        <PopoverContent align="start" aria-label={t("views.options", { name: t("views.chip.filter") })} className="w-72">
    <div className="space-y-3">
        {dateFilters.map((key) => <label key={key} className="space-y-1">
          <span>{t(key === "category" ? "time.category" : `time.${key}Date`)}</span>
          <Input type={key === "category" ? "text" : "date"} value={filter[key]} onChange={(event) => change(key, event.target.value)} />
        </label>)}
        <label className="grid gap-1">{t("time.billing")}
          <select className={selectClass} value={filter.billing_type} onChange={(e) => change("billing_type", e.target.value)}>
            <option value="">{t("time.all")}</option>
            <option value="billable">{t("time.billable")}</option>
            <option value="non_billable">{t("time.non_billable")}</option>
          </select>
        </label>
        <label className="grid gap-1">{t("time.locked")}
          <select className={selectClass} value={filter.locked} onChange={(e) => change("locked", e.target.value)}>
            <option value="">{t("time.all")}</option>
            <option value="true">{t("time.locked")}</option>
            <option value="false">{t("time.unlocked")}</option>
          </select>
        </label>
        <label className="grid gap-1">{t("time.sort")}
          <select className={selectClass} value={filter.sort} onChange={(e) => change("sort", e.target.value)}>
            {Object.entries(sorts).map(([key, value]) => <option key={key} value={key}>{t(`time.${value}`)}</option>)}
          </select>
        </label>
      </div>
        </PopoverContent>
      </Popover>
    </div>
    {result.isPending && <p role="status">{t("items.loading")}</p>}
    {result.isError && <ErrorState message={t("time.loadFailed")} retryLabel={t("items.retry")} onRetry={() => void result.refetch()} />}
    {result.data?.items.length === 0 && <div className="space-y-2 rounded border border-dashed p-6 text-center">
      <p>{filtersActive ? t("time.empty") : t("time.noEntries")}</p>
      {filtersActive && <Button variant="outline" size="sm" onClick={() => { setOffset(0); setFilter({ ...filter, from: "", to: "", category: "", billing_type: "", locked: "" }); }}>{t("time.clearFilters")}</Button>}
    </div>}
    {!!result.data?.items.length && <div className="overflow-auto rounded border">
      <table aria-label={t("time.timelog")} className="w-full text-sm">
        <thead className="bg-muted"><tr>{[t("items.task"), ...columns.map((key) => t(`time.${key}`))].map((label) =>
          <th scope="col" key={label} className="p-2 text-left">{label}</th>)}</tr></thead>
        <tbody>{result.data.items.map((entry) => <tr className="border-t align-top" key={entry.id}>
          <td className="p-2">{entry.item_title}</td>
          <td className="p-2">{entry.user_name}</td>
          <td className="whitespace-nowrap p-2">{formatDate(entry.date, profile)}</td>
          <td className="p-2">{formatMinutes(entry.minutes ?? 0)}</td>
          <td className="p-2">{entry.category}</td>
          <td className="max-w-xs break-words p-2">{entry.comment}</td>
          <td className="p-2">{t(`time.${entry.billing_type}`)}</td>
          <td className="p-2">{t(entry.locked ? "time.locked" : "time.unlocked")}</td>
        </tr>)}</tbody>
      </table>
    </div>}
    {!!result.data?.items.length && <p className="text-sm text-muted-foreground">{t("time.pageTotal", { count: result.data.items.length, time: formatMinutes(result.data.items.reduce((sum, entry) => sum + (entry.minutes ?? 0), 0)) })}</p>}
    {(offset > 0 || result.data?.next_offset != null) && <div className="flex gap-2">
      <Button variant="outline" disabled={!offset || result.isPending} onClick={() => setOffset(Math.max(0, offset - pageSize))}>{t("time.previousPage")}</Button>
      <Button variant="outline" disabled={result.data?.next_offset == null} onClick={() => setOffset(result.data?.next_offset ?? offset)}>{t("time.nextPage")}</Button>
    </div>}
  </section>;
}
