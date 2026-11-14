import { useState } from "react";
import { useTranslation } from "react-i18next";
import { DatePicker, calendarDate, dateValue } from "@/components/ui/date-picker";

export function ItemDates({ initialStart, initialDue, invalid, onEdit, onChange }: {
  initialStart?: string | null;
  initialDue?: string | null;
  invalid?: boolean;
  onEdit?: () => void;
  /** Called with the new start and due (YYYY-MM-DD or "") after each pick. */
  onChange?: (start: string, due: string) => void;
}) {
  const { t } = useTranslation();
  const [start, setStart] = useState(() => calendarDate(initialStart));
  const [due, setDue] = useState(() => calendarDate(initialDue));
  return <div className="grid grid-cols-2 gap-2">
    <div className="min-w-0 space-y-1">
      <p>{t("items.start")}</p>
      <DatePicker value={start} onChange={(value) => { onEdit?.(); setStart(value); onChange?.(dateValue(value), dateValue(due)); }} label={t("items.start")} workingDays />
      <input type="hidden" name="start_date" value={dateValue(start)} />
    </div>
    <div className="min-w-0 space-y-1" aria-invalid={invalid}>
      <p>{t("items.due")}</p>
      <DatePicker value={due} onChange={(value) => { onEdit?.(); setDue(value); onChange?.(dateValue(start), dateValue(value)); }} label={t("items.due")} workingDays />
      <input type="hidden" name="due_date" value={dateValue(due)} />
    </div>
  </div>;
}
