import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";

const GRID_CLASS = "grid gap-3 sm:grid-cols-2 lg:grid-cols-3";
const CARD_CLASS = "flex flex-col justify-between gap-3 rounded border p-4";
const NAME_CLASS = "font-medium";
const DESC_CLASS = "mt-1 text-sm text-muted-foreground";

// Read-only index of the Integration Center sections. Each card only switches to its section; it stores nothing itself.
export function IntegrationGallery({ sections, onOpen }: { sections: { id: string; label: string }[]; onOpen: (id: string) => void }) {
  const { t } = useTranslation();
  return <section className="space-y-4" aria-labelledby="integration-gallery-title">
    <h3 id="integration-gallery-title" className="font-medium">{t("apps.gallery.title")}</h3>
    <p className="text-sm text-muted-foreground">{t("apps.gallery.hint")}</p>
    <ul className={GRID_CLASS}>
      {sections.map((s) => <li key={s.id} className={CARD_CLASS}>
        <div>
          <p className={NAME_CLASS}>{s.label}</p>
          <p className={DESC_CLASS}>{t(`apps.gallery.desc.${s.id}`)}</p>
        </div>
        <Button type="button" variant="outline" aria-label={t("apps.gallery.openNamed", { name: s.label })} onClick={() => onOpen(s.id)}>{t("apps.gallery.open")}</Button>
      </li>)}
    </ul>
  </section>;
}
