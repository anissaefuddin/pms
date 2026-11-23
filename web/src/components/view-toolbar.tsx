import type { ReactNode } from "react";
import { ArrowUpDown, Filter, Layers, Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { FieldFilter } from "@/components/view-columns";
import type { CustomField } from "@/lib/custom-fields";
import { fieldSort, sortableField } from "@/lib/view-columns";

export const viewSorts = ["manual", "priority", "due_date", "status", "title", "predecessors", "start_date"] as const;
const presets = ["all", "active", "mine"] as const;
const selectClass = "block w-full rounded border bg-background p-2";

function Chip({ label, icon, active, children }: { label: string; icon: ReactNode; active?: boolean; children: ReactNode }) {
  const { t } = useTranslation();
  return <Popover>
    <PopoverTrigger render={<Button variant={active ? "secondary" : "outline"} size="sm" />}>{icon}{label}</PopoverTrigger>
    <PopoverContent align="start" aria-label={t("views.options", { name: label })} className="w-64 space-y-3">{children}</PopoverContent>
  </Popover>;
}

/** One-row view toolbar: chips with popovers; all state lives in the URL through `setSearch`. */
export function ViewToolbar({ filter, preset, typeFilter, sort, group, fieldFilter, customFields, itemTypes, groups, setSearch, columns, filterOnly = false }: {
  filter: string; preset: string; typeFilter: string; sort: string; group: string; fieldFilter: string;
  customFields: CustomField[]; itemTypes?: { id: string; key: string; name: string }[]; groups?: readonly string[];
  setSearch: (key: string, value: string) => void; columns?: ReactNode; filterOnly?: boolean;
}) {
  const { t } = useTranslation();
  return <div role="toolbar" aria-label={t("views.toolbar")} className="flex flex-wrap items-center gap-1 rounded-lg border bg-card p-1 text-[13px] md:flex-nowrap">
    <Chip label={t("views.chip.filter")} icon={<Filter />} active={!!(preset || typeFilter || fieldFilter)}>
      <label className="block space-y-1"><span>{t("views.preset.label")}</span>
        <select aria-label={t("views.preset.label")} value={preset || "all"} onChange={(event) => setSearch("preset", event.target.value === "all" ? "" : event.target.value)} className={selectClass}>
          {presets.map((value) => <option value={value} key={value}>{t(`views.preset.${value}`)}</option>)}
        </select></label>
      {!filterOnly && <label className="block space-y-1"><span>{t("views.typeFilter")}</span>
        <select aria-label={t("views.typeFilter")} value={typeFilter} onChange={(event) => setSearch("type", event.target.value)} className={selectClass}>
          <option value="">{t("views.typeAny")}</option>
          {typeFilter && !itemTypes?.some((entry) => entry.key === typeFilter) && <option value={typeFilter}>{typeFilter}</option>}
          {itemTypes?.map((entry) => <option key={entry.id} value={entry.key}>{entry.name}</option>)}
        </select></label>}
      <FieldFilter fields={customFields.filter((field) => field.type !== "people")} value={fieldFilter} onChange={(value) => setSearch("field", value)} />
    </Chip>
    {!filterOnly && <Chip label={t("views.chip.sort")} icon={<ArrowUpDown />} active={sort !== "manual"}>
      <label className="block space-y-1"><span>{t("views.sort")}</span>
        <select aria-label={t("views.sort")} value={sort} onChange={(event) => setSearch("sort", event.target.value)} className={selectClass}>
          {viewSorts.map((value) => <option value={value} key={value}>{t(`views.${value}`)}</option>)}
          {customFields.filter(sortableField).map((field) => <option value={fieldSort(field.id)} key={field.id}>{t("views.fieldSort", { name: field.name })}</option>)}
        </select></label>
    </Chip>}
    {!filterOnly && groups && <Chip label={t("views.chip.group")} icon={<Layers />} active={group !== "none"}>
      <label className="block space-y-1"><span>{t("views.group")}</span>
        <select aria-label={t("views.group")} value={group} onChange={(event) => setSearch("group", event.target.value === "none" ? "" : event.target.value)} className={selectClass}>
          {groups.map((value) => <option value={value} key={value}>{t(`views.${value}`)}</option>)}
        </select></label>
    </Chip>}
    {columns}
    <label className="relative flex w-full items-center sm:ml-auto sm:w-auto">
      <Search className="pointer-events-none absolute left-2 size-4 text-muted-foreground" aria-hidden />
      <Input aria-label={t("views.filter")} placeholder={t("views.searchPlaceholder")} value={filter} maxLength={120} className="h-8 w-full pl-8 sm:w-44"
        onChange={(event) => setSearch("filter", event.target.value)} />
    </label>
  </div>;
}
