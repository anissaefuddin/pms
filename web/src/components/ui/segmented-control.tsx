import { useRef, type KeyboardEvent, type ReactNode } from "react"

import { cn } from "@/lib/utils"
import { nextIndex } from "@/lib/ui-kit"

export type SegmentedItem = { value: string; label: string; icon?: ReactNode }

/** Pilihan tunggal berbentuk radiogroup; panah berpindah dan memilih, Home/End ke ujung (roving tabindex). */
export function SegmentedControl({
  items,
  value,
  onChange,
  "aria-label": ariaLabel,
  className,
}: {
  items: SegmentedItem[]
  value: string
  onChange: (value: string) => void
  "aria-label"?: string
  className?: string
}) {
  const refs = useRef<(HTMLButtonElement | null)[]>([])
  const activeIndex = Math.max(0, items.findIndex((item) => item.value === value))

  function onKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    const next = nextIndex(index, items.length, event.key)
    if (next === null) return
    event.preventDefault()
    onChange(items[next].value)
    refs.current[next]?.focus()
  }

  return (
    <div
      role="radiogroup"
      aria-label={ariaLabel}
      className={cn("inline-flex h-8 items-center gap-0.5 rounded-lg bg-muted p-0.5", className)}
    >
      {items.map((item, i) => {
        const checked = i === activeIndex
        return (
          <button
            key={item.value}
            ref={(el) => {
              refs.current[i] = el
            }}
            type="button"
            role="radio"
            aria-checked={checked}
            tabIndex={checked ? 0 : -1}
            onClick={() => onChange(item.value)}
            onKeyDown={(event) => onKeyDown(event, i)}
            className={cn(
              "inline-flex h-7 items-center gap-1.5 rounded-md px-2.5 text-sm font-medium outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 [&_svg]:size-4",
              checked ? "bg-card text-foreground shadow-sm" : "text-foreground/75 hover:text-foreground"
            )}
          >
            {item.icon}
            {item.label}
          </button>
        )
      })}
    </div>
  )
}
