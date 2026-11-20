import type { ComponentType, SVGProps } from "react";
import { BarChart3, BookOpen, Briefcase, CalendarDays, FlaskConical, Folder, Globe, Heart, House, Layers, Megaphone, Rocket, Shield, Star, Users, Wrench } from "lucide-react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";

// Must match IconKeys in server/internal/spaces/governance.go (D-04): the server rejects any other key.
const ICONS: Record<string, ComponentType<SVGProps<SVGSVGElement>>> = {
  folder: Folder, briefcase: Briefcase, rocket: Rocket, megaphone: Megaphone, users: Users, shield: Shield, chart: BarChart3,
  calendar: CalendarDays, flask: FlaskConical, wrench: Wrench, book: BookOpen, globe: Globe, heart: Heart, star: Star, home: House, layers: Layers,
};

export const SPACE_ICON_KEYS = Object.keys(ICONS);

/** A Space's icon; a Space without a key shows Layers. Decorative: the name next to it carries the meaning. */
export function SpaceIcon({ iconKey, className }: { iconKey?: string | null; className?: string }) {
  const Icon = (iconKey && ICONS[iconKey]) || Layers;
  return <Icon aria-hidden="true" className={cn("size-4 shrink-0", className)} />;
}

/** Visual choice of one icon from the catalogue. Empty means the default folder. */
export function SpaceIconPicker({ value, onChange, disabled }: { value: string | null; onChange: (key: string | null) => void; disabled?: boolean }) {
  const { t } = useTranslation();
  return (
    <fieldset disabled={disabled} className="space-y-2">
      <legend className="text-sm font-medium">{t("spaces.spaceSettings.general.icon")}</legend>
      <div role="group" aria-label={t("spaces.spaceSettings.general.icon")} className="grid grid-cols-8 gap-1.5">
        {SPACE_ICON_KEYS.map((key) => {
          const selected = (value ?? "layers") === key;
          return <button key={key} type="button" aria-pressed={selected} title={t(`spaces.spaceSettings.icons.${key}`)}
            aria-label={t(`spaces.spaceSettings.icons.${key}`)} onClick={() => onChange(key === "layers" ? null : key)}
            className={cn("grid size-9 place-items-center rounded-lg border text-foreground hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50",
              selected && "border-primary ring-2 ring-ring")}>
            <SpaceIcon iconKey={key} />
          </button>;
        })}
      </div>
    </fieldset>
  );
}
