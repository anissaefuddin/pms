import type { ReactNode } from "react";
import { Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";

// Shared building blocks for Account management pages: one header, a consistent toolbar, semantic status, and empty states.

/**
 * Description and the single primary action for a page. The page title is shown once by the Settings layout,
 * so this header only adds a title when a page needs a section heading of its own.
 */
export function AdminPageHeader({ title, description, action }: { title?: string; description?: string; action?: ReactNode }) {
  return <header className="flex flex-wrap items-start justify-between gap-4">
    <div className="min-w-0 space-y-1">
      {title && <h2 className="text-base font-semibold">{title}</h2>}
      {description && <p className="text-sm text-muted-foreground">{description}</p>}
    </div>
    {action && <div className="shrink-0">{action}</div>}
  </header>;
}

/** Search and filters above a table. Filters are plain selects so they work the same on every page. */
export function AdminToolbar({ search, onSearch, searchLabel, children }: { search?: string; onSearch?: (value: string) => void; searchLabel: string; children?: ReactNode }) {
  return <div className="flex flex-wrap items-center gap-2">
    {onSearch && <div className="relative min-w-56 flex-1">
      <Search aria-hidden="true" className="pointer-events-none absolute start-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input type="search" aria-label={searchLabel} placeholder={searchLabel} value={search ?? ""} onChange={(event) => onSearch(event.target.value)} className="ps-8" />
    </div>}
    {children}
  </div>;
}

const TONES = {
  success: "bg-emerald-500/15 text-emerald-800 dark:text-emerald-300",
  warning: "bg-amber-500/20 text-amber-900 dark:text-amber-300",
  danger: "bg-red-500/15 text-red-800 dark:text-red-300",
  neutral: "bg-muted text-foreground",
} as const;

/** Status with a dot and a text label, so the state is never carried by colour alone. */
export function AdminStatus({ tone, label }: { tone: keyof typeof TONES; label: string }) {
  return <span className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium ${TONES[tone]}`}>
    <span aria-hidden="true" className="size-1.5 rounded-full bg-current" />{label}
  </span>;
}

/** Empty, loading and error states share one layout. */
export function AdminEmptyState({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return <div role="status" className="flex flex-col items-center gap-2 rounded-lg border border-dashed px-6 py-10 text-center">
    <p className="font-medium">{title}</p>
    {description && <p className="max-w-md text-sm text-muted-foreground">{description}</p>}
    {action}
  </div>;
}

/** Initials avatar for people in lists and pickers. */
export function InitialsAvatar({ name, size = "size-8" }: { name: string; size?: string }) {
  const initials = name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]).join("").toUpperCase() || "?";
  return <span aria-hidden="true" className={`grid ${size} shrink-0 place-items-center rounded-full bg-muted text-xs font-semibold`}>{initials}</span>;
}

/** Compact summary of counts with an optional usage bar (used for seats). */
export function AdminMetrics({ items, usage }: { items: { label: string; value: string | number }[]; usage?: { used: number; max: number; label: string } }) {
  const { t } = useTranslation();
  const percent = usage && usage.max > 0 ? Math.min(100, Math.round((usage.used / usage.max) * 100)) : 0;
  return <section aria-label={t("settings.users.capacity")} className="space-y-3 rounded-lg border p-4">
    {usage && <>
      <div className="flex items-baseline justify-between text-sm">
        <span className="font-medium">{usage.label}</span>
        <span className="text-muted-foreground">{usage.used} / {usage.max}</span>
      </div>
      <div role="progressbar" aria-label={usage.label} aria-valuemin={0} aria-valuemax={usage.max} aria-valuenow={usage.used} className="h-2 overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full bg-primary" style={{ width: `${percent}%` }} />
      </div>
    </>}
    <dl className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
      {items.map((item) => <div key={item.label}><dt className="text-muted-foreground">{item.label}</dt><dd className="text-lg font-semibold">{item.value}</dd></div>)}
    </dl>
  </section>;
}
