import { cn } from "@/lib/utils"
import { initials } from "@/lib/ui-kit"

export function AvatarStack({ names, max = 4, className }: { names: string[]; max?: number; className?: string }) {
  const shown = names.slice(0, max)
  const extra = names.length - shown.length

  return (
    <div role="group" aria-label={names.join(", ")} className={cn("flex -space-x-2", className)}>
      {shown.map((name, i) => (
        <span
          key={`${i}-${name}`}
          aria-hidden="true"
          title={name}
          className="inline-flex size-7 items-center justify-center rounded-full border-2 border-card bg-muted text-xs font-semibold text-muted-foreground"
        >
          {initials(name)}
        </span>
      ))}
      {extra > 0 && (
        <span
          aria-hidden="true"
          className="inline-flex size-7 items-center justify-center rounded-full border-2 border-card bg-secondary text-xs font-semibold text-secondary-foreground"
        >
          +{extra}
        </span>
      )}
    </div>
  )
}
