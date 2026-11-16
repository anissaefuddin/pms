import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { AdminStatus } from "@/components/admin/admin-ui";

// Integration Gallery: the integrations people can connect, then the ways to build one yourself.
// Each card only opens its section; the gallery stores nothing.
// GitHub mark (Octicons, MIT): a black circle with the cat cut out, shown on a white tile as in GitHub's own logo.
// Google Calendar is the full-colour logo file.
const GITHUB_MARK = <svg viewBox="0 0 16 16" fill="currentColor" className="size-8" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" /></svg>;
const PROVIDERS = [
  { id: "github", logo: GITHUB_MARK, tileClass: "border bg-white text-neutral-900", section: "connectors" },
  { id: "google", logo: <img src="/brand/google-calendar.svg" alt="" className="size-10" />, tileClass: "", section: "connectors" },
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
      <h2 id="integration-gallery-title" className="text-base font-semibold">{t("apps.gallery.title")}</h2>
      <p className="text-sm text-muted-foreground">{t("apps.gallery.hint")}</p>
    </div>
    <ul className="grid gap-4 sm:grid-cols-2">
      {PROVIDERS.map((provider) => <li key={provider.id} className="flex flex-col gap-4 rounded-lg border p-4">
        <div className="flex items-start gap-3">
          <span aria-hidden="true" className={`grid size-10 shrink-0 place-items-center overflow-hidden rounded-lg ${provider.tileClass}`}>{provider.logo}</span>
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
