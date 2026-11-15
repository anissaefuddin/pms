import type { CSSProperties } from "react"

import { cn } from "@/lib/utils"
import { isCatToken } from "@/lib/ui-kit"

// Kelas statis agar Tailwind ikut men-generate; jangan dibentuk dari string dinamis.
const CAT_CLASSES = {
  "cat-1": "bg-cat-1-subtle text-cat-1",
  "cat-2": "bg-cat-2-subtle text-cat-2",
  "cat-3": "bg-cat-3-subtle text-cat-3",
  "cat-4": "bg-cat-4-subtle text-cat-4",
  "cat-5": "bg-cat-5-subtle text-cat-5",
  "cat-6": "bg-cat-6-subtle text-cat-6",
} as const

/** `color` berupa nama token cat-1..cat-6 atau string warna CSS (mis. dari workflow). */
export function StatusChip({ label, color, className }: { label: string; color?: string; className?: string }) {
  const token = isCatToken(color) ? CAT_CLASSES[color] : undefined
  const style: CSSProperties | undefined =
    color && !token ? { color, backgroundColor: `color-mix(in srgb, ${color} 14%, transparent)` } : undefined

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium",
        token ?? (color ? undefined : "bg-muted text-muted-foreground"),
        className
      )}
      style={style}
    >
      <span className="size-1.5 rounded-full bg-current" aria-hidden="true" />
      {label}
    </span>
  )
}
