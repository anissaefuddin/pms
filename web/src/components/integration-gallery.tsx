import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { AdminStatus } from "@/components/admin/admin-ui";

// Integration Gallery: the integrations people can connect, then the ways to build one yourself.
// Each card only opens its section; the gallery stores nothing.
const PROVIDERS = [
  { id: "github", tile: "GH", tileClass: "bg-neutral-900 text-white", section: "connectors" },
  { id: "google", tile: "31", tileClass: "bg-indigo-600 text-white", section: "connectors" },
] as const;
const BUILDERS = [
  { id: "api", section: "api", adminOnly: false },
  { id: "webhooks", section: "webhooks", adminOnly: false },
  { id: "clients", section: "clients", adminOnly: true },
] as const;

export function IntegrationGallery({ admin, onOpen }: { admin: boolean; onOpen: (id: string) => void }) {
  const { t } = useTranslation();
  return <section className="space-y-6" aria-labelledby="integration-gallery-title">
    <div className="space-y-1">
      <h3 id="integration-gallery-title" className="text-base font-semibold">{t("apps.gallery.title")}</h3>
      <p className="text-sm text-muted-foreground">{t("apps.gallery.hint")}</p>
    </div>
    <ul className="grid gap-4 sm:grid-cols-2">
      {PROVIDERS.map((provider) => <li key={provider.id} className="flex flex-col gap-4 rounded-lg border p-4">
        <div className="flex items-start gap-3">
          <span aria-hidden="true" className={`grid size-10 shrink-0 place-items-center rounded-lg text-sm font-semibold ${provider.tileClass}`}>{provider.tile}</span>
          <div className="min-w-0 flex-1">
            <p className="font-medium">{t(`apps.gallery.providers.${provider.id}.name`)}</p>
            <p className="text-sm text-muted-foreground">{t(`apps.gallery.providers.${provider.id}.desc`)}</p>
          </div>
        </div>
        <div className="flex items-center justify-between gap-2">
          {admin ? <AdminStatus tone="neutral" label={t("apps.gallery.adminSetup")} /> : <span className="text-xs text-muted-foreground">{t("apps.gallery.adminOnly")}</span>}
          <Button type="button" variant="outline" disabled={!admin} aria-label={t("apps.gallery.configureNamed", { name: t(`apps.gallery.providers.${provider.id}.name`) })}
            onClick={() => onOpen(provider.section)}>{t("apps.gallery.configure")}</Button>
        </div>
      </li>)}
    </ul>
    <div className="space-y-3">
      <h4 className="text-sm font-semibold">{t("apps.gallery.buildTitle")}</h4>
      <ul className="grid gap-3 sm:grid-cols-3">
        {BUILDERS.filter((item) => admin || !item.adminOnly).map((item) => <li key={item.id} className="flex flex-col gap-3 rounded-lg border p-4">
          <div>
            <p className="font-medium">{t(`apps.gallery.builders.${item.id}.name`)}</p>
            <p className="text-sm text-muted-foreground">{t(`apps.gallery.builders.${item.id}.desc`)}</p>
          </div>
          <Button type="button" variant="outline" className="mt-auto" aria-label={t("apps.gallery.openNamed", { name: t(`apps.gallery.builders.${item.id}.name`) })}
            onClick={() => onOpen(item.section)}>{t("apps.gallery.open")}</Button>
        </li>)}
      </ul>
    </div>
  </section>;
}
