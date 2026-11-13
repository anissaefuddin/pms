import { useEffect, useState, type FormEvent } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { ApiError, api } from "@/lib/api";
import { WidgetBody, type WidgetConfig } from "@/components/widget-card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { WidgetData, WidgetType } from "@/lib/dashboards";

export const Route = createFileRoute("/d/$token")({ component: PublicDashboardPage });
type Board = components["schemas"]["PublicDashboard"];

function PublicDashboardPage() {
  const { t } = useTranslation();
  const { token } = Route.useParams();
  const [password, setPassword] = useState("");
  const view = useMutation({
    mutationFn: (pw: string) => api<Board>(`/api/v1/public/dashboards/${encodeURIComponent(token)}`, { method: "POST", body: JSON.stringify(pw ? { password: pw } : {}) }),
  });
  // The first load tries without a password; a 401 turns into the password form.
  const { mutate } = view;
  useEffect(() => { mutate(""); }, [mutate, token]);
  const needsPassword = view.error instanceof ApiError && view.error.status === 401;
  function submit(event: FormEvent) { event.preventDefault(); view.mutate(password); }
  if (view.data) return <main className="min-h-screen space-y-5 bg-background p-4 text-foreground sm:p-8">
    <header><p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{t("dashboards.public.badge")}</p>
      <h1 className="text-2xl font-semibold">{view.data.name}</h1></header>
    <div className={view.data.layout === "stack" ? "grid gap-4" : "grid gap-4 lg:grid-cols-2"}>
      {view.data.widgets.map((widget) => <section key={widget.id} aria-labelledby={`pw-${widget.id}`} className="rounded-xl border bg-card p-4">
        <h2 id={`pw-${widget.id}`} className="text-base font-semibold">{widget.name}</h2>
        {(widget.config as WidgetConfig).description && <p className="text-sm text-muted-foreground">{(widget.config as WidgetConfig).description}</p>}
        {widget.masked ? <p className="mt-2 text-sm text-muted-foreground">{t("dashboards.public.masked")}</p>
          : widget.failed ? <p role="alert" className="mt-2 text-sm text-destructive">{t("dashboards.widget.error")}</p>
            : <WidgetBody type={widget.type as WidgetType} config={widget.config as WidgetConfig} data={widget.data as WidgetData} pending={false} error={null} />}
      </section>)}
    </div>
  </main>;
  if (needsPassword) return <main className="mx-auto max-w-sm space-y-3 p-8">
    <h1 className="text-xl font-semibold">{t("dashboards.public.passwordTitle")}</h1>
    <form onSubmit={submit} className="space-y-3">
      <label className="block text-sm">{t("dashboards.link.password")}<Input type="password" required value={password} onChange={(e) => setPassword(e.target.value)} /></label>
      {password && view.isError && <p role="alert" className="text-sm text-destructive">{t("dashboards.public.wrongPassword")}</p>}
      <Button type="submit" disabled={view.isPending}>{t("dashboards.public.open")}</Button>
    </form>
  </main>;
  if (view.isError) return <main className="p-8"><h1 className="text-2xl font-semibold">{t("dashboards.public.unavailable")}</h1><p className="mt-2 text-muted-foreground">{t("dashboards.public.unavailableHint")}</p></main>;
  return <main className="p-8"><p role="status">{t("dashboards.public.loading")}</p></main>;
}
