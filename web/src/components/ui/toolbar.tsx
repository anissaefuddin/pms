import type { ComponentProps } from "react"

import { cn } from "@/lib/utils"

export function Toolbar({
  "aria-label": ariaLabel,
  className,
  children,
  ...props
}: ComponentProps<"div"> & { "aria-label": string }) {
  return (
    <div
      role="toolbar"
      aria-label={ariaLabel}
      aria-orientation="horizontal"
      className={cn("flex flex-wrap items-center gap-2", className)}
      {...props}
    >
      {children}
    </div>
  )
}

export function ToolbarSeparator({ className }: { className?: string }) {
  return <div role="separator" aria-orientation="vertical" className={cn("mx-1 h-5 w-px bg-border", className)} />
}
