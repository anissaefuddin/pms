import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ArrowDown, ArrowUp } from "lucide-react";
import type { AutomationRule } from "@/lib/automation";
import { AdminToolbar } from "@/components/admin/admin-ui";

// Shared by the Workspace and Space rule lists so both tables search, filter and sort the same way.
export type RuleState = "all" | "enabled" | "disabled" | "attention" | "archived";
export type RuleSortKey = "name" | "state" | "last_run";
export type RuleSort = { key: RuleSortKey; dir: 1 | -1 };

export const ruleStates: RuleState[] = ["all", "enabled", "disabled", "attention", "archived"];
const selectClass = "h-9 rounded-lg border border-input bg-background px-2 text-sm";

/** A rule's state in one word: archived wins, then a paused or invalid rule, then on or off. */
export function stateOf(rule: AutomationRule): Exclude<RuleState, "all"> {
  if (rule.archived_at) return "archived";
  if (rule.health !== "ok") return "attention";
  return rule.enabled ? "enabled" : "disabled";
}

/** Rules whose name matches the search and whose state matches the filter, sorted by one column. */
export function filterRules(rules: AutomationRule[], search: string, state: RuleState, sort: RuleSort): AutomationRule[] {
  const needle = search.trim().toLowerCase();
  const list = rules.filter((rule) => (state === "all" || stateOf(rule) === state) && (!needle || rule.name.toLowerCase().includes(needle)));
  const value = (rule: AutomationRule) => sort.key === "name" ? rule.name.toLowerCase()
    : sort.key === "state" ? stateOf(rule) : rule.last_run_at ?? "";
  return [...list].sort((a, b) => (value(a) < value(b) ? -1 : value(a) > value(b) ? 1 : 0) * sort.dir);
}

export function nextSort(sort: RuleSort, key: RuleSortKey): RuleSort {
  return sort.key === key ? { key, dir: sort.dir === 1 ? -1 : 1 } : { key, dir: 1 };
}

/** Search box, state filter, and any extra buttons, on one toolbar row. */
export function RuleToolbar({ search, onSearch, state, onState, children }: {
  search: string; onSearch: (value: string) => void; state: RuleState; onState: (value: RuleState) => void; children?: ReactNode;
}) {
  const { t } = useTranslation();
  return <AdminToolbar searchLabel={t("automation.search")} search={search} onSearch={onSearch}>
    <select aria-label={t("automation.filterState")} className={selectClass} value={state} onChange={(event) => onState(event.target.value as RuleState)}>
      {ruleStates.map((value) => <option key={value} value={value}>{t(`automation.filter.${value}`)}</option>)}
    </select>
    {children}
  </AdminToolbar>;
}

/** A sortable column header, the same button-with-aria-sort pattern as the Custom fields table. */
export function SortHeader({ labelKey, sortKey, sort, onSort }: { labelKey: string; sortKey: RuleSortKey; sort: RuleSort; onSort: (next: RuleSort) => void }) {
  const { t } = useTranslation();
  const active = sort.key === sortKey;
  return <th scope="col" aria-sort={active ? (sort.dir === 1 ? "ascending" : "descending") : "none"} className="py-2 pr-4 font-medium">
    <button type="button" className="inline-flex items-center gap-1" onClick={() => onSort(nextSort(sort, sortKey))}>
      {t(labelKey)}{active && (sort.dir === 1 ? <ArrowUp className="size-3.5" aria-hidden="true" /> : <ArrowDown className="size-3.5" aria-hidden="true" />)}
    </button></th>;
}
