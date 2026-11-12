import { useState } from "react";
import { useTranslation } from "react-i18next";
import { DatePicker } from "@/components/ui/date-picker";

function asLocalDate(value?: string | null) {
  return value ? new Date(`${value}T12:00:00`) : undefined;
}

function asDateString(value?: Date) {
  if (!value) return "";
  return [String(value.getFullYear()).padStart(4, "0"), ...[value.getMonth() + 1, value.getDate()]
    .map((part) => String(part).padStart(2, "0"))].join("-");
}

export function ItemDates({ initialStart, initialDue, invalid }: {
  initialStart?: string | null;
  initialDue?: string | null;
  invalid?: boolean;
}) {
  const { t } = useTranslation();
  const [start, setStart] = useState(() => asLocalDate(initialStart));
  const [due, setDue] = useState(() => asLocalDate(initialDue));
  return <div className="grid grid-cols-2 gap-2">
    <div className="min-w-0 space-y-1">
      <p>{t("items.start")}</p>
      <DatePicker value={start} onChange={setStart} label={t("items.start")} workingDays />
      <input type="hidden" name="start_date" value={asDateString(start)} />
    </div>
    <div className="min-w-0 space-y-1" aria-invalid={invalid}>
      <p>{t("items.due")}</p>
      <DatePicker value={due} onChange={setDue} label={t("items.due")} workingDays />
      <input type="hidden" name="due_date" value={asDateString(due)} />
    </div>
  </div>;
}
