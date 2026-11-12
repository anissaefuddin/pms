import { CalendarIcon } from "lucide-react";
import { useTranslation } from "react-i18next";
import { DayPicker } from "react-day-picker";
import "react-day-picker/style.css";
import { Button } from "./button";
import { Popover, PopoverContent, PopoverTrigger } from "./popover";

export function DatePicker({ value, onChange }: { value?: Date; onChange(date?: Date): void }) {
  const { t } = useTranslation();
  return (
    <Popover>
      <PopoverTrigger render={<Button variant="outline" />}>
        <CalendarIcon className="size-4" />
        {value?.toLocaleDateString() ?? t("common.datePicker.placeholder")}
      </PopoverTrigger>
      <PopoverContent className="w-auto p-3">
        <DayPicker mode="single" selected={value} onSelect={onChange} />
      </PopoverContent>
    </Popover>
  );
}
