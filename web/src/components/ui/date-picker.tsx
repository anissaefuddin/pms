import { CalendarIcon } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { DayPicker } from "react-day-picker";
import "react-day-picker/style.css";
import { Button } from "./button";
import { Popover, PopoverContent, PopoverTrigger } from "./popover";

export function DatePicker({ value, onChange, label, workingDays }: {
  value?: Date; onChange(date?: Date): void; label?: string; workingDays?: boolean;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger render={<Button type="button" variant="outline" aria-label={label} />}>
        <CalendarIcon className="size-4" />
        {value?.toLocaleDateString() ?? t("common.datePicker.placeholder")}
      </PopoverTrigger>
      <PopoverContent className="w-auto p-3" aria-label={label}>
        <DayPicker mode="single" selected={value} defaultMonth={value} onSelect={(date) => {
          onChange(date);
          if (workingDays) setOpen(false);
        }}
          disabled={workingDays ? { dayOfWeek: [0, 6] } : undefined} />
      </PopoverContent>
    </Popover>
  );
}
