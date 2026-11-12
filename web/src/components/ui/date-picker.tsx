import { CalendarIcon } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { DayPicker } from "react-day-picker";
import "react-day-picker/style.css";
import { formatDate, useProfile } from "@/lib/profile";
import { Button } from "./button";
import { Popover, PopoverContent, PopoverTrigger } from "./popover";

export function calendarDate(value?: string | null) {
  return value ? new Date(`${value}T12:00:00`) : undefined;
}

export function dateValue(value?: Date) {
  if (!value) return "";
  return [String(value.getFullYear()).padStart(4, "0"), ...[value.getMonth() + 1, value.getDate()]
    .map((part) => String(part).padStart(2, "0"))].join("-");
}

export function DatePicker({ value, onChange, label, workingDays, disabled }: {
  value?: Date; onChange(date?: Date): void; label?: string; workingDays?: boolean; disabled?: boolean;
}) {
  const { t } = useTranslation();
  const profile = useProfile();
  const weekStartsOn = profile?.week_start === "sunday" ? 0 : profile?.week_start === "saturday" ? 6 : 1;
  const [open, setOpen] = useState(false);
  const display = value ? formatDate(dateValue(value), profile) : t("common.datePicker.placeholder");
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger render={<Button type="button" variant="outline" aria-label={label ? `${label}: ${display}` : undefined} disabled={disabled} />}>
        <CalendarIcon className="size-4" />
        {display}
      </PopoverTrigger>
      <PopoverContent className="w-auto p-3" aria-label={label}>
        <DayPicker weekStartsOn={weekStartsOn} mode="single" selected={value} defaultMonth={value} onSelect={(date) => {
          onChange(date);
          if (workingDays) setOpen(false);
        }}
          disabled={workingDays ? { dayOfWeek: [0, 6] } : undefined} />
      </PopoverContent>
    </Popover>
  );
}
