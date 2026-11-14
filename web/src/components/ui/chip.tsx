import type { ReactNode } from "react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

export const chipClass = "inline-flex min-h-8 items-center gap-1 rounded-full border bg-background px-3 text-sm hover:bg-muted focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50";

/** A pill that opens a popover for editing one field in place. `label` is the accessible name (field plus value). */
export function PopoverChip({ label, children, content, disabled }: {
  label: string; children: ReactNode; content: ReactNode; disabled?: boolean;
}) {
  return <Popover>
    <PopoverTrigger className={chipClass} aria-label={label} disabled={disabled}>{children}</PopoverTrigger>
    <PopoverContent align="start" className="w-80 space-y-2">{content}</PopoverContent>
  </Popover>;
}
