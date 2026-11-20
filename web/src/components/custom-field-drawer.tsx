import { useMemo, useState, type FormEvent } from "react";
import { useQuery, useQueries, useQueryClient } from "@tanstack/react-query";
import { useSpaces } from "@/lib/space-queries";
import { can } from "@/lib/spaces";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { useItemTypes } from "@/lib/item-types";
import { customFieldText, type CustomField, type CustomFieldValue } from "@/lib/custom-fields";
import { FormError } from "@/components/auth/form-error";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { InitialsAvatar } from "@/components/admin/admin-ui";
import {
  CustomFieldFields, draftFrom, draftPayload, type Field, type FieldDraft,
} from "@/components/custom-field-form";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

type Level = "view" | "edit" | "none";
type Grant = { subject_kind: "user" | "group"; subject_id: string; level: Level; subject_name?: string };
type Person = { id: string; first_name: string; last_name: string; email: string };
type Group = { id: string; name: string };
const LEVELS: Level[] = ["view", "edit", "none"];

export type FieldScope = { spaceId: string | null; ownerName: string };

/**
 * Right-side editor for one custom field. Sections: General and Type settings (shared with the Space dialog), Ownership and
 * applicability, Permissions (grants, saved separately), and Preview. The draft is saved only by Save; closing with unsaved
 * changes asks first.
 */
export function CustomFieldDrawer({ open, onOpenChange, scope, field, onSaved }: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  scope: FieldScope;
  /** undefined creates a field in the scope. */
  field?: Field;
  onSaved: () => void;
}) {
  const { t } = useTranslation();
  const isAdmin = useProfile()?.user_type === "admin";
  const [draft, setDraft] = useState<FieldDraft>(() => draftFrom(field));
  const [initial, setInitial] = useState(() => JSON.stringify(draft));
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [discardOpen, setDiscardOpen] = useState(false);
  const dirty = JSON.stringify(draft) !== initial;
  // A field can be managed when the scope lets the person edit it; the server still decides on every write.
  const canEdit = scope.spaceId === null ? isAdmin : true;

  function requestClose() {
    if (dirty && !pending) setDiscardOpen(true);
    else onOpenChange(false);
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending || !canEdit) return;
    const built = draftPayload(draft);
    if (built.error || !built.body) {
      setError(t(built.error ?? "spaces.customFields.nameRequired"));
      return;
    }
    const base = scope.spaceId ? `/api/v1/spaces/${scope.spaceId}/custom-fields` : "/api/v1/custom-fields";
    setPending(true);
    setError("");
    try {
      if (draft.id) await api(`${base}/${draft.id}`, { method: "PATCH", body: JSON.stringify(built.body) });
      else await api(base, { method: "POST", body: JSON.stringify({ ...built.body, type: draft.type }) });
      onSaved();
      onOpenChange(false);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setPending(false);
    }
  }

  return <>
    <Dialog open={open} onOpenChange={(next) => (next ? onOpenChange(true) : requestClose())}>
      <DialogContent className="inset-y-0 top-0 right-0 bottom-0 left-auto h-dvh max-h-dvh w-full max-w-xl translate-x-0 translate-y-0 overflow-y-auto rounded-none p-0 sm:max-w-xl">
        <form className="flex min-h-full flex-col" onSubmit={save} aria-describedby="cf-drawer-desc">
          <header className="space-y-1 border-b p-5 pr-12">
            <DialogTitle>{field ? t("spaces.customFields.drawer.editTitle", { name: field.name }) : t("spaces.customFields.drawer.createTitle")}</DialogTitle>
            <DialogDescription id="cf-drawer-desc">{t("spaces.customFields.drawer.scope", { owner: scope.ownerName })}</DialogDescription>
          </header>
          <div className="flex-1 space-y-8 p-5">
            {!canEdit && <p role="status" className="rounded-md border p-3 text-sm text-muted-foreground">{t("spaces.customFields.drawer.readOnly")}</p>}
            <CustomFieldFields draft={draft} onChange={setDraft} pending={pending || !canEdit} typeLocked={!!field} />
            {field && <OwnershipSection field={field} scope={scope} isAdmin={isAdmin} />}
            {field && !field.space_id && <WorkspaceAssignments field={field} canManage={canEdit} />}
            {field && <PermissionsSection field={field} scope={scope} canManage={canEdit} />}
            <PreviewSection draft={draft} />
            <FormError message={error} />
          </div>
          <footer className="sticky bottom-0 flex justify-end gap-2 border-t bg-background p-4">
            <Button type="button" variant="outline" disabled={pending} onClick={requestClose}>{t("spaces.customFields.drawer.close")}</Button>
            <Button type="submit" disabled={pending || !canEdit || !dirty}>{t("spaces.customFields.drawer.save")}</Button>
          </footer>
        </form>
      </DialogContent>
    </Dialog>
    <ConfirmDialog open={discardOpen} onOpenChange={setDiscardOpen}
      title={t("spaces.customFields.drawer.discardTitle")} description={t("spaces.customFields.drawer.discardDescription")}
      confirmLabel={t("spaces.customFields.drawer.discard")} cancelLabel={t("spaces.customFields.drawer.keepEditing")}
      onConfirm={() => { setDiscardOpen(false); setInitial(""); onOpenChange(false); }} />
  </>;
}

/** Where the field is owned and how many item types use it. The applicability count needs admin, so others see the owner only. */
function OwnershipSection({ field, scope, isAdmin }: { field: Field; scope: FieldScope; isAdmin: boolean }) {
  const { t } = useTranslation();
  const types = useItemTypes(isAdmin);
  const assigned = useQueries({
    queries: (isAdmin ? types.data ?? [] : []).map((type) => ({
      queryKey: ["item-type-fields", type.id],
      queryFn: () => api<{ items: { field_id: string }[] }>(`/api/v1/item-types/${type.id}/fields`),
    })),
  });
  const pending = isAdmin && (types.isPending || assigned.some((query) => query.isPending));
  const count = assigned.filter((query) => query.data?.items.some((entry) => entry.field_id === field.id)).length;
  return <section aria-labelledby="cf-ownership" className="space-y-2">
    <h3 id="cf-ownership" className="text-sm font-semibold">{t("spaces.customFields.sections.ownership")}</h3>
    <p className="text-sm">{t("spaces.customFields.drawer.owner", { owner: scope.spaceId ? scope.ownerName : t("spaces.customFields.workspace") })}</p>
    {isAdmin && <p className="text-sm text-muted-foreground">
      {pending ? t("spaces.customFields.usage.loading") : count > 0 ? t("spaces.customFields.drawer.appliesTo", { count }) : t("spaces.customFields.drawer.appliesNone")}
    </p>}
  </section>;
}

/** Restrict a field to chosen people and groups (CF-D03). Grants save on their own button; a new field must be saved first. */
function PermissionsSection({ field, scope, canManage }: { field: Field; scope: FieldScope; canManage: boolean }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const me = useProfile();
  const base = scope.spaceId ? `/api/v1/spaces/${scope.spaceId}/custom-fields` : "/api/v1/custom-fields";
  const key = ["custom-field-grants", field.id];
  const grants = useQuery({ queryKey: key, retry: false,
    queryFn: async () => (await api<{ items: Grant[] }>(`${base}/${field.id}/grants`)).items });
  const [rows, setRows] = useState<Grant[] | null>(null);
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const current: Grant[] = rows ?? grants.data ?? [];
  const restricted = current.length > 0;
  const people = useQuery({ queryKey: ["grant-people", query], enabled: query.trim().length >= 2,
    queryFn: async () => (await api<{ items: Person[] }>(`/api/v1/users?q=${encodeURIComponent(query.trim())}&limit=8`)).items });
  const groups = useQuery({ queryKey: ["groups"], queryFn: async () => (await api<{ items: Group[] }>("/api/v1/groups")).items });
  const groupMatches = useMemo(() => (groups.data ?? []).filter((group) => query.trim() && group.name.toLowerCase().includes(query.trim().toLowerCase())).slice(0, 5), [groups.data, query]);

  function setCurrent(next: Grant[]) { setRows(next); setError(""); }
  function add(grant: Grant) {
    if (current.some((row) => row.subject_kind === grant.subject_kind && row.subject_id === grant.subject_id)) return;
    setCurrent([...current, grant]);
    setQuery("");
  }
  async function save() {
    setBusy(true);
    setError("");
    try {
      const body = { grants: current.map(({ subject_kind, subject_id, level }) => ({ subject_kind, subject_id, level })) };
      await api(`${base}/${field.id}/grants`, { method: "PUT", body: JSON.stringify(body) });
      setRows(null);
      await client.invalidateQueries({ queryKey: key });
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return <section aria-labelledby="cf-permissions" className="space-y-3">
    <h3 id="cf-permissions" className="text-sm font-semibold">{t("spaces.customFields.sections.permissions")}</h3>
    {grants.isError && <FormError message={t("spaces.customFields.permissions.loadError")} />}
    <p className="text-sm text-muted-foreground">{restricted ? t("spaces.customFields.permissions.restrictedHelp") : t("spaces.customFields.permissions.inherit")}</p>
    {!canManage ? null : <>
      {!restricted && <Button type="button" variant="outline" size="sm" disabled={busy}
        onClick={() => me && setCurrent([{ subject_kind: "user", subject_id: me.id, level: "edit", subject_name: t("spaces.customFields.permissions.you") }])}>
        {t("spaces.customFields.permissions.restrict")}</Button>}
      {restricted && <>
        <ul className="divide-y rounded-md border">
          {current.map((row, index) => <li key={`${row.subject_kind}:${row.subject_id}`} className="flex flex-wrap items-center gap-2 px-3 py-2">
            <span className="min-w-0 flex-1 truncate text-sm">{row.subject_name || row.subject_id}
              <span className="text-muted-foreground"> · {t(`spaces.customFields.permissions.kind.${row.subject_kind}`)}</span></span>
            <select aria-label={t("spaces.customFields.permissions.levelFor", { name: row.subject_name || row.subject_id })}
              className="h-8 rounded-lg border border-input bg-background px-2 text-sm" value={row.level} disabled={busy}
              onChange={(event) => setCurrent(current.map((entry, i) => i === index ? { ...entry, level: event.target.value as Level } : entry))}>
              {LEVELS.map((level) => <option key={level} value={level}>{t(`spaces.customFields.permissions.level.${level}`)}</option>)}
            </select>
            <Button type="button" variant="ghost" size="sm" disabled={busy}
              aria-label={t("spaces.customFields.permissions.remove", { name: row.subject_name || row.subject_id })}
              onClick={() => setCurrent(current.filter((_, i) => i !== index))}>{t("spaces.customFields.permissions.removeLabel")}</Button>
          </li>)}
        </ul>
        <label className="block space-y-1 text-sm">
          <span>{t("spaces.customFields.permissions.addPerson")}</span>
          <Input type="search" value={query} placeholder={t("spaces.customFields.permissions.searchPlaceholder")} disabled={busy}
            onChange={(event) => setQuery(event.target.value)} />
        </label>
        {query.trim().length >= 2 && <ul className="max-h-48 divide-y overflow-y-auto rounded-md border">
          {(people.data ?? []).map((person) => {
            const name = `${person.first_name} ${person.last_name}`.trim() || person.email;
            return <li key={person.id}><button type="button" className="flex w-full items-center gap-2 px-3 py-2 text-start hover:bg-accent"
              onClick={() => add({ subject_kind: "user", subject_id: person.id, level: "view", subject_name: name })}>
              <InitialsAvatar name={name} size="size-6" /><span className="truncate text-sm">{name}</span></button></li>;
          })}
          {groupMatches.map((group) => <li key={group.id}><button type="button" className="flex w-full items-center gap-2 px-3 py-2 text-start hover:bg-accent"
            onClick={() => add({ subject_kind: "group", subject_id: group.id, level: "view", subject_name: group.name })}>
            <span className="truncate text-sm">{group.name}</span><span className="text-muted-foreground text-xs">{t("spaces.customFields.permissions.kind.group")}</span></button></li>)}
        </ul>}
        <div className="flex flex-wrap items-center justify-between gap-2">
          <Button type="button" variant="ghost" size="sm" disabled={busy} onClick={() => setCurrent([])}>{t("spaces.customFields.permissions.lift")}</Button>
          <Button type="button" size="sm" disabled={busy || rows === null} onClick={() => void save()}>{t("spaces.customFields.permissions.save")}</Button>
        </div>
      </>}
      <FormError message={error} />
    </>}
  </section>;
}

/** What the value looks like in Table and Item detail, from the draft as it is now. */
function PreviewSection({ draft }: { draft: FieldDraft }) {
  const { t } = useTranslation();
  const body = draftPayload(draft).body;
  const preview: CustomField = {
    id: draft.id ?? "preview", space_id: "", name: draft.name || t("spaces.customFields.drawer.untitled"), type: draft.type,
    options: draft.options, position: 0, archived_at: null,
    config: (body?.config ?? {}) as CustomField["config"],
  };
  const sample: Record<string, CustomFieldValue> = {
    number: 1234.5, currency: 1234.5, percent: 42, duration: 90, formula: 42, text: t("spaces.customFields.drawer.sampleText"),
    date: "2026-11-18", checkbox: true, single_select: draft.options[0]?.id ?? "", multi_select: draft.options.slice(0, 2).map((option) => option.id),
    people: [], relation: "",
  };
  const shown = customFieldText(preview, sample[draft.type], "en");
  return <section aria-labelledby="cf-preview" className="space-y-2">
    <h3 id="cf-preview" className="text-sm font-semibold">{t("spaces.customFields.sections.preview")}</h3>
    <p className="text-xs text-muted-foreground">{t("spaces.customFields.drawer.previewHint")}</p>
    <div className="flex flex-wrap items-center gap-2 rounded-md border p-3 text-sm">
      <span className="font-medium">{preview.name}</span>
      <span className="text-muted-foreground">{shown || "—"}</span>
    </div>
  </section>;
}

/** Where a Workspace field applies (DR-08): a Space shows its values only once the Space is assigned. Each Space's Full members toggle it. */
function WorkspaceAssignments({ field, canManage }: { field: Field; canManage: boolean }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const spaces = useSpaces();
  const manageable = (spaces.data ?? []).filter((space) => can(space, "share"));
  const choices = useQueries({
    queries: manageable.map((space) => ({
      queryKey: ["field-choices", space.id],
      queryFn: () => api<{ items: { id: string; assigned: boolean }[] }>(`/api/v1/spaces/${space.id}/field-choices`),
    })),
  });
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<string>();

  async function toggle(spaceId: string, assigned: boolean) {
    setBusy(spaceId);
    setError("");
    try {
      const base = `/api/v1/spaces/${spaceId}/custom-fields/${field.id}/assignments`;
      if (assigned) await api(`${base}/space/${spaceId}`, { method: "DELETE" });
      else await api(base, { method: "POST", body: JSON.stringify({ scope_kind: "space", scope_id: spaceId }) });
      await client.invalidateQueries({ queryKey: ["field-choices", spaceId] });
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(undefined);
    }
  }

  if (manageable.length === 0) return null;
  return <section aria-labelledby="cf-where" className="space-y-2">
    <h3 id="cf-where" className="text-sm font-semibold">{t("spaces.customFields.assign.title")}</h3>
    <p className="text-xs text-muted-foreground">{t("spaces.customFields.assign.help")}</p>
    <ul className="divide-y rounded-md border">
      {manageable.map((space, index) => {
        const choice = choices[index]?.data?.items.find((entry) => entry.id === field.id);
        const assigned = !!choice?.assigned;
        return <li key={space.id} className="flex items-center justify-between gap-2 px-3 py-2">
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={assigned} disabled={!canManage || !choice || busy === space.id}
              onChange={() => void toggle(space.id, assigned)} />
            <span>{space.name}</span>
          </label>
        </li>;
      })}
    </ul>
    <FormError message={error} />
  </section>;
}
