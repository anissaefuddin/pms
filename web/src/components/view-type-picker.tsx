import { useTranslation } from "react-i18next";
import { SUPPORTED_DEFAULT_VIEWS, type SupportedDefaultView } from "@/lib/create-item-validate";
import { cn } from "@/lib/utils";

export const VIEW_CHOICES = ["table", "board", "gantt", "calendar", "files", "chart"] as const;
export type ViewChoice = (typeof VIEW_CHOICES)[number];

// Ilustrasi 48x32 memakai currentColor (token warna lewat kelas text-*), tanpa hex.
function ViewIllustration({ view }: { view: ViewChoice }) {
  const svg = { viewBox: "0 0 48 32", "aria-hidden": true, className: "h-8 w-12 text-muted-foreground" } as const;
  switch (view) {
    case "table":
      return <svg {...svg}><rect x="2" y="3" width="44" height="26" rx="3" fill="none" stroke="currentColor" /><path d="M2 11h44M2 19h44M18 3v26" stroke="currentColor" /><rect x="5" y="13" width="10" height="4" fill="currentColor" className="text-primary" /></svg>;
    case "board":
      return <svg {...svg}>{[2, 18, 34].map((x) => <g key={x}><rect x={x} y="3" width="12" height="26" rx="2" fill="none" stroke="currentColor" /><rect x={x + 2} y="7" width="8" height="5" rx="1" fill="currentColor" className="text-primary" /><rect x={x + 2} y="15" width="8" height="5" rx="1" fill="currentColor" /></g>)}</svg>;
    case "gantt":
      return <svg {...svg}><path d="M2 4v24" stroke="currentColor" /><rect x="6" y="5" width="20" height="5" rx="2" fill="currentColor" className="text-primary" /><rect x="14" y="14" width="26" height="5" rx="2" fill="currentColor" /><rect x="10" y="23" width="16" height="5" rx="2" fill="currentColor" /></svg>;
    case "calendar":
      return <svg {...svg}><rect x="3" y="5" width="42" height="24" rx="3" fill="none" stroke="currentColor" /><path d="M3 12h42M16 5v24M29 5v24" stroke="currentColor" /><rect x="17" y="13" width="12" height="8" fill="currentColor" className="text-primary" /></svg>;
    case "files":
      return <svg {...svg}><path d="M3 8a3 3 0 0 1 3-3h10l3 3h23a3 3 0 0 1 3 3v15a3 3 0 0 1-3 3H6a3 3 0 0 1-3-3z" fill="currentColor" /><rect x="14" y="2" width="16" height="12" rx="1" fill="currentColor" className="text-primary" /></svg>;
    case "chart":
      return <svg {...svg}><path d="M3 29h42" stroke="currentColor" /><rect x="7" y="16" width="6" height="13" fill="currentColor" /><rect x="18" y="8" width="6" height="21" fill="currentColor" className="text-primary" /><rect x="29" y="12" width="6" height="17" fill="currentColor" /><rect x="40" y="4" width="2" height="25" fill="currentColor" /></svg>;
  }
}

/** Enam kartu view default. Chart adalah ringkasan jumlah item per status (ProjectChart), bukan builder report. */
export function ViewTypePicker({ value, onChange, disabled }: { value: SupportedDefaultView; onChange: (view: SupportedDefaultView) => void; disabled?: boolean }) {
  const { t } = useTranslation();
  return <div role="radiogroup" aria-label={t("items.view")} className="grid grid-cols-2 gap-2 sm:grid-cols-3">
    {SUPPORTED_DEFAULT_VIEWS.map((view) => {
      return <label key={view} className={cn(
        "flex cursor-pointer flex-col gap-2 rounded-lg border bg-card p-2.5 text-left text-sm transition-colors hover:border-ring/60",
        "has-checked:border-primary has-checked:ring-3 has-checked:ring-ring/30",
        "has-focus-visible:ring-3 has-focus-visible:ring-ring/50",
        "has-disabled:cursor-not-allowed has-disabled:opacity-60 has-disabled:hover:border-border"
      )}>
        <span className="flex items-center justify-between gap-2">
          <ViewIllustration view={view} />
          <input type="radio" name="default_view_type" value={view} className="sr-only"
            checked={value === view} disabled={disabled}
            onChange={() => onChange(view)} />
        </span>
        <span className="font-medium text-foreground">{t(`views.types.${view}`)}</span>
        <span className="text-xs text-muted-foreground">{t(`items.viewDesc.${view}`)}</span>
      </label>;
    })}
  </div>;
}
