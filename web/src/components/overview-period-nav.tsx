import { useTranslation } from "react-i18next";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { addDays } from "@/lib/overview";
import { formatDate, useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";

/** Moves a date range back or forward by its length. onMove(null) returns to today. */
export function PeriodNav({ start, end, days, onMove }: {
  start?: string
  end?: string
  days: number
  onMove: (from: string | null) => void
}) {
  const { t } = useTranslation();
  const profile = useProfile();
  return <div className="flex flex-wrap items-center gap-2">
    <Button variant="outline" size="sm" aria-label={t("dashboards.overview.range.prev")} disabled={!start}
      onClick={() => start && onMove(addDays(start, -days))}><ChevronLeft /></Button>
    <Button variant="outline" size="sm" onClick={() => onMove(null)}>{t("dashboards.overview.range.today")}</Button>
    <Button variant="outline" size="sm" aria-label={t("dashboards.overview.range.next")} disabled={!start}
      onClick={() => start && onMove(addDays(start, days))}><ChevronRight /></Button>
    {start && end && <p className="text-sm text-muted-foreground">
      {t("dashboards.overview.range.label", { from: formatDate(start, profile), to: formatDate(end, profile) })}</p>}
  </div>;
}
