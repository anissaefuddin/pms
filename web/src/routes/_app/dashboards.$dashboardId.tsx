import { useState, type FormEvent } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronDown, MoreHorizontal } from "lucide-react";
import { useTranslation } from "react-i18next";
import { ApiError, api } from "@/lib/api";
import { LISTS, TEMPLATES, WIDGET_CATEGORIES, WIDGET_TYPES, isChart, type Dashboard, type Widget, type WidgetData, type WidgetType } from "@/lib/dashboards";
import { cleanQuery, defaultQuery, type AnalyticsQuery, type AnalyticsResult } from "@/lib/reports";
import { useDebounced, useSpaces } from "@/lib/space-queries";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { FormError } from "@/components/auth/form-error";
import { DashboardLinkDialog } from "@/components/dashboard-link-dialog";
import { QueryEditor } from "@/components/query-editor";
import { WidgetBody, type WidgetConfig } from "@/components/widget-card";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/dashboards/$dashboardId")({ component: DashboardPage });

const PIE_STYLES = ["classic", "donut", "ring"] as const;
const select = "mt-1 block h-9 rounded-md border border-input bg-background px-2";
const widgetPath = (dashboardId: string, widgetId?: string) => `/api/v1/dashboards/${dashboardId}/widgets${widgetId ? `/${widgetId}` : ""}`;

function DashboardPage() {
  const { t } = useTranslation();
  const { dashboardId } = Route.useParams();
  const qc = useQueryClient();
  const spaces = useSpaces().data ?? [];
  const board = useQuery({ queryKey: ["dashboard", dashboardId], retry: false, queryFn: () => api<Dashboard>(`/api/v1/dashboards/${dashboardId}`) });
  const [editing, setEditing] = useState<Widget | "new">();
  const [picking, setPicking] = useState(false);
  const [sharing, setSharing] = useState(false);
  const [newType, setNewType] = useState<WidgetType>("task_list");
  const [deleting, setDeleting] = useState<Widget>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const data = board.data;
  if (board.isPending) return <main className="p-8"><p role="status" className="text-sm text-muted-foreground">{t("dashboards.board.loading")}</p></main>;
  if (board.isError || !data) return <main className="space-y-3 p-8"><p role="alert">{t("dashboards.board.notFound")}</p>
    <Link to="/dashboards" className="underline">{t("dashboards.board.back")}</Link></main>;
  const space = spaces.find((s) => s.id === data.space_id);
  const canEdit = !data.space_id || !!space?.my_capabilities?.includes("edit");
  const widgets = [...data.widgets].sort((a, b) => a.position - b.position || a.id.localeCompare(b.id));

  /** Every widget change bumps the dashboard version; the answer is the new dashboard. */
  async function change(path: string, method: "POST" | "PUT" | "PATCH" | "DELETE", version: number, body?: unknown) {
    const next = await api<Dashboard>(path, { method, headers: { "If-Match": `"${version}"` }, body: body === undefined ? undefined : JSON.stringify(body) });
    qc.setQueryData(["dashboard", dashboardId], next);
    return next;
  }
  const failure = (err: unknown) => setError(err instanceof ApiError && err.status === 409 ? t("dashboards.conflict") : (err as Error).message || t("dashboards.actionFailed"));

  async function remove() {
    if (!deleting) return;
    setBusy(true); setError("");
    try { await change(widgetPath(dashboardId, deleting.id), "DELETE", data!.version); setDeleting(undefined); }
    catch (err) { failure(err); }
    finally { setBusy(false); }
  }

  async function layout(next: "columns" | "stack") {
    setError("");
    try { await change(`/api/v1/dashboards/${dashboardId}`, "PATCH", data!.version, { layout: next }); }
    catch (err) { failure(err); }
  }

  /** Swaps a widget with its neighbour by exchanging their positions (renumbered when they tie). */
  async function shift(widget: Widget, by: -1 | 1) {
    const at = widgets.findIndex((w) => w.id === widget.id), other = widgets[at + by];
    if (!other) return;
    setError("");
    try {
      let version = data!.version;
      for (const [w, position] of [[widget, at + by], [other, at]] as const) {
        const next = await change(widgetPath(dashboardId, w.id), "PUT", version, { name: w.name, type: w.type, position, config: w.config });
        version = next.version;
      }
    } catch (err) { failure(err); }
  }

  return <main className="space-y-5 p-8">
    <header className="space-y-2 border-b pb-5">
      <Link to="/dashboards" className="text-sm underline">{t("dashboards.board.back")}</Link>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <DashboardTitle current={data} />
        <div className="flex flex-wrap items-center gap-2">
          {canEdit && <LayoutPicker value={data.layout} onChange={(next) => void layout(next)} />}
          {canEdit && <Button variant="outline" onClick={() => setSharing(true)}>{t("dashboards.board.share")}</Button>}
          {canEdit && <Button onClick={() => setPicking(true)}>{t("dashboards.board.add")}</Button>}
        </div>
      </div>
      <p className="text-sm text-muted-foreground">{space ? t("dashboards.board.location", { space: space.name }) : data.space_id ? "" : t("dashboards.board.personal")}</p>
      {!canEdit && <p className="text-sm text-muted-foreground">{t("dashboards.board.readOnly")}</p>}
    </header>
    {error && !editing && !deleting && <p role="alert" className="text-sm text-destructive">{error}</p>}
    {widgets.length === 0 && <p className="rounded-xl border border-dashed bg-card p-12 text-center text-sm text-muted-foreground">{t("dashboards.board.empty")}</p>}
    <div className={data.layout === "stack" ? "grid gap-4" : "grid gap-4 lg:grid-cols-2"}>
      {widgets.map((widget, index) => <WidgetCard key={widget.id} dashboardId={dashboardId} version={data.version} widget={widget} canEdit={canEdit}
        first={index === 0} last={index === widgets.length - 1} onEdit={() => { setError(""); setEditing(widget); }} onDelete={() => { setError(""); setDeleting(widget); }} onShift={(by) => void shift(widget, by)} />)}
    </div>
    <DashboardLinkDialog dashboardId={dashboardId} open={sharing} onOpenChange={setSharing} />
    <WidgetPicker open={picking} onClose={() => setPicking(false)} onPick={(type) => { setPicking(false); setNewType(type); setError(""); setEditing("new"); }} />
    {editing && <WidgetEditor key={editing === "new" ? "new" : editing.id} initial={editing === "new" ? undefined : editing} startType={newType} onClose={() => setEditing(undefined)}
      onSave={async (input) => { await change(widgetPath(dashboardId, editing === "new" ? undefined : editing.id), editing === "new" ? "POST" : "PUT", data.version,
        { ...input, position: editing === "new" ? widgets.length : editing.position }); setEditing(undefined); }} />}
    <ConfirmDialog open={!!deleting} onOpenChange={(open) => !open && setDeleting(undefined)} busy={busy} error={error}
      title={t("dashboards.widget.deleteTitle")} description={t("dashboards.widget.deleteDescription", { name: deleting?.name ?? "" })}
      confirmLabel={t("dashboards.widget.deleteConfirm")} cancelLabel={t("dashboards.delete.cancel")} onConfirm={() => void remove()} />
  </main>;
}

/** The title is a menu that jumps to another dashboard in the same place (personal or the Space). */
function DashboardTitle({ current }: { current: Dashboard }) {
  const { t } = useTranslation();
  const list = useQuery({
    queryKey: ["dashboards", current.space_id ?? ""], retry: false,
    queryFn: async () => (await api<{ items: { id: string; name: string }[] }>(`/api/v1/dashboards${current.space_id ? `?space_id=${current.space_id}` : ""}`)).items,
  });
  return <DropdownMenu>
    <DropdownMenuTrigger render={<Button variant="ghost" className="workspace-title h-auto gap-2 px-2" aria-label={t("dashboards.board.switch", { name: current.name })} />}>
      <h1 className="workspace-title">{current.name}</h1><ChevronDown aria-hidden />
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start">
      {(list.data ?? []).map((entry) => <DropdownMenuItem key={entry.id} render={<Link to="/dashboards/$dashboardId" params={{ dashboardId: entry.id }} />}>{entry.name}</DropdownMenuItem>)}
      <DropdownMenuItem render={<Link to="/dashboards" />}>{t("dashboards.board.all")}</DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>;
}

const layoutIcon = {
  stack: <svg viewBox="0 0 32 24" className="h-6 w-8" aria-hidden><rect x="2" y="2" width="28" height="8" rx="1.5" fill="currentColor" opacity=".5" /><rect x="2" y="14" width="28" height="8" rx="1.5" fill="currentColor" opacity=".5" /></svg>,
  columns: <svg viewBox="0 0 32 24" className="h-6 w-8" aria-hidden><rect x="2" y="2" width="12" height="20" rx="1.5" fill="currentColor" opacity=".5" /><rect x="18" y="2" width="12" height="20" rx="1.5" fill="currentColor" opacity=".5" /></svg>,
};

const layoutKeys = ["stack", "columns"] as const;

function LayoutPicker({ value, onChange }: { value: "columns" | "stack"; onChange: (next: "columns" | "stack") => void }) {
  const { t } = useTranslation();
  return <Popover>
    <PopoverTrigger render={<Button variant="outline" aria-label={t("dashboards.board.layout")} />}>{layoutIcon[value]}{t("dashboards.board.layout")}</PopoverTrigger>
    <PopoverContent align="end" className="flex w-auto gap-3">
      {layoutKeys.map((key) => <button key={key} type="button" aria-pressed={value === key} onClick={() => onChange(key)}
        className="flex flex-col items-center gap-1 rounded-lg border p-3 text-sm hover:bg-muted aria-pressed:border-primary aria-pressed:ring-1 aria-pressed:ring-primary focus-visible:outline-2 focus-visible:outline-ring">
        {layoutIcon[key]}{t(key === "stack" ? "dashboards.board.layoutVertical" : "dashboards.board.layoutHorizontal")}</button>)}
    </PopoverContent>
  </Popover>;
}

function WidgetCard({ dashboardId, version, widget, canEdit, first, last, onEdit, onDelete, onShift }: {
  dashboardId: string; version: number; widget: Widget; canEdit: boolean; first: boolean; last: boolean;
  onEdit: () => void; onDelete: () => void; onShift: (by: -1 | 1) => void;
}) {
  const { t } = useTranslation();
  const data = useQuery({
    queryKey: ["widget-data", dashboardId, widget.id, version], retry: false,
    queryFn: () => api<WidgetData>(`/api/v1/dashboards/${dashboardId}/widgets/${widget.id}/data`),
  });
  return <section aria-labelledby={`w-${widget.id}`} className="rounded-xl border bg-card p-4">
    <header className="mb-3 flex items-start justify-between gap-2">
      <div className="min-w-0">
        <h2 id={`w-${widget.id}`} className="text-base font-semibold">{widget.name}</h2>
        {(widget.config as WidgetConfig).description && <p className="text-sm text-muted-foreground">{(widget.config as WidgetConfig).description}</p>}
      </div>
      {canEdit && <DropdownMenu>
        <DropdownMenuTrigger render={<Button size="sm" variant="ghost" aria-label={t("dashboards.widget.actions", { name: widget.name })} />}><MoreHorizontal /></DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={onEdit}>{t("dashboards.widget.edit")}</DropdownMenuItem>
          {!first && <DropdownMenuItem onClick={() => onShift(-1)}>{t("dashboards.widget.earlier")}</DropdownMenuItem>}
          {!last && <DropdownMenuItem onClick={() => onShift(1)}>{t("dashboards.widget.later")}</DropdownMenuItem>}
          <DropdownMenuItem onClick={onDelete}>{t("dashboards.widget.delete")}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>}
    </header>
    <WidgetBody type={widget.type} config={widget.config as WidgetConfig} data={data.data} pending={data.isPending} error={data.error} />
  </section>;
}

type Input = { name: string; type: WidgetType; config: WidgetConfig };

/** Add or edit one widget. Chart, indicator and table widgets show a live preview that runs the draft query. */
function WidgetEditor({ initial, startType, onClose, onSave }: { initial?: Widget; startType: WidgetType; onClose: () => void; onSave: (input: Input) => Promise<void> }) {
  const { t } = useTranslation();
  const start = (initial?.config ?? {}) as WidgetConfig;
  const [name, setName] = useState(initial?.name ?? "");
  const [type, setType] = useState<WidgetType>(initial?.type ?? startType);
  const [source, setSource] = useState(start.list ? `list:${start.list}` : start.template ? `template:${start.template}` : `list:${LISTS[0]}`);
  const [query, setQuery] = useState<AnalyticsQuery>(start.query ?? { ...defaultQuery("tasks"), group_by: "status_group" });
  const [legend, setLegend] = useState(start.show_legend ?? true);
  const [labels, setLabels] = useState(start.show_labels ?? false);
  const [description, setDescription] = useState(start.description ?? "");
  const [pie, setPie] = useState<NonNullable<WidgetConfig["pie_style"]>>(start.pie_style ?? "classic");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const listLike = type === "task_list";
  const draft = useDebounced(JSON.stringify(cleanQuery(query)), 400);
  const preview = useQuery({
    queryKey: ["widget-preview", draft], enabled: !listLike && type !== "burndown", retry: false,
    queryFn: async () => ({ kind: "series", result: await api<AnalyticsResult>("/api/v1/analytics/query", { method: "POST", body: draft }) }) as WidgetData,
  });
  const config = (): WidgetConfig => {
    const note = description.trim() ? { description: description.trim() } : {};
    if (listLike) return { ...(source.startsWith("list:") ? { list: source.slice(5) } : { template: source.slice(9) }), ...note };
    return { ...note, query: cleanQuery(query), ...(isChart(type) ? { show_legend: legend, show_labels: labels } : {}), ...(type === "pie" ? { pie_style: pie } : {}) };
  };
  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true); setError("");
    try { await onSave({ name, type, config: config() }); }
    catch (err) { setError(err instanceof ApiError && err.status === 409 ? t("dashboards.conflict") : (err as Error).message || t("dashboards.actionFailed")); }
    finally { setBusy(false); }
  }
  const [tab, setTab] = useState<"data" | "design" | "filters">("data");
  const tabs = ["data", "design", "filters"] as const;
  const previewBody = listLike || type === "burndown"
    ? <p className="text-sm text-muted-foreground">{t(listLike ? "dashboards.editor.listPreview" : "dashboards.editor.burndownHint")}</p>
    : <WidgetBody type={type} config={{ query: cleanQuery(query), show_legend: legend, show_labels: labels, pie_style: pie }} data={preview.data} pending={preview.isPending} error={preview.error} />;
  return <Dialog open onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-5xl">
      <form onSubmit={(e) => void submit(e)} className="space-y-4">
        <DialogHeader><DialogTitle>{t(initial ? "dashboards.editor.editTitle" : "dashboards.editor.addTitle")}</DialogTitle></DialogHeader>
        <div className="grid gap-4 lg:grid-cols-2">
          <section aria-labelledby="widget-preview" className="space-y-2 self-start rounded-lg border p-4">
            <h3 id="widget-preview" className="text-sm font-semibold">{t("dashboards.editor.preview")}</h3>
            <p className="text-sm text-muted-foreground">{t("dashboards.editor.previewHint")}</p>
            {previewBody}
          </section>
          <div className="min-w-0 space-y-3">
            <div role="tablist" aria-label={t("dashboards.editor.tabs")} className="flex gap-1 border-b"
              onKeyDown={(e) => {
                const move = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
                if (!move) return;
                const next = tabs[(tabs.indexOf(tab) + move + tabs.length) % tabs.length];
                setTab(next);
                document.getElementById(`widget-tab-${next}`)?.focus();
              }}>
              {tabs.map((key) => <button key={key} id={`widget-tab-${key}`} type="button" role="tab" aria-selected={tab === key} aria-controls={`widget-panel-${key}`}
                tabIndex={tab === key ? 0 : -1} onClick={() => setTab(key)}
                className="-mb-px border-b-2 border-transparent px-3 py-2 text-sm aria-selected:border-primary aria-selected:font-semibold focus-visible:outline-2 focus-visible:outline-ring">{t(`dashboards.editor.tab.${key}`)}</button>)}
            </div>
            <div role="tabpanel" id={`widget-panel-${tab}`} aria-labelledby={`widget-tab-${tab}`} className="space-y-4">
              {tab === "data" && <>
                <div className="flex flex-wrap gap-4">
                  <label className="block text-sm">{t("dashboards.editor.name")}
                    <Input required maxLength={120} value={name} onChange={(e) => setName(e.target.value)} className="mt-1 w-64" /></label>
                  <label className="block text-sm">{t("dashboards.editor.type")}
                    <select className={select} value={type} onChange={(e) => setType(e.target.value as WidgetType)}>
                      {WIDGET_TYPES.map((value) => <option key={value} value={value}>{t(`dashboards.type.${value}`)}</option>)}
                    </select></label>
                  <label className="block text-sm">{t("dashboards.editor.description")}
                    <Input maxLength={500} value={description} onChange={(e) => setDescription(e.target.value)} className="mt-1 w-full sm:w-96" /></label>
                  {listLike && <label className="block text-sm">{t("dashboards.editor.list")}
                    <select className={select} value={source} onChange={(e) => setSource(e.target.value)}>
                      <optgroup label={t("dashboards.editor.listGroup")}>{LISTS.map((value) => <option key={value} value={`list:${value}`}>{t(`dashboards.list.${value}`)}</option>)}</optgroup>
                      <optgroup label={t("dashboards.editor.templateGroup")}>{TEMPLATES.map((value) => <option key={value} value={`template:${value}`}>{t(`dashboards.template.${value}`)}</option>)}</optgroup>
                    </select></label>}
                </div>
                {!listLike && <QueryEditor query={query} setQuery={setQuery} part="data" />}
              </>}
              {tab === "design" && (isChart(type) ? <fieldset className="space-y-2 rounded-lg border p-4"><legend className="px-1 text-sm font-semibold">{t("dashboards.editor.options")}</legend>
                <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="size-6" checked={legend} onChange={(e) => setLegend(e.target.checked)} />{t("dashboards.editor.legend")}</label>
                <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="size-6" checked={labels} onChange={(e) => setLabels(e.target.checked)} />{t("dashboards.editor.labels")}</label>
                {type === "pie" && <label className="block text-sm">{t("dashboards.editor.pieStyle")}
                  <select className={select} value={pie} onChange={(e) => setPie(e.target.value as typeof pie)}>
                    {PIE_STYLES.map((value) => <option key={value} value={value}>{t(`dashboards.pie.${value}`)}</option>)}
                  </select></label>}
              </fieldset> : <p className="text-sm text-muted-foreground">{t("dashboards.editor.noDesign")}</p>)}
              {tab === "filters" && (listLike ? <p className="text-sm text-muted-foreground">{t("dashboards.editor.noFilters")}</p> : <QueryEditor query={query} setQuery={setQuery} part="filters" />)}
            </div>
          </div>
        </div>
        <FormError message={error} />
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>{t("dashboards.dialog.cancel")}</Button>
          <Button type="submit" disabled={busy || !name.trim()}>{t("dashboards.editor.save")}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>;
}

/** "Choose widget": the kinds grouped by what they show (DASH-08). */
function WidgetPicker({ open, onClose, onPick }: { open: boolean; onClose: () => void; onPick: (type: WidgetType) => void }) {
  const { t } = useTranslation();
  return <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
    <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
      <DialogHeader><DialogTitle>{t("dashboards.picker.title")}</DialogTitle></DialogHeader>
      <p className="text-sm text-muted-foreground">{t("dashboards.picker.hint")}</p>
      {WIDGET_CATEGORIES.map((category) => <section key={category.key} aria-label={t(`dashboards.picker.category.${category.key}`)} className="space-y-2">
        <h3 className="text-sm font-semibold">{t(`dashboards.picker.category.${category.key}`)}</h3>
        <ul className="grid gap-2 sm:grid-cols-2">{category.types.map((type) => <li key={type}>
          <button type="button" onClick={() => onPick(type)} className="w-full rounded-lg border p-3 text-left hover:bg-muted focus-visible:outline-ring">
            <span className="block font-medium">{t(`dashboards.type.${type}`)}</span>
            <span className="block text-sm text-muted-foreground">{t(`dashboards.picker.desc.${type}`)}</span>
          </button></li>)}</ul>
      </section>)}
    </DialogContent>
  </Dialog>;
}
