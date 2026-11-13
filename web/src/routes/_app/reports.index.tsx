import { useState, type FormEvent } from "react";
import { Link, createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { MoreHorizontal } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import { defaultQuery, type Report, type ReportSummary } from "@/lib/reports";
import { useSpaces } from "@/lib/space-queries";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/reports/")({ component: Reports });

function Reports() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const profile = useProfile();
  const navigate = useNavigate();
  const list = useQuery({
    queryKey: ["reports"], retry: false,
    queryFn: async () => (await api<{ items: ReportSummary[] }>("/api/v1/reports")).items,
  });
  const spaces = (useSpaces().data ?? []).filter((space) => space.kind === "shared" && space.my_capabilities?.includes("create"));
  const [filter, setFilter] = useState("");
  const templates = useQuery({
    queryKey: ["report-templates"], retry: false, staleTime: Infinity,
    queryFn: async () => (await api<{ items: { key: string }[] }>("/api/v1/report-templates")).items,
  });
  const [template, setTemplate] = useState("");
  const [name, setName] = useState("");
  const [location, setLocation] = useState("");
  const [sharing, setSharing] = useState<"private" | "space">("private");
  const [deleting, setDeleting] = useState<ReportSummary>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const rows = (list.data ?? []).filter((r) => r.name.toLowerCase().includes(filter.trim().toLowerCase()));

  async function create(event: FormEvent) {
    event.preventDefault();
    setBusy(true); setError("");
    try {
      const report = await api<Report>("/api/v1/reports", { method: "POST", body: JSON.stringify({
        name, space_id: location || null, sharing: location ? sharing : "private",
        ...(template ? { template } : { definition: { layout: "table", query: defaultQuery("tasks") } }) }) });
      await qc.invalidateQueries({ queryKey: ["reports"] });
      await navigate({ to: "/reports/$reportId", params: { reportId: report.id } });
    } catch (err) { setError((err as Error).message || t("reports.create.failed")); }
    finally { setBusy(false); }
  }

  async function remove() {
    if (!deleting) return;
    setBusy(true); setError("");
    try {
      await api(`/api/v1/reports/${deleting.id}`, { method: "DELETE", headers: { "If-Match": `"${deleting.version}"` } });
      setDeleting(undefined);
      toast.success(t("reports.deleted"));
      await qc.invalidateQueries({ queryKey: ["reports"] });
    } catch (err) { setError((err as Error).message); }
    finally { setBusy(false); }
  }

  return <main className="space-y-5 p-8">
    <header className="border-b pb-5">
      <h1 className="workspace-title">{t("reports.title")}</h1>
      <p className="text-sm text-muted-foreground">{t("reports.description")}</p>
    </header>
    <form className="flex flex-wrap items-end gap-3" onSubmit={(e) => void create(e)} aria-label={t("reports.create.title")}>
      {!!templates.data?.length && <label className="block text-sm">{t("reports.create.template")}
        <select className="mt-1 block h-9 rounded-md border border-input bg-background px-2" value={template}
          onChange={(e) => { setTemplate(e.target.value); if (e.target.value && !name.trim()) setName(t(`reports.template.${e.target.value}`)); }}>
          <option value="">{t("reports.create.blank")}</option>
          {templates.data.map((tpl) => <option key={tpl.key} value={tpl.key}>{t(`reports.template.${tpl.key}`)}</option>)}
        </select></label>}
      <label className="block text-sm">{t("reports.create.name")}
        <Input required maxLength={120} value={name} onChange={(e) => setName(e.target.value)} className="mt-1 w-64" /></label>
      <label className="block text-sm">{t("reports.create.location")}
        <select className="mt-1 block h-9 rounded-md border border-input bg-background px-2" value={location} onChange={(e) => setLocation(e.target.value)}>
          <option value="">{t("reports.create.personal")}</option>
          {spaces.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
        </select></label>
      {location && <label className="block text-sm">{t("reports.create.sharing")}
        <select className="mt-1 block h-9 rounded-md border border-input bg-background px-2" value={sharing} onChange={(e) => setSharing(e.target.value as "private" | "space")}>
          <option value="private">{t("reports.sharing.private")}</option>
          <option value="space">{t("reports.sharing.space")}</option>
        </select></label>}
      <Button type="submit" disabled={busy || !name.trim()}>{t("reports.create.submit")}</Button>
    </form>
    {error && !deleting && <p role="alert" className="text-sm text-destructive">{error}</p>}
    {list.isPending && <p role="status" className="text-sm text-muted-foreground">{t("reports.loading")}</p>}
    {list.isError && <div><p role="alert">{t("reports.error")}</p><Button variant="outline" className="mt-3" onClick={() => list.refetch()}>{t("reports.retry")}</Button></div>}
    {list.data?.length === 0 && <section className="rounded-xl border border-dashed bg-card p-12 text-center">
      <h2 className="text-lg font-semibold">{t("reports.empty.title")}</h2>
      <p className="mx-auto mt-1 max-w-md text-sm text-muted-foreground">{t("reports.empty.description")}</p>
    </section>}
    {!!list.data?.length && <>
      <label className="block max-w-sm text-sm"><span className="sr-only">{t("reports.filter")}</span>
        <Input type="search" placeholder={t("reports.filter")} value={filter} onChange={(e) => setFilter(e.target.value)} /></label>
      {rows.length === 0 ? <p role="status" className="text-sm text-muted-foreground">{t("reports.noMatch")}</p> :
      <table className="w-full text-left text-sm">
        <thead><tr className="border-b">
          <th scope="col" className="py-2 pr-4">{t("reports.col.name")}</th>
          <th scope="col" className="py-2 pr-4">{t("reports.col.owner")}</th>
          <th scope="col" className="py-2 pr-4">{t("reports.col.space")}</th>
          <th scope="col" className="py-2 pr-4">{t("reports.col.sharing")}</th>
          <th scope="col" className="py-2 pr-4">{t("reports.col.created")}</th>
          <th scope="col" className="py-2 pr-4">{t("reports.col.updated")}</th>
          <th scope="col" className="py-2 pr-4">{t("reports.col.subscription")}</th>
          <th scope="col" className="py-2"><span className="sr-only">{t("reports.col.actions")}</span></th>
        </tr></thead>
        <tbody>{rows.map((report) => <tr key={report.id} className="border-b align-top">
          <th scope="row" className="py-2 pr-4 font-medium">
            <Link to="/reports/$reportId" params={{ reportId: report.id }} className="underline">{report.name}</Link></th>
          <td className="py-2 pr-4">{report.owner.name}</td>
          <td className="py-2 pr-4">{report.space ? <Link to="/spaces/$spaceId" params={{ spaceId: report.space.id }} className="underline">{report.space.name}</Link> : t("reports.personal")}</td>
          <td className="py-2 pr-4"><span className="rounded-full border px-2 py-0.5 text-xs">{t(`reports.sharing.${report.sharing}`)}</span></td>
          <td className="py-2 pr-4">{formatDate(report.created_at, profile)}</td>
          <td className="py-2 pr-4">{formatDate(report.updated_at, profile)}</td>
          <td className="py-2 pr-4">{report.subscription
            ? t("reports.subscription.weekly", { day: t(`reports.day.${report.subscription.weekday}`), hour: String(report.subscription.hour).padStart(2, "0") })
            : t("reports.subscription.none")}</td>
          <td className="py-2">
            <DropdownMenu>
              <DropdownMenuTrigger render={<Button size="sm" variant="outline" aria-label={t("reports.actions.named", { name: report.name })} />}><MoreHorizontal /></DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem render={<Link to="/reports/$reportId" params={{ reportId: report.id }} />}>{t("reports.actions.open")}</DropdownMenuItem>
                <DropdownMenuItem onClick={() => { setError(""); setDeleting(report); }}>{t("reports.actions.delete")}</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </td>
        </tr>)}</tbody>
      </table>}
    </>}
    <ConfirmDialog open={!!deleting} onOpenChange={(open) => !open && setDeleting(undefined)} busy={busy} error={error}
      title={t("reports.delete.title")} description={t("reports.delete.description", { name: deleting?.name ?? "" })}
      confirmLabel={t("reports.delete.confirm")} cancelLabel={t("reports.delete.cancel")} onConfirm={() => void remove()} />
  </main>;
}
