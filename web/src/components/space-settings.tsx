import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { Link, useBlocker, useNavigate } from "@tanstack/react-router";
import { LayoutTemplate } from "lucide-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api, type CurrentUser } from "@/lib/api";
import { SHARE_ROLES, can, etag, spaceKeys, type Group, type Space } from "@/lib/spaces";
import { Search } from "@/components/share-dialog";
import { cn } from "@/lib/utils";
import { WorkspaceHeader } from "@/components/workspace-header";
import { WorkspaceLayout } from "@/components/workspace-layout";
import { SettingsRow, SettingsSaveBar, SettingsSection } from "@/components/settings/settings-ui";
import { ItemTypesPanel } from "@/routes/_app/settings/item-types";
import { useSpace } from "@/lib/space-queries";
import { LoadingState } from "@/components/ui/loading-state";
import { ErrorState } from "@/components/ui/error-state";
import { SpaceAutomationPanel } from "@/components/space-automation";
import { FormsList } from "@/routes/_app/spaces_.$spaceId.forms";
import { CustomFieldRegistry } from "@/components/custom-field-registry";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { FormError } from "@/components/auth/form-error";
import { Button, buttonVariants } from "@/components/ui/button";
import { SpaceIconPicker } from "@/components/space-icon";
import { Input } from "@/components/ui/input";
import { EmptyState } from "@/components/ui/empty-state";

// Tab slugs are the URL contract: /spaces/:spaceId/settings/:tab. Labels are translated below.
export const SPACE_SETTINGS_TABS = ["general", "members", "tools", "item-types", "custom-fields", "workflows", "automation", "forms", "blueprints"] as const;
export type SpaceSettingsTab = (typeof SPACE_SETTINGS_TABS)[number];

const TAB_LABEL = {
  general: "spaces.spaceSettings.tab.general",
  members: "spaces.spaceSettings.tab.members",
  tools: "spaces.spaceSettings.tab.tools",
  "item-types": "spaces.spaceSettings.tab.itemTypes",
  "custom-fields": "spaces.spaceSettings.tab.customFields",
  workflows: "spaces.spaceSettings.tab.workflows",
  automation: "spaces.spaceSettings.tab.automation",
  forms: "spaces.spaceSettings.tab.forms",
  blueprints: "spaces.spaceSettings.tab.blueprints",
} as const satisfies Record<SpaceSettingsTab, string>;

// Navigation groups for the Space settings sidebar. Every tab belongs to exactly one group.
const SPACE_SETTINGS_GROUPS: { id: string; label?: string; tabs: SpaceSettingsTab[] }[] = [
  { id: "general", tabs: ["general"] },
  { id: "access", label: "spaces.spaceSettings.group.access", tabs: ["members"] },
  { id: "configuration", label: "spaces.spaceSettings.group.configuration", tabs: ["item-types", "custom-fields", "workflows"] },
  { id: "tools", label: "spaces.spaceSettings.group.toolsAutomation", tabs: ["tools", "automation", "forms", "blueprints"] },
];

export function isSpaceSettingsTab(value: string): value is SpaceSettingsTab {
  return (SPACE_SETTINGS_TABS as readonly string[]).includes(value);
}

/** Full Space Settings page. The Space header gear, the sidebar gear and direct links all open this. */
/** Where the Space settings are opened from: the Workspace registry (/settings/spaces) or the Space work area. Same content, different URLs and back link. */
export type SpaceSettingsContext = "workspace" | "space";
const TAB_PATH = { workspace: "/settings/spaces/$spaceId/$tab", space: "/spaces/$spaceId/settings/$tab" } as const;

/** Loads the Space for either settings route and handles loading, not found and error the same way. */
export function SpaceSettingsPage({ spaceId, tab, context }: { spaceId: string; tab: SpaceSettingsTab; context: SpaceSettingsContext }) {
  const { t } = useTranslation();
  const space = useSpace(spaceId);
  const queryClient = useQueryClient();
  const me = queryClient.getQueryData<CurrentUser>(["me"]);
  if (space.isPending) return <main className="p-8"><LoadingState label={t("spaces.page.loading")} /></main>;
  if (space.error instanceof ApiError && space.error.status === 404) {
    return <main className="p-8"><div className="max-w-md space-y-3">
      <h1 className="workspace-title">{t("spaces.notFound.title")}</h1>
      <p className="text-muted-foreground">{t("spaces.notFound.description")}</p>
      {me && <Link to="/spaces/$spaceId" params={{ spaceId: me.personal_space_id }} className={buttonVariants({ variant: "outline" })}>{t("spaces.notFound.home")}</Link>}
    </div></main>;
  }
  if (space.isError) return <main className="p-8"><ErrorState message={t("spaces.page.error")} retryLabel={t("spaces.page.retry")} onRetry={() => void space.refetch()} /></main>;
  return <SpaceSettings space={space.data} me={me} tab={tab} context={context} />;
}

export function SpaceSettings({ space, me, tab, context = "space" }: { space: Space; me?: CurrentUser; tab: SpaceSettingsTab; context?: SpaceSettingsContext }) {
  const { t } = useTranslation();
  const [dirty, setDirty] = useState(false);
  // Leaving the page or switching tab with unsaved General changes asks first.
  const blocker = useBlocker({ shouldBlockFn: () => dirty, enableBeforeUnload: dirty, withResolver: true });
  const personal = space.kind === "personal";
  return (
    <WorkspaceLayout>
      {/* Breadcrumb: shows that this is one Space's settings, not the Workspace's. The header moves it into the top bar. */}
      <WorkspaceHeader
        breadcrumb={
        <nav aria-label={t("spaces.spaceSettings.breadcrumb")} className="mb-2 text-sm text-muted-foreground">
          <ol className="flex flex-wrap items-center gap-1">
            <li><Link to="/settings/spaces" className="underline-offset-2 hover:underline">{t("spaces.spaceSettings.crumbSpaces")}</Link> <span aria-hidden="true">/</span></li>
            {context === "workspace"
              ? <li><Link to="/settings/spaces/$spaceId/$tab" params={{ spaceId: space.id, tab: "general" }} className="underline-offset-2 hover:underline">{personal ? t("spaces.personal") : space.name}</Link> <span aria-hidden="true">/</span></li>
              : <>
                <li><Link to="/spaces/$spaceId" params={{ spaceId: space.id }} className="underline-offset-2 hover:underline">{personal ? t("spaces.personal") : space.name}</Link> <span aria-hidden="true">/</span></li>
                <li>{t("spaces.spaceSettings.crumbSettings")} <span aria-hidden="true">/</span></li>
              </>}
            <li aria-current="page" className="text-foreground">{t(TAB_LABEL[tab])}</li>
          </ol>
        </nav>
        }
        title={t("spaces.spaceSettings.title")}
        actions={context === "workspace"
          ? <Link to="/settings/spaces" className={buttonVariants({ variant: "outline" })}>{t("spaces.spaceSettings.backAll")}</Link>
          : <Link to="/spaces/$spaceId" params={{ spaceId: space.id }} className={buttonVariants({ variant: "outline" })}>{t("spaces.spaceSettings.backSpace")}</Link>}
        meta={<p className="mt-1 text-sm text-muted-foreground">
          <Link to="/spaces/$spaceId" params={{ spaceId: space.id }} className="underline-offset-2 hover:underline">{personal ? t("spaces.personal") : space.name}</Link>
        </p>}
      />
      {/* Grouped: a row of tabs on small screens (scrolls), a side list with group headings from lg up. */}
      <div className="mt-4 grid gap-6 lg:grid-cols-[12rem_minmax(0,1fr)]">
        <nav aria-label={t("spaces.spaceSettings.navLabel")} className="flex gap-1 overflow-x-auto border-b lg:flex-col lg:gap-4 lg:overflow-visible lg:border-b-0 lg:border-r lg:pe-4">
          {SPACE_SETTINGS_GROUPS.map((group) => (
            <div key={group.id} className="flex gap-1 lg:flex-col lg:gap-0.5">
              {group.label && <p className="hidden px-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground lg:block">{t(group.label)}</p>}
              {group.tabs.map((key) => (
                <Link key={key} to={TAB_PATH[context]} params={{ spaceId: space.id, tab: key }}
                  aria-current={key === tab ? "page" : undefined}
                  className={cn("shrink-0 border-b-2 px-3 py-2 text-sm text-muted-foreground hover:text-foreground lg:border-b-0 lg:border-s-2 lg:rounded-e-md",
                    key === tab ? "border-primary font-medium text-foreground lg:bg-muted" : "border-transparent")}>
                  {t(TAB_LABEL[key])}
                </Link>
              ))}
            </div>
          ))}
        </nav>
        <div className="min-w-0 max-w-3xl">
        {tab === "general" && <GeneralTab space={space} me={me} onDirtyChange={setDirty} />}
        {tab === "members" && <MembersTab space={space} me={me} />}
        {tab === "tools" && <LinkTab title={t("spaces.spaceSettings.tools.title")} description={t("spaces.spaceSettings.tools.description")}>
          <SpaceToolsPanel space={space} />
        </LinkTab>}
        {tab === "item-types" && <ItemTypesPanel spaceId={space.id} />}
        {tab === "custom-fields" && <CustomFieldsTab space={space} />}
        {tab === "workflows" && <WorkflowDefaultsTab space={space} />}
        {tab === "automation" && <LinkTab title={t("spaces.spaceSettings.automation.title")} description={t("spaces.spaceSettings.automation.description")}>
          <SpaceAutomationPanel spaceId={space.id} />
        </LinkTab>}
        {tab === "forms" && <LinkTab title={t("spaces.spaceSettings.forms.title")} description={t("spaces.spaceSettings.forms.description")}>
          <FormsList spaceId={space.id} embedded />
        </LinkTab>}
        {tab === "blueprints" && <BlueprintsTab space={space} />}
        </div>
      </div>
      <ConfirmDialog
        open={blocker.status === "blocked"}
        onOpenChange={(open) => { if (!open) blocker.reset?.(); }}
        title={t("spaces.spaceSettings.general.discardTitle")}
        description={t("spaces.spaceSettings.general.discardDescription")}
        confirmLabel={t("spaces.spaceSettings.general.leave")}
        cancelLabel={t("spaces.spaceSettings.general.stay")}
        onConfirm={() => blocker.proceed?.()}
      />
    </WorkspaceLayout>
  );
}

function GeneralTab({ space, me, onDirtyChange }: { space: Space; me?: CurrentUser; onDirtyChange: (dirty: boolean) => void }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const personal = space.kind === "personal";
  const canEdit = can(space, "edit") && !personal && !space.archived;
  const [name, setName] = useState(space.name);
  const [description, setDescription] = useState(space.description ?? "");
  const [iconKey, setIconKey] = useState<string | null>(space.icon_key ?? null);
  const [errors, setErrors] = useState<{ name?: string; description?: string }>({});
  const [formError, setFormError] = useState("");
  const [stale, setStale] = useState(false);
  const [busy, setBusy] = useState(false);
  // Dirty is compared with the saved values, so editing back to the saved text is clean again.
  const dirty = name !== space.name || description !== (space.description ?? "") || iconKey !== (space.icon_key ?? null);
  useEffect(() => { onDirtyChange(dirty); }, [dirty, onDirtyChange]);
  useEffect(() => () => onDirtyChange(false), [onDirtyChange]);

  // A newer version from the server replaces the form only when nothing is waiting to be saved.
  useEffect(() => {
    if (!dirty) { setName(space.name); setDescription(space.description ?? ""); setIconKey(space.icon_key ?? null); }
  }, [space.version]);

  function cancel() {
    setName(space.name); setDescription(space.description ?? ""); setIconKey(space.icon_key ?? null);
    setErrors({}); setFormError(""); setStale(false);
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy || !canEdit) return;
    const trimmedName = name.trim();
    const trimmedDescription = description.trim();
    const next: typeof errors = {};
    // Counts match the server (runes, not UTF-16 units).
    if (!trimmedName) next.name = t("spaces.form.nameRequired");
    else if ([...trimmedName].length > 120) next.name = t("spaces.spaceSettings.general.nameTooLong");
    if ([...trimmedDescription].length > 1000) next.description = t("spaces.spaceSettings.general.descriptionTooLong");
    setErrors(next);
    if (next.name || next.description) return;
    setBusy(true); setFormError(""); setStale(false);
    try {
      const updated = await api<Space>(`/api/v1/spaces/${space.id}`, {
        method: "PATCH", headers: { "If-Match": etag(space) },
        body: JSON.stringify({ name: trimmedName, description: trimmedDescription || null, icon_key: iconKey }),
      });
      queryClient.setQueryData(spaceKeys.one(space.id), updated);
      await queryClient.invalidateQueries({ queryKey: spaceKeys.all });
      setName(updated.name); setDescription(updated.description ?? ""); setIconKey(updated.icon_key ?? null);
      toast.success(t("spaces.form.saved"));
    } catch (err) {
      if (err instanceof ApiError && (err.status === 409 || err.status === 428)) setStale(true);
      else setFormError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function reload() {
    await queryClient.invalidateQueries({ queryKey: spaceKeys.one(space.id) });
    await queryClient.invalidateQueries({ queryKey: spaceKeys.all });
    setStale(false);
  }

  return (
    <div className="space-y-6">
      <form className="space-y-6" noValidate onSubmit={save} aria-label={t("spaces.spaceSettings.general.title")}>
        <SettingsSection title={t("spaces.spaceSettings.general.title")} description={t("spaces.spaceSettings.general.description")}>
          {stale && <div role="alert" className="flex items-center justify-between gap-2 rounded-lg border border-warning bg-warning-subtle p-2 text-sm">
            <span>{t("spaces.spaceSettings.general.stale")}</span>
            <Button type="button" size="sm" variant="outline" onClick={() => void reload()}>{t("spaces.conflict.reload")}</Button>
          </div>}
          {personal && <p className="text-sm text-muted-foreground">{t("spaces.spaceSettings.general.personalReadOnly")}</p>}
          {space.has_active_admin === false && <p role="note" className="text-sm text-warning-foreground">{t("spaces.spaceSettings.general.noActiveAdmin")}</p>}
          {space.archived && <p className="text-sm text-muted-foreground">{t("spaces.spaceSettings.general.archivedNote")}</p>}
          {!personal && !space.archived && !canEdit && <p className="text-sm text-muted-foreground">{t("spaces.spaceSettings.general.readOnly")}</p>}
          <fieldset disabled={!canEdit || busy} className="space-y-4">
            <div className="space-y-1.5">
              <label htmlFor="space-settings-name" className="block text-sm font-medium">{t("spaces.form.name")}</label>
              <Input id="space-settings-name" value={name} onChange={(event) => { setName(event.target.value); setErrors((e) => ({ ...e, name: undefined })); }}
                autoComplete="off" aria-invalid={!!errors.name} aria-describedby={errors.name ? "space-settings-name-error" : undefined} />
              {errors.name && <p id="space-settings-name-error" className="text-sm text-destructive">{errors.name}</p>}
            </div>
            <div className="space-y-1.5">
              <label htmlFor="space-settings-description" className="block text-sm font-medium">{t("spaces.form.description")}</label>
              <textarea id="space-settings-description" value={description} rows={3} aria-invalid={!!errors.description}
                aria-describedby={errors.description ? "space-settings-description-error" : undefined}
                onChange={(event) => { setDescription(event.target.value); setErrors((e) => ({ ...e, description: undefined })); }}
                className="w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-1.5 text-base outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 md:text-sm dark:bg-input/30" />
              {errors.description && <p id="space-settings-description-error" className="text-sm text-destructive">{errors.description}</p>}
            </div>
            {!personal && <SpaceIconPicker value={iconKey} onChange={setIconKey} disabled={!canEdit} />}
            <FormError message={formError} />
            {canEdit && <SettingsSaveBar dirty={dirty} busy={busy} onCancel={cancel} />}
          </fieldset>
        </SettingsSection>
        <SettingsSection title={t("spaces.spaceSettings.general.details")}>
          <SettingsRow label={t("spaces.spaceSettings.general.spaceId")}>
            <code className="break-all text-xs text-muted-foreground">{space.id}</code>
          </SettingsRow>
          <SettingsRow label={t("spaces.spaceSettings.general.type")}>
            <span className="text-sm">{personal ? t("spaces.personal") : t("settings.spaces.shared")}</span>
          </SettingsRow>
        </SettingsSection>
      </form>
      {!personal && <TemplateSection space={space} me={me} />}
      {!personal && <LifecycleSection space={space} me={me} />}
    </div>
  );
}

/** Save this Space's structure as a template. Full access, and a user type that may create Spaces. */
function TemplateSection({ space, me }: { space: Space; me?: CurrentUser }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [name, setName] = useState(space.name);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  if (!space.my_capabilities?.includes("share") || (me?.user_type !== "admin" && me?.user_type !== "regular")) return null;
  return (
    <SettingsSection title={t("spaces.template.saveTitle")} description={t("spaces.template.saveHint")}>
      <form className="flex flex-wrap items-end gap-2" onSubmit={async (event) => {
        event.preventDefault();
        setBusy(true); setError("");
        try {
          await api("/api/v1/space-templates", { method: "POST", body: JSON.stringify({ space_id: space.id, name: name.trim() }) });
          await queryClient.invalidateQueries({ queryKey: ["space-templates"] });
          toast.success(t("spaces.template.savedToast"));
        } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
      }}>
        <label className="min-w-48 flex-1 space-y-1 text-sm">{t("spaces.template.name")}
          <Input value={name} onChange={(event) => setName(event.target.value)} maxLength={120} required /></label>
        <Button type="submit" disabled={busy || !name.trim()}>{t("spaces.template.save")}</Button>
      </form>
      <FormError message={error} />
    </SettingsSection>
  );
}

/** Visibility and archive (D-01, D-03). Only Workspace admins and explicit Space Admins see these controls. */
function LifecycleSection({ space, me }: { space: Space; me?: CurrentUser }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [pending, setPending] = useState<null | "public" | "private" | "archive" | "restore">(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const canAdminister = !!space.is_space_admin; // D-SA1: a workspace admin without the explicit role is not a Space Admin
  if (!canAdminister) return null;
  const isPublic = space.visibility === "public";

  async function confirm() {
    if (!pending) return;
    setBusy(true); setError("");
    try {
      if (pending === "public" || pending === "private") {
        const updated = await api<Space>(`/api/v1/spaces/${space.id}`, { method: "PATCH", headers: { "If-Match": etag(space) }, body: JSON.stringify({ visibility: pending }) });
        queryClient.setQueryData(spaceKeys.one(space.id), updated);
      } else {
        await api(`/api/v1/spaces/${space.id}/${pending}`, { method: "POST", headers: { "If-Match": etag(space) } });
      }
      await queryClient.invalidateQueries({ queryKey: spaceKeys.all });
      await queryClient.invalidateQueries({ queryKey: spaceKeys.one(space.id) });
      toast.success(t("spaces.spaceSettings.lifecycle.done"));
      setPending(null);
    } catch (e) {
      setError(e instanceof ApiError && e.status === 409 ? t("spaces.conflict.message") : (e as Error).message);
    } finally { setBusy(false); }
  }

  return (
    <SettingsSection title={t("spaces.spaceSettings.lifecycle.title")} description={t("spaces.spaceSettings.lifecycle.description")}>
      {!space.archived && space.kind === "shared" && <SettingsRow label={t("spaces.spaceSettings.lifecycle.visibility")}
        hint={isPublic ? t("spaces.spaceSettings.lifecycle.publicHint") : t("spaces.spaceSettings.lifecycle.privateHint")}>
        <Button variant="outline" onClick={() => setPending(isPublic ? "private" : "public")}>
          {isPublic ? t("spaces.spaceSettings.lifecycle.makePrivate") : t("spaces.spaceSettings.lifecycle.makePublic")}
        </Button>
      </SettingsRow>}
      <SettingsRow label={space.archived ? t("spaces.spaceSettings.lifecycle.restore") : t("spaces.spaceSettings.lifecycle.archive")}
        hint={space.archived ? t("spaces.spaceSettings.lifecycle.restoreHint") : t("spaces.spaceSettings.lifecycle.archiveHint")}>
        <Button variant={space.archived ? "outline" : "destructive"} onClick={() => setPending(space.archived ? "restore" : "archive")}>
          {space.archived ? t("spaces.spaceSettings.lifecycle.restore") : t("spaces.spaceSettings.lifecycle.archive")}
        </Button>
      </SettingsRow>
      <ConfirmDialog
        open={!!pending}
        onOpenChange={(open) => { if (!open) setPending(null); }}
        title={t(`spaces.spaceSettings.lifecycle.confirm.${pending ?? "archive"}.title`, { name: space.name })}
        description={t(`spaces.spaceSettings.lifecycle.confirm.${pending ?? "archive"}.description`, { name: space.name })}
        confirmLabel={t(`spaces.spaceSettings.lifecycle.confirm.${pending ?? "archive"}.action`)}
        cancelLabel={t("spaces.form.cancel")}
        onConfirm={confirm}
        busy={busy}
        error={error}
      />
    </SettingsSection>
  );
}

type SpaceMember = { user_id: string; name: string; user_type: string; role: string; capped_by_user_type: boolean; sources: { kind: "direct" | "group"; name?: string; role: string; role_name?: string }[] };

type WorkflowDefaults = { task: string | null; project: string | null };
const WORKFLOW_BASES: (keyof WorkflowDefaults)[] = ["task", "project"];

/** Default workflow for root Tasks and Projects in this Space (B). Existing items keep their workflow and status. */
function WorkflowDefaultsTab({ space }: { space: Space }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const canEdit = can(space, "edit") && space.kind === "shared";
  const workflows = useQuery({ queryKey: ["workflows"], queryFn: () => api<{ items: { id: string; name: string }[] }>("/api/v1/workflows") });
  const defaults = useQuery({ queryKey: ["space-default-workflows", space.id], queryFn: () => api<WorkflowDefaults>(`/api/v1/spaces/${space.id}/default-workflows`) });
  async function change(base: keyof WorkflowDefaults, value: string) {
    const next = { ...(defaults.data ?? { task: null, project: null }), [base]: value || null };
    setBusy(true); setError("");
    try {
      await api(`/api/v1/spaces/${space.id}/default-workflows`, { method: "PUT", body: JSON.stringify(next) });
      toast.success(t("spaces.spaceSettings.workflows.saved"));
      await queryClient.invalidateQueries({ queryKey: ["space-default-workflows", space.id] });
    } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  const options = workflows.data?.items ?? [];
  return <SettingsSection title={t("spaces.spaceSettings.workflows.title")} description={t("spaces.spaceSettings.workflows.description")}>
    {defaults.isError && <FormError message={t("spaces.spaceSettings.workflows.loadError")} />}
    {WORKFLOW_BASES.map((base) => <div key={base} className="flex flex-wrap items-center justify-between gap-3">
      <label htmlFor={`space-workflow-${base}`} className="text-sm font-medium">{t(`spaces.spaceSettings.workflows.${base}`)}</label>
      <select id={`space-workflow-${base}`} disabled={!canEdit || busy} value={defaults.data?.[base] ?? ""}
        onChange={(e) => void change(base, e.target.value)} className="h-8 min-w-48 rounded-md border border-input bg-background px-2 text-sm">
        <option value="">{t("spaces.spaceSettings.workflows.workspaceDefault")}</option>
        {options.map((w) => <option key={w.id} value={w.id}>{w.name}</option>)}
      </select>
    </div>)}
    {!canEdit && <p className="text-sm text-muted-foreground">{t("spaces.spaceSettings.workflows.readOnly")}</p>}
    <p role="alert" className="text-sm text-destructive">{error}</p>
    <Link to="/settings/workflows" className={linkClass}>{t("spaces.spaceSettings.workflows.open")}</Link>
  </SettingsSection>;
}

type SpaceBlueprint = { id: string; name: string; item_type: string | null; item_count: number; scope: string; version: number };

/** Blueprints of this Space (C). Lists only what the server returns to people who can view the Space; create and edit reuse the Blueprint page. */
function BlueprintsTab({ space }: { space: Space }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const blueprints = useQuery({
    queryKey: ["space-blueprints", space.id],
    queryFn: async () => (await api<{ items: SpaceBlueprint[] }>(`/api/v1/spaces/${space.id}/blueprints`)).items,
    enabled: space.kind === "shared",
  });
  return <SettingsSection title={t("spaces.spaceSettings.blueprints.title")} description={t("spaces.spaceSettings.blueprints.description")}>
    {blueprints.isError && <FormError message={t("spaces.spaceSettings.blueprints.loadError")} />}
    {blueprints.data && !blueprints.data.length && <EmptyState icon={<LayoutTemplate />} title={t("spaces.spaceSettings.blueprints.emptyTitle")}
      description={t("spaces.spaceSettings.blueprints.empty")} primaryAction={{ label: t("spaces.spaceSettings.blueprints.open"), onClick: () => void navigate({ to: "/blueprints" }) }} />}
    {!!blueprints.data?.length && <ul className="divide-y rounded-lg border text-sm">
      {blueprints.data.map((bp) => <li key={bp.id} className="flex flex-wrap items-center justify-between gap-2 px-3 py-2">
        <span className="font-medium">{bp.name}</span>
        <span className="text-xs text-muted-foreground">{t("spaces.spaceSettings.blueprints.count", { count: bp.item_count })} · {t("spaces.spaceSettings.blueprints.version", { version: bp.version })}{bp.item_type && <> · {bp.item_type}</>}</span>
      </li>)}
    </ul>}
    {!(blueprints.data && !blueprints.data.length) && <Link to="/blueprints" className={linkClass}>{t("spaces.spaceSettings.blueprints.open")}</Link>}
  </SettingsSection>;
}

type SpaceAdmin = { user_id: string; name: string; granted_at: string };

/** Explicit Space Admins (D-SA1). Admins are granted only to members; the last admin cannot be removed; transfer is explicit. */
function AdminsSection({ space, me, members }: { space: Space; me?: CurrentUser; members: SpaceMember[] }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [picked, setPicked] = useState("");
  // Removing your own admin role is allowed only when another admin remains; it is confirmed first because you lose control of the Space.
  const [selfRemove, setSelfRemove] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const admins = useQuery({
    queryKey: ["space-admins", space.id],
    queryFn: async () => (await api<{ items: SpaceAdmin[] }>(`/api/v1/spaces/${space.id}/admins`)).items,
  });
  const canAdminister = !!space.is_space_admin; // D-SA1: a workspace admin without the explicit role is not a Space Admin
  const adminIds = new Set((admins.data ?? []).map((a) => a.user_id));
  // S-1: only an admin or regular user can be offered the role; the backend refuses the others with 422.
  const candidates = members.filter((m) => !adminIds.has(m.user_id) && (m.user_type === "admin" || m.user_type === "regular"));
  const removeAdmin = (userID: string) => api(`/api/v1/spaces/${space.id}/admins/${userID}`, { method: "DELETE" });
  async function run(action: () => Promise<unknown>) {
    setBusy(true); setError("");
    try {
      await action();
      setPicked("");
      await queryClient.invalidateQueries({ queryKey: ["space-admins", space.id] });
      await queryClient.invalidateQueries({ queryKey: ["space-members", space.id] });
      await queryClient.invalidateQueries({ queryKey: spaceKeys.one(space.id) });
    } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  return (
    <SettingsSection title={t("spaces.spaceSettings.admins.title")} description={t("spaces.spaceSettings.admins.description")}>
      {admins.data && <ul className="divide-y rounded-lg border text-sm">
        {admins.data.map((admin) => <li key={admin.user_id} className="flex items-center justify-between gap-2 px-3 py-2">
          <span>{admin.name}</span>
          {canAdminister && <span title={admins.data.length <= 1 ? t("spaces.spaceSettings.admins.lastAdmin") : undefined} className="inline-flex">
            <Button variant="ghost" size="sm" disabled={busy || admins.data.length <= 1} aria-describedby={admins.data.length <= 1 ? "last-admin-hint" : undefined}
              onClick={() => (admin.user_id === me?.id ? setSelfRemove(true) : void run(() => removeAdmin(admin.user_id)))}>
              {t("spaces.spaceSettings.admins.remove", { name: admin.name })}
            </Button>
          </span>}
        </li>)}
        {!admins.data.length && <li className="px-3 py-2 text-muted-foreground">{t("spaces.spaceSettings.admins.none")}</li>}
      </ul>}
      {canAdminister && !!candidates.length && <div className="flex flex-wrap items-end gap-2">
        <div className="space-y-1">
          <label htmlFor="space-admin-pick" className="block text-sm font-medium">{t("spaces.spaceSettings.admins.pick")}</label>
          <select id="space-admin-pick" value={picked} onChange={(e) => setPicked(e.target.value)} className="block h-8 rounded-md border border-input bg-background px-2 text-sm">
            <option value="">{t("spaces.spaceSettings.admins.pickNone")}</option>
            {candidates.map((m) => <option key={m.user_id} value={m.user_id}>{m.name}</option>)}
          </select>
        </div>
        <Button variant="outline" disabled={!picked || busy} onClick={() => void run(() => api(`/api/v1/spaces/${space.id}/admins`, { method: "POST", body: JSON.stringify({ user_id: picked }) }))}>
          {t("spaces.spaceSettings.admins.grant")}
        </Button>
        <Button variant="outline" disabled={!picked || busy} onClick={() => void run(() => api(`/api/v1/spaces/${space.id}/transfer-ownership`, { method: "POST", body: JSON.stringify({ user_id: picked }) }))}>
          {t("spaces.spaceSettings.admins.transfer")}
        </Button>
      </div>}
      {admins.data && admins.data.length <= 1 && <p id="last-admin-hint" className="text-xs text-muted-foreground">{t("spaces.spaceSettings.admins.lastAdmin")}</p>}
      <p role="alert" className="text-sm text-destructive">{error}</p>
      <ConfirmDialog
        open={selfRemove}
        onOpenChange={(open) => { if (!open) setSelfRemove(false); }}
        title={t("spaces.spaceSettings.admins.selfTitle")}
        description={t("spaces.spaceSettings.admins.selfDescription")}
        confirmLabel={t("spaces.spaceSettings.admins.selfConfirm")}
        cancelLabel={t("spaces.spaceSettings.general.stay")}
        onConfirm={() => { setSelfRemove(false); void run(() => removeAdmin(me?.id ?? "")); }}
      />
    </SettingsSection>
  );
}

type ShareRow = { principal_type: "user" | "group"; principal_id: string; principal_name: string; role: string; role_name?: string; inherited_from: string | null };

/** Direct shares of one Space: find or invite people, change direct roles, remove with the impact shown (D-06, Batch 2). */
function DirectAccessEditor({ space, me, members }: { space: Space; me: CurrentUser; members: SpaceMember[] }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [removing, setRemoving] = useState<ShareRow | null>(null);
  const shares = useQuery({
    queryKey: ["space-shares", space.id],
    queryFn: async () => {
      let etagValue = "";
      const { items } = await api<{ items: ShareRow[] }>(`/api/v1/spaces/${space.id}/shares`, undefined, (headers) => { etagValue = headers.get("ETag") ?? ""; });
      return { items: items.filter((row) => !row.inherited_from), etag: etagValue };
    },
    gcTime: 0,
  });
  const groups = useQuery({ queryKey: ["groups"], queryFn: async () => (await api<{ items: Group[] }>("/api/v1/groups")).items });
  const rows = shares.data?.items ?? [];
  const taken = new Set(rows.map((r) => `${r.principal_type}:${r.principal_id}:direct`));
  // People who reach the Space through a group: removing their direct share does not remove that access.
  const viaGroups = (userName: string) => members.find((m) => m.name === userName)?.sources.filter((s) => s.kind === "group").map((s) => s.name ?? "") ?? [];

  async function save(next: ShareRow[], message: string) {
    if (!shares.data) return;
    setBusy(true); setError("");
    try {
      await api(`/api/v1/spaces/${space.id}/shares`, {
        method: "PUT", headers: { "If-Match": shares.data.etag },
        body: JSON.stringify({ shares: next.map(({ principal_type, principal_id, role }) => ({ principal_type, principal_id, role })) }),
      });
      toast.success(message);
      await queryClient.invalidateQueries({ queryKey: ["space-shares", space.id] });
      await queryClient.invalidateQueries({ queryKey: ["space-members", space.id] });
      await queryClient.invalidateQueries({ queryKey: spaceKeys.one(space.id) });
    } catch (e) {
      setError(e instanceof ApiError && e.status === 409 ? t("spaces.spaceSettings.direct.stale") : (e as Error).message);
      if (e instanceof ApiError && e.status === 409) await queryClient.invalidateQueries({ queryKey: ["space-shares", space.id] });
    } finally { setBusy(false); }
  }

  return (
    <SettingsSection title={t("spaces.spaceSettings.direct.title")} description={t("spaces.spaceSettings.direct.description")}>
      {shares.isError && <FormError message={t("spaces.spaceSettings.direct.loadError")} />}
      <Search me={me} groups={groups.data ?? []} taken={taken}
        onAdd={(principal, role) => void save([...rows, { ...principal, role, inherited_from: null, principal_name: principal.principal_name }], t("spaces.spaceSettings.direct.added"))} />
      {!!rows.length && <ul className="divide-y rounded-lg border text-sm">
        {rows.map((row) => <li key={`${row.principal_type}:${row.principal_id}`} className="flex flex-wrap items-center justify-between gap-2 px-3 py-2">
          <span className="font-medium">{row.principal_name}{row.principal_type === "group" && <span className="ms-2 text-xs text-muted-foreground">{t("spaces.spaceSettings.direct.groupTag")}</span>}</span>
          <span className="flex items-center gap-2">
            <select aria-label={t("spaces.spaceSettings.direct.roleFor", { name: row.principal_name })} disabled={busy || row.role.startsWith("custom:")}
              value={row.role} className="h-8 rounded-md border border-input bg-background px-2 text-sm"
              onChange={(e) => {
                // Direct role and effective role can differ: say so when a group still gives this person access.
                const groupNames = row.principal_type === "user" ? viaGroups(row.principal_name) : [];
                void save(rows.map((r) => r === row ? { ...r, role: e.target.value } : r),
                  groupNames.length ? t("spaces.spaceSettings.direct.savedKeepsGroup", { groups: groupNames.join(", ") }) : t("spaces.spaceSettings.direct.saved"));
              }}>
              {row.role.startsWith("custom:") && <option value={row.role}>{row.role_name || t("share.role.custom")}</option>}
              {SHARE_ROLES.map((r) => <option key={r} value={r}>{t(`share.role.${r}`)}</option>)}
            </select>
            <Button variant="ghost" size="sm" disabled={busy} onClick={() => setRemoving(row)}>
              {t("spaces.spaceSettings.direct.remove", { name: row.principal_name })}
            </Button>
          </span>
        </li>)}
      </ul>}
      <p role="alert" className="text-sm text-destructive">{error}</p>
      <ConfirmDialog
        open={!!removing}
        onOpenChange={(open) => { if (!open) setRemoving(null); }}
        title={t("spaces.spaceSettings.direct.removeTitle", { name: removing?.principal_name ?? "" })}
        description={removing?.principal_type === "user" && viaGroups(removing.principal_name).length
          ? t("spaces.spaceSettings.direct.removeKeepsGroup", { groups: viaGroups(removing.principal_name).join(", ") })
          : t("spaces.spaceSettings.direct.removeDescription")}
        confirmLabel={t("spaces.spaceSettings.direct.removeConfirm")}
        cancelLabel={t("spaces.form.cancel")}
        busy={busy}
        error={error}
        onConfirm={async () => {
          if (!removing) return;
          await save(rows.filter((r) => r !== removing), t("spaces.spaceSettings.direct.removed"));
          setRemoving(null);
        }}
      />
    </SettingsSection>
  );
}

type SpaceTool = { id: string; name: string; type: string; is_default: boolean; position: number; visibility: string };

/** Tools of one Space (Batch 6). Adding, renaming, duplicating and reordering stay in the Space's own tool bar; here you see
 *  the tools, open one, and choose the landing tool. Shortcuts elsewhere never replace these Space settings. */
function SpaceToolsPanel({ space }: { space: Space }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const canEdit = can(space, "edit");
  const tools = useQuery({
    queryKey: ["space-tools", space.id],
    queryFn: async () => (await api<{ items: SpaceTool[] }>(`/api/v1/spaces/${space.id}/views`)).items,
  });
  async function setLanding(tool: SpaceTool) {
    setBusy(true); setError("");
    try {
      await api(`/api/v1/views/${tool.id}`, { method: "PATCH", body: JSON.stringify({ is_default: true }) });
      toast.success(t("spaces.spaceSettings.tools.landingSaved", { name: tool.name }));
      await queryClient.invalidateQueries({ queryKey: ["space-tools", space.id] });
    } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  return <div className="space-y-3">
    {tools.isError && <FormError message={t("spaces.spaceSettings.tools.loadError")} />}
    {tools.data && !tools.data.length && <p className="text-sm text-muted-foreground">{t("spaces.spaceSettings.tools.empty")}</p>}
    {!!tools.data?.length && <ul className="divide-y rounded-lg border text-sm">
      {tools.data.map((tool) => <li key={tool.id} className="flex flex-wrap items-center justify-between gap-2 px-3 py-2">
        <span className="min-w-0">
          <span className="font-medium">{tool.name}</span>
          <span className="ms-2 text-xs text-muted-foreground">{t(`spaces.spaceSettings.tools.type.${tool.type}`, { defaultValue: tool.type })}</span>
          {tool.is_default && <span className="ms-2 rounded bg-muted px-1.5 py-0.5 text-xs">{t("spaces.spaceSettings.tools.landing")}</span>}
        </span>
        <span className="flex gap-1">
          <Link to="/spaces/$spaceId" params={{ spaceId: space.id }} search={{ view: tool.id }} className={buttonVariants({ variant: "outline", size: "sm" })}
            aria-label={t("spaces.spaceSettings.tools.openNamed", { name: tool.name })}>{t("spaces.spaceSettings.tools.open")}</Link>
          {canEdit && !tool.is_default && <Button size="sm" variant="ghost" disabled={busy} onClick={() => void setLanding(tool)}
            aria-label={t("spaces.spaceSettings.tools.setLandingNamed", { name: tool.name })}>{t("spaces.spaceSettings.tools.setLanding")}</Button>}
        </span>
      </li>)}
    </ul>}
    <p role="alert" className="text-sm text-destructive">{error}</p>
    {canEdit && <p className="text-sm text-muted-foreground">{t("spaces.spaceSettings.tools.manageHint")}</p>}
  </div>;
}

// Members and access: who has access and why. Changes still go through the share editor, which the server checks.
function MembersTab({ space, me }: { space: Space; me?: CurrentUser }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const shared = space.kind === "shared";
  const canShare = shared && can(space, "share");
  const members = useQuery({
    queryKey: ["space-members", space.id],
    queryFn: async () => (await api<{ items: SpaceMember[] }>(`/api/v1/spaces/${space.id}/members`)).items,
    enabled: shared,
  });
  const roleLabel = (role: string) => role === "custom" ? t("share.role.custom") : t(`share.role.${role}`);
  return (
    <div className="space-y-6">
      <SettingsSection title={t("spaces.spaceSettings.members.title")} description={t("spaces.spaceSettings.members.description")}>
        {!shared && <p className="text-sm text-muted-foreground">{t("spaces.spaceSettings.members.personal")}</p>}
        {shared && !canShare && <p className="text-sm text-muted-foreground">{t("spaces.spaceSettings.members.noPermission")}</p>}
        {/* Read-only summary: who has access and why. Changes happen in "Direct access" below, the one place to add or edit. */}
        {members.isPending && shared && <p role="status" className="text-sm text-muted-foreground">{t("spaces.spaceSettings.members.loading")}</p>}
        {members.isError && <FormError message={t("spaces.spaceSettings.members.loadError")} />}
        {members.data && !members.data.length && <p className="text-sm text-muted-foreground">{t("spaces.spaceSettings.members.empty")}</p>}
        {!!members.data?.length && <div className="overflow-x-auto rounded-lg border">
          <table className="w-full text-start text-sm">
            <caption className="sr-only">{t("spaces.spaceSettings.members.title")}</caption>
            <thead className="bg-muted/60 text-xs font-medium text-foreground">
              <tr>
                <th scope="col" className="px-3 py-2">{t("spaces.spaceSettings.members.colName")}</th>
                <th scope="col" className="px-3 py-2">{t("spaces.spaceSettings.members.colRole")}</th>
                <th scope="col" className="px-3 py-2">{t("spaces.spaceSettings.members.colWhy")}</th>
              </tr>
            </thead>
            <tbody className="divide-y">{members.data.map((member) => <tr key={member.user_id}>
              <th scope="row" className="px-3 py-2.5 text-start font-medium">{member.name}</th>
              <td className="px-3 py-2.5">
                {roleLabel(member.role)}
                {member.capped_by_user_type && <span className="ms-2 text-xs text-muted-foreground">{t("spaces.spaceSettings.members.capped")}</span>}
              </td>
              <td className="px-3 py-2.5 text-muted-foreground">
                <ul className="space-y-0.5">
                  {member.sources.map((source, index) => <li key={index}>
                    {source.kind === "direct" ? t("spaces.spaceSettings.members.direct") : t("spaces.spaceSettings.members.viaGroup", { name: source.name ?? "" })}
                    {" · "}{source.role_name || roleLabel(source.role)}
                  </li>)}
                </ul>
              </td>
            </tr>)}</tbody>
          </table>
        </div>}
      </SettingsSection>
      {canShare && me && <DirectAccessEditor space={space} me={me} members={members.data ?? []} />}
      {shared && <AdminsSection space={space} me={me} members={members.data ?? []} />}
      <SettingsSection title={t("spaces.spaceSettings.members.rulesTitle")}>
        <ul className="list-disc space-y-1 ps-5 text-sm text-muted-foreground">
          <li>{t("spaces.spaceSettings.members.ruleUnion")}</li>
          <li>{t("spaces.spaceSettings.members.ruleUserType")}</li>
          <li>{t("spaces.spaceSettings.members.ruleItems")}</li>
          <li>{t("spaces.spaceSettings.members.ruleJobRole")}</li>
        </ul>
      </SettingsSection>
    </div>
  );
}

function CustomFieldsTab({ space }: { space: Space }) {
  const { t } = useTranslation();
  const canManage = can(space, "share") && can(space, "delete");
  return canManage
    ? <CustomFieldRegistry spaceId={space.id} />
    : <SettingsSection title={t("spaces.customFields.title")} description={t("spaces.customFields.description")}>
        <p className="text-sm text-muted-foreground">{t("spaces.spaceSettings.customFields.noPermission")}</p>
      </SettingsSection>;
}

/** Tabs whose editor already lives on another page: this tab explains it and links there. */
function LinkTab({ title, description, children }: { title: string; description: string; children: ReactNode }) {
  return <SettingsSection title={title} description={description}>{children}</SettingsSection>;
}

const linkClass = buttonVariants({ variant: "outline" }) + " w-fit";
