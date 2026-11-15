import type { ReactNode } from "react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { primaryActionFor } from "@/lib/ui-kit"

export type EmptyStateAction = { label: string; onClick: () => void }

export function EmptyState({
  icon,
  title,
  description,
  primaryAction,
  secondaryAction,
  canCreate = true,
  className,
}: {
  icon?: ReactNode
  title: string
  description?: string
  primaryAction?: EmptyStateAction
  secondaryAction?: EmptyStateAction
  /** Izin membuat; bila false, aksi utama tidak dirender. */
  canCreate?: boolean
  className?: string
}) {
  const primary = primaryActionFor(canCreate, primaryAction)

  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed bg-card px-6 py-10 text-center",
        className
      )}
    >
      {icon && <div className="text-muted-foreground [&_svg]:size-8" aria-hidden="true">{icon}</div>}
      <h2 className="text-base font-semibold text-foreground">{title}</h2>
      {description && <p className="max-w-sm text-sm text-muted-foreground">{description}</p>}
      {(primary || secondaryAction) && (
        <div className="mt-1 flex flex-wrap items-center justify-center gap-2">
          {primary && <Button onClick={primary.onClick}>{primary.label}</Button>}
          {secondaryAction && (
            <Button variant="outline" onClick={secondaryAction.onClick}>
              {secondaryAction.label}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
