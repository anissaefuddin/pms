import { useState, type FormEvent } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { CAPABILITIES, roleKeys, useCustomRoles, type Capability, type CustomRole } from "@/lib/roles";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PermissionMatrix } from "@/components/permission-matrix";

export const Route = createFileRoute("/_app/settings/roles")({ component: RolesPage });

const roleViews = ["list", "matrix"] as const;

function RolesPage() {
  const { t } = useTranslation();
  const profile = useProfile();
  const admin = profile?.user_type === "admin";
  const roles = useCustomRoles(admin);
  const qc = useQueryClient();
  const [editing, setEditing] = useState<CustomRole | "new">();
  const [deleting, setDeleting] = useState<CustomRole>();
  const [error, setError] = useState("");
  const [view, setView] = useState<(typeof roleViews)[number]>("list");
  if (profile && !admin) return <p role="alert">{t("roles.adminOnly")}</p>;

  async function remove() {
    if (!deleting) return;
    setError("");
    try {
      await api(`/api/v1/custom-roles/${deleting.id}`, { method: "DELETE", headers: { "If-Match": `"${deleting.version}"` } });
      setDeleting(undefined);
      toast.success(t("roles.deleted"));
      await qc.invalidateQueries({ queryKey: roleKeys.all });
    } catch (err) { setError(err instanceof ApiError && err.status === 409 ? t("roles.inUse") : (err as Error).message); }
  }

  return <section className="space-y-5">
    <header className="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 className="text-lg font-semibold">{t("roles.title")}</h2>
        <p className="text-sm text-muted-foreground">{t("roles.description")}</p>
      </div>
      <Button onClick={() => setEditing("new")}>{t("roles.new")}</Button>
    </header>
    <div role="tablist" aria-label={t("roles.title")} className="flex gap-2 border-b">
      {roleViews.map((key) => <button key={key} type="button" role="tab" aria-selected={view === key}
        onClick={() => setView(key)} className={`-mb-px border-b-2 px-3 py-2 text-sm ${view === key ? "border-primary font-medium" : "border-transparent text-muted-foreground"}`}>
        {key === "list" ? t("roles.tabs.list") : t("roles.tabs.matrix")}</button>)}
    </div>
    {view === "matrix" && roles.data && <PermissionMatrix caption={t("roles.tabs.matrix")}
      columns={roles.data.map((role) => ({ id: role.id, label: role.name }))}
      rows={CAPABILITIES.map((cap) => ({ id: cap, label: t(`roles.capability.${cap}`) }))}
      allowed={(roleId, cap) => roles.data.find((role) => role.id === roleId)?.capabilities.includes(cap as Capability) ?? false} />}
    {view === "list" && <>
    {roles.isPending && <p role="status" className="text-sm text-muted-foreground">{t("roles.loading")}</p>}
    {roles.isError && <p role="alert">{t("roles.error")}</p>}
    {roles.data?.length === 0 && <p role="status" className="rounded-xl border border-dashed p-8 text-center text-sm text-muted-foreground">{t("roles.empty")}</p>}
    {!!roles.data?.length && <table className="w-full text-left text-sm">
      <thead><tr className="border-b">
        <th scope="col" className="py-2 pr-4">{t("roles.col.name")}</th>
        <th scope="col" className="py-2 pr-4">{t("roles.col.capabilities")}</th>
        <th scope="col" className="py-2 pr-4">{t("roles.col.inUse")}</th>
        <th scope="col" className="py-2"><span className="sr-only">{t("roles.col.actions")}</span></th>
      </tr></thead>
      <tbody>{roles.data.map((role) => <tr key={role.id} className="border-b align-top">
        <th scope="row" className="py-2 pr-4 font-medium">{role.name}{role.description && <span className="block text-xs font-normal text-muted-foreground">{role.description}</span>}</th>
        <td className="py-2 pr-4">{role.capabilities.map((c) => t(`roles.capability.${c}`)).join(", ")}</td>
        <td className="py-2 pr-4">{role.in_use}</td>
        <td className="space-x-2 py-2 text-right">
          <Button size="sm" variant="outline" aria-label={t("roles.editNamed", { name: role.name })} onClick={() => setEditing(role)}>{t("roles.edit")}</Button>
          <Button size="sm" variant="outline" aria-label={t("roles.deleteNamed", { name: role.name })} onClick={() => { setError(""); setDeleting(role); }}>{t("roles.delete")}</Button>
        </td>
      </tr>)}</tbody>
    </table>}
    {editing && <RoleForm key={editing === "new" ? "new" : editing.id} role={editing === "new" ? undefined : editing} onClose={() => setEditing(undefined)} />}
    <ConfirmDialog open={!!deleting} onOpenChange={(open) => !open && setDeleting(undefined)} error={error}
      title={t("roles.deleteTitle")} description={t("roles.deleteDescription", { name: deleting?.name ?? "" })}
      confirmLabel={t("roles.deleteConfirm")} cancelLabel={t("roles.cancel")} onConfirm={() => void remove()} />
    </>}
  </section>;
}

function RoleForm({ role, onClose }: { role?: CustomRole; onClose: () => void }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [name, setName] = useState(role?.name ?? "");
  const [description, setDescription] = useState(role?.description ?? "");
  const [caps, setCaps] = useState<Set<Capability>>(new Set(role?.capabilities ?? ["view"]));
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const toggle = (c: Capability, on: boolean) => setCaps((prev) => { const next = new Set(prev); if (on) next.add(c); else next.delete(c); return next; });
  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true); setError("");
    try {
      const body = JSON.stringify({ name, description, capabilities: CAPABILITIES.filter((c) => caps.has(c)) });
      if (role) await api(`/api/v1/custom-roles/${role.id}`, { method: "PUT", headers: { "If-Match": `"${role.version}"` }, body });
      else await api("/api/v1/custom-roles", { method: "POST", body });
      toast.success(t("roles.saved"));
      await qc.invalidateQueries({ queryKey: roleKeys.all });
      onClose();
    } catch (err) {
      setError(err instanceof ApiError && err.status === 409 ? t("roles.conflict") : (err as Error).message || t("roles.saveFailed"));
    } finally { setBusy(false); }
  }
  return <form onSubmit={(e) => void submit(e)} className="space-y-4 rounded-lg border p-4" aria-label={t(role ? "roles.editTitle" : "roles.newTitle")}>
    <h3 className="font-semibold">{t(role ? "roles.editTitle" : "roles.newTitle")}</h3>
    <label className="block text-sm">{t("roles.name")}<Input required maxLength={60} value={name} onChange={(e) => setName(e.target.value)} className="mt-1 max-w-sm" /></label>
    <label className="block text-sm">{t("roles.descriptionLabel")}<Input maxLength={300} value={description} onChange={(e) => setDescription(e.target.value)} className="mt-1" /></label>
    <fieldset className="space-y-1">
      <legend className="text-sm font-medium">{t("roles.capabilities")}</legend>
      <p className="text-sm text-muted-foreground">{t("roles.capabilitiesHint")}</p>
      <div className="grid gap-1 sm:grid-cols-2">{CAPABILITIES.map((c) => <label key={c} className="flex items-center gap-2 text-sm">
        <input type="checkbox" className="size-6" checked={caps.has(c)} disabled={c === "view"} onChange={(e) => toggle(c, e.target.checked)} />
        {t(`roles.capability.${c}`)}<span className="text-xs text-muted-foreground">{t(`roles.capabilityHelp.${c}`)}</span></label>)}</div>
    </fieldset>
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    <div className="flex gap-2">
      <Button type="submit" disabled={busy || !name.trim()}>{t("roles.save")}</Button>
      <Button type="button" variant="outline" onClick={onClose}>{t("roles.cancel")}</Button>
    </div>
  </form>;
}
