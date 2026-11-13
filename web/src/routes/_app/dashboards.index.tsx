import { useState, type FormEvent } from "react";
import { Link, createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { MoreHorizontal } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import type { Dashboard, DashboardSummary } from "@/lib/dashboards";
import { useSpaces } from "@/lib/space-queries";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/dashboards/")({ component: Dashboards });

const MODES = ["rename", "duplicate", "move"] as const;
type Mode = (typeof MODES)[number];
const select = "mt-1 block h-9 rounded-md border border-input bg-background px-2";

function Dashboards() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const spaces = (useSpaces().data ?? []).filter((space) => space.kind === "shared");
  const [location, setLocation] = useState(""); // "" = my personal dashboards
  const list = useQuery({
    queryKey: ["dashboards", location], retry: false,
    queryFn: async () => (await api<{ items: DashboardSummary[] }>(`/api/v1/dashboards${location ? `?space_id=${location}` : ""}`)).items,
  });
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [edit, setEdit] = useState<{ mode: Mode; dashboard: DashboardSummary }>();
  const [deleting, setDeleting] = useState<DashboardSummary>();
  const refresh = () => qc.invalidateQueries({ queryKey: ["dashboards"] });
  const failure = (err: unknown) => setError(err instanceof ApiError && err.status === 409 ? t("dashboards.conflict") : (err as Error).message || t("dashboards.actionFailed"));

  async function create(event: FormEvent) {
    event.preventDefault();
    setBusy(true); setError("");
    try {
      const created = await api<Dashboard>("/api/v1/dashboards", { method: "POST", body: JSON.stringify({ name, space_id: location || null }) });
      await refresh();
      await navigate({ to: "/dashboards/$dashboardId", params: { dashboardId: created.id } });
    } catch (err) { failure(err); }
    finally { setBusy(false); }
  }

  async function remove() {
    if (!deleting) return;
    setBusy(true); setError("");
    try {
      await api(`/api/v1/dashboards/${deleting.id}`, { method: "DELETE", headers: { "If-Match": `"${deleting.version}"` } });
      setDeleting(undefined);
      toast.success(t("dashboards.deleted"));
      await refresh();
    } catch (err) { failure(err); }
    finally { setBusy(false); }
  }

  return <main className="space-y-5 p-8">
    <header className="border-b pb-5">
      <h1 className="text-2xl font-semibold">{t("dashboards.title")}</h1>
      <p className="text-sm text-muted-foreground">{t("dashboards.description")}</p>
    </header>
    <div className="flex flex-wrap items-end gap-3">
      <label className="block text-sm">{t("dashboards.scope.label")}
        <select className={select} value={location} onChange={(e) => setLocation(e.target.value)}>
          <option value="">{t("dashboards.scope.personal")}</option>
          {spaces.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
        </select></label>
      <form className="flex items-end gap-3" onSubmit={(e) => void create(e)} aria-label={t("dashboards.create.submit")}>
        <label className="block text-sm">{t("dashboards.create.name")}
          <Input required maxLength={120} value={name} onChange={(e) => setName(e.target.value)} className="mt-1 w-64" /></label>
        <Button type="submit" disabled={busy || !name.trim()}>{t("dashboards.create.submit")}</Button>
      </form>
    </div>
    {error && !edit && !deleting && <p role="alert" className="text-sm text-destructive">{error}</p>}
    {list.isPending && <p role="status" className="text-sm text-muted-foreground">{t("dashboards.loading")}</p>}
    {list.isError && <div><p role="alert">{(list.error as ApiError).status === 403 ? t("dashboards.actionFailed") : t("dashboards.error")}</p>
      <Button variant="outline" className="mt-3" onClick={() => list.refetch()}>{t("dashboards.retry")}</Button></div>}
    {list.data?.length === 0 && <section className="rounded-xl border border-dashed bg-card p-12 text-center">
      <h2 className="text-lg font-semibold">{t("dashboards.empty.title")}</h2>
      <p className="mx-auto mt-1 max-w-md text-sm text-muted-foreground">{t("dashboards.empty.description")}</p></section>}
    {!!list.data?.length && <table className="w-full text-left text-sm">
      <thead><tr className="border-b">
        <th scope="col" className="py-2 pr-4">{t("dashboards.col.name")}</th>
        <th scope="col" className="py-2"><span className="sr-only">{t("dashboards.col.actions")}</span></th>
      </tr></thead>
      <tbody>{list.data.map((dashboard) => <tr key={dashboard.id} className="border-b">
        <th scope="row" className="py-2 pr-4 font-medium"><Link to="/dashboards/$dashboardId" params={{ dashboardId: dashboard.id }} className="underline">{dashboard.name}</Link></th>
        <td className="py-2 text-right">
          <DropdownMenu>
            <DropdownMenuTrigger render={<Button size="sm" variant="outline" aria-label={t("dashboards.actions.named", { name: dashboard.name })} />}><MoreHorizontal /></DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem render={<Link to="/dashboards/$dashboardId" params={{ dashboardId: dashboard.id }} />}>{t("dashboards.actions.open")}</DropdownMenuItem>
              {MODES.map((mode) => <DropdownMenuItem key={mode} onClick={() => { setError(""); setEdit({ mode, dashboard }); }}>{t(`dashboards.actions.${mode}`)}</DropdownMenuItem>)}
              <DropdownMenuItem onClick={() => { setError(""); setDeleting(dashboard); }}>{t("dashboards.actions.delete")}</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </td>
      </tr>)}</tbody>
    </table>}
    {edit && <EditDialog key={`${edit.mode}:${edit.dashboard.id}`} {...edit} spaces={spaces.filter((s) => s.my_capabilities?.includes("create"))} onClose={() => setEdit(undefined)} onDone={refresh} />}
    <ConfirmDialog open={!!deleting} onOpenChange={(open) => !open && setDeleting(undefined)} busy={busy} error={error}
      title={t("dashboards.delete.title")} description={t("dashboards.delete.description", { name: deleting?.name ?? "" })}
      confirmLabel={t("dashboards.delete.confirm")} cancelLabel={t("dashboards.delete.cancel")} onConfirm={() => void remove()} />
  </main>;
}

/** Rename, duplicate (name and destination) or move (destination) one dashboard; each sends the version it was loaded at. */
function EditDialog({ mode, dashboard, spaces, onClose, onDone }: { mode: Mode; dashboard: DashboardSummary; spaces: { id: string; name: string }[]; onClose: () => void; onDone: () => Promise<unknown> }) {
  const { t } = useTranslation();
  const [name, setName] = useState(mode === "duplicate" ? `${dashboard.name} (copy)`.slice(0, 120) : dashboard.name);
  const [destination, setDestination] = useState(dashboard.space_id ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true); setError("");
    const id = `/api/v1/dashboards/${dashboard.id}`;
    const ifMatch = { "If-Match": `"${dashboard.version}"` };
    const space_id = destination || null;
    try {
      if (mode === "rename") await api(id, { method: "PATCH", headers: ifMatch, body: JSON.stringify({ name }) });
      if (mode === "duplicate") await api(`${id}/duplicate`, { method: "POST", body: JSON.stringify({ name, space_id }) });
      if (mode === "move") await api(`${id}/move`, { method: "POST", headers: ifMatch, body: JSON.stringify({ space_id }) });
      await onDone();
      onClose();
    } catch (err) { setError(err instanceof ApiError && err.status === 409 ? t("dashboards.conflict") : (err as Error).message || t("dashboards.actionFailed")); }
    finally { setBusy(false); }
  }
  return <Dialog open onOpenChange={(open) => !open && onClose()}>
    <DialogContent>
      <form onSubmit={(e) => void submit(e)} className="space-y-4">
        <DialogHeader><DialogTitle>{t(`dashboards.${mode}.title`)}</DialogTitle></DialogHeader>
        {mode !== "move" && <label className="block text-sm">{t(mode === "rename" ? "dashboards.create.name" : "dashboards.duplicate.name")}
          <Input required maxLength={120} value={name} onChange={(e) => setName(e.target.value)} className="mt-1" /></label>}
        {mode !== "rename" && <label className="block text-sm">{t(mode === "move" ? "dashboards.move.destination" : "dashboards.duplicate.destination")}
          <select className={select} value={destination} onChange={(e) => setDestination(e.target.value)}>
            <option value="">{t("dashboards.scope.personal")}</option>
            {spaces.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
          </select></label>}
        <FormError message={error} />
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>{t("dashboards.dialog.cancel")}</Button>
          <Button type="submit" disabled={busy || (mode !== "move" && !name.trim())}>{t(`dashboards.${mode}.submit`)}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>;
}
