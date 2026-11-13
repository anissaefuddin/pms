import { useTranslation } from "react-i18next";
import { ENUM_DIMENSIONS, type AnalyticsQuery, type AnalyticsResult } from "@/lib/reports";

/** Labels for a query's metric, dimensions and values; shared by reports, widget tables and the chart summaries. */
export function useResultLabels(query: AnalyticsQuery) {
  const { t } = useTranslation();
  const metric = t(query.reporting_on === "time_entries" && query.metric === "count" ? "reports.metric.countEntries" : `reports.metric.${query.metric}`);
  const group = query.group_by ? t(`reports.dim.${query.group_by}`) : t("reports.result.group");
  const value = (dimension: string | undefined, key: string, text: string) => !text && !key ? t("reports.value.none")
    : dimension && ENUM_DIMENSIONS.includes(dimension) ? t(`reports.value.${key}`, { defaultValue: text }) : text;
  return { metric, group, breakBy: query.break_by ? t(`reports.dim.${query.break_by}`) : "", value };
}

export function ResultTable({ result, query, caption }: { result: AnalyticsResult; query: AnalyticsQuery; caption?: string }) {
  const { t } = useTranslation();
  const labels = useResultLabels(query);
  return <table className="w-full text-left text-sm">
    {caption && <caption className="sr-only">{caption}</caption>}
    <thead><tr className="border-b">
      <th scope="col" className="py-2 pr-4">{labels.group}</th>
      {query.break_by && <th scope="col" className="py-2 pr-4">{labels.breakBy}</th>}
      <th scope="col" className="py-2 pr-4 text-right">{labels.metric}</th>
    </tr></thead>
    <tbody>{result.rows.map((row) => <tr key={`${row.group.key}/${row.break?.key ?? ""}`} className="border-b">
      <th scope="row" className="py-2 pr-4 font-normal">{labels.value(query.group_by, row.group.key, row.group.label)}</th>
      {query.break_by && <td className="py-2 pr-4">{row.break ? labels.value(query.break_by, row.break.key, row.break.label) : ""}</td>}
      <td className="py-2 pr-4 text-right tabular-nums">{row.value}</td>
    </tr>)}</tbody>
    <tfoot><tr><th scope="row" colSpan={query.break_by ? 2 : 1} className="py-2 pr-4 text-left">{t("reports.result.total", { count: result.total })}</th><td /></tr></tfoot>
  </table>;
}
