import { useRef, useState, type FormEvent } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { ApiError, api, type CurrentUser } from "@/lib/api";
import { etag, spaceKeys, type Group, type Share, type Space } from "@/lib/spaces";
import { Search } from "@/components/share-dialog";
import { FormError } from "@/components/auth/form-error";
import { TextField } from "@/components/auth/text-field";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { CustomerPicker } from "@/components/customer-picker";
import type { CustomerOption } from "@/lib/customers";

type SpaceTemplate = components["schemas"]["SpaceTemplate"];

type Props = { open: boolean; onOpenChange: (open: boolean) => void; space?: Space };

/** Create (no `space`) or rename/edit a Space. Personal spaces are never edited, the page does not offer it. */
export function SpaceDialog({ open, onOpenChange, space }: Props) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogTitle className="text-lg font-semibold">{space ? t("spaces.form.edit.title") : t("spaces.form.create.title")}</DialogTitle>
        {/* New version after a reload remounts the form with the server's values. */}
        {open && <SpaceForm key={space?.version} space={space} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function SpaceForm({ space, onDone }: { space?: Space; onDone: () => void }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [error, setError] = useState("");
  const [stale, setStale] = useState<"conflict" | "missing" | null>(null);
  const [pending, setPending] = useState(false);
  const [shares, setShares] = useState<Share[]>([]);
  const me = queryClient.getQueryData<CurrentUser>(["me"]);
  const [templateId, setTemplateId] = useState("");
  // B3-SC: every new Space states its ownership explicitly. There is no default, and no implicit internal fallback.
  const [ownership, setOwnership] = useState<"internal" | "client" | null>(null);
  const [customer, setCustomer] = useState<CustomerOption | null>(null);
  const requestId = useRef(crypto.randomUUID()); // a retry of the same dialog never builds a second Space
  const templates = useQuery({ queryKey: ["space-templates"], queryFn: async () => (await api<{ items: SpaceTemplate[] }>("/api/v1/space-templates")).items, enabled: !space });
  const template = templates.data?.find((entry) => entry.id === templateId);
  const groups = useQuery({ queryKey: ["groups"], queryFn: async () => (await api<{ items: Group[] }>("/api/v1/groups")).items, enabled: !space });

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const name = String(data.get("name") ?? "").trim();
    const description = String(data.get("description") ?? "").trim();
    if (!name) return setError(t("spaces.form.nameRequired"));
    if (!space && !ownership) return setError(t("spaces.form.ownership.required"));
    if (!space && ownership === "client" && !customer) return setError(t("spaces.form.ownership.customerRequired"));
    const scope = { scope_state: ownership ?? undefined, client_id: ownership === "client" ? customer?.id : undefined };
    setError(""); setStale(null); setPending(true);
    try {
      if (space) {
        const updated = await api<Space>(`/api/v1/spaces/${space.id}`, {
          method: "PATCH", headers: { "If-Match": etag(space) }, body: JSON.stringify({ name, description: description || null }),
        });
        queryClient.setQueryData(spaceKeys.one(space.id), updated);
        await queryClient.invalidateQueries({ queryKey: spaceKeys.all });
        toast.success(t("spaces.form.saved"));
        onDone();
      } else {
        if (template) {
          const created = await api<Space>(`/api/v1/space-templates/${template.id}/spaces`, { method: "POST", body: JSON.stringify({ name, request_id: requestId.current, ...scope }) });
          await queryClient.invalidateQueries({ queryKey: spaceKeys.all });
          toast.success(t("spaces.form.created"));
          onDone();
          return await navigate({ to: "/spaces/$spaceId", params: { spaceId: created.id } });
        }
        const access = shares.map(({ principal_type, principal_id, role }) => ({ principal_type, principal_id, role }));
        const created = await api<Space>("/api/v1/spaces", { method: "POST", body: JSON.stringify({ name, description: description || undefined, ...scope, shares: access.length ? access : undefined }) });
        await queryClient.invalidateQueries({ queryKey: spaceKeys.all });
        toast.success(t("spaces.form.created"));
        onDone();
        await navigate({ to: "/spaces/$spaceId", params: { spaceId: created.id } });
      }
    } catch (e) {
      if (e instanceof ApiError && (e.status === 409 || e.status === 428)) setStale(e.status === 409 ? "conflict" : "missing");
      else setError((e as Error).message);
    } finally {
      setPending(false);
    }
  }

  async function reload() {
    if (!space) return;
    await queryClient.invalidateQueries({ queryKey: spaceKeys.all });
    setStale(null);
  }

  return (
    <form className="space-y-3" noValidate onSubmit={submit}>
      {stale && (
        <div role="alert" className="flex items-center justify-between gap-2 rounded-lg border border-warning bg-warning-subtle p-2 text-sm">
          <span>{stale === "conflict" ? t("spaces.conflict.message") : t("spaces.conflict.missing")}</span>
          <Button type="button" size="sm" variant="outline" onClick={reload}>{t("spaces.conflict.reload")}</Button>
        </div>
      )}
      {!space && !!templates.data?.length && <div className="space-y-1.5">
        <label htmlFor="space-template" className="block text-sm font-medium">{t("spaces.template.label")}</label>
        <select id="space-template" value={templateId} onChange={(event) => setTemplateId(event.target.value)}
          className="block h-8 w-full rounded-md border border-input bg-background px-2 text-sm">
          <option value="">{t("spaces.template.blank")}</option>
          {templates.data.filter((entry) => entry.compatible).map((entry) => <option key={entry.id} value={entry.id}>{entry.saved ? t("spaces.template.saved") : entry.category}: {entry.name}</option>)}
        </select>
        {template && <p className="text-sm text-muted-foreground">{template.description} {t("spaces.template.contents", { ...template.counts })}</p>}
        {template?.mine && <Button type="button" variant="ghost" size="sm" disabled={pending} onClick={async () => {
          try {
            await api(`/api/v1/space-templates/${template.id}`, { method: "DELETE" });
            setTemplateId("");
            await queryClient.invalidateQueries({ queryKey: ["space-templates"] });
          } catch (e) { setError((e as Error).message); }
        }}>{t("spaces.template.delete")}</Button>}
      </div>}
      <TextField label={t("spaces.form.name")} name="name" defaultValue={space?.name ?? ""} maxLength={120} required autoComplete="off" />
      {!space && <fieldset className="space-y-2">
        <legend className="text-sm font-medium">{t("spaces.form.ownership.label")}</legend>
        <div role="radiogroup" aria-label={t("spaces.form.ownership.label")} className="flex flex-col gap-2">
          <label className="flex items-start gap-2 text-sm">
            <input type="radio" name="ownership" value="internal" className="mt-1" checked={ownership === "internal"}
              onChange={() => { setOwnership("internal"); setCustomer(null); }} />
            <span><span className="block font-medium">{t("spaces.form.ownership.internal")}</span>
              <span className="block text-xs text-muted-foreground">{t("spaces.form.ownership.internalHint")}</span></span>
          </label>
          <label className="flex items-start gap-2 text-sm">
            <input type="radio" name="ownership" value="client" className="mt-1" checked={ownership === "client"}
              onChange={() => setOwnership("client")} />
            <span><span className="block font-medium">{t("spaces.form.ownership.customer")}</span>
              <span className="block text-xs text-muted-foreground">{t("spaces.form.ownership.customerHint")}</span></span>
          </label>
        </div>
        {ownership === "client" && <CustomerPicker label={t("spaces.form.ownership.customerPick")} value={customer?.id ?? null} selected={customer} onChange={setCustomer} clearable />}
      </fieldset>}
      {!template && <div className="space-y-1.5">
        <label htmlFor="space-description" className="block text-sm font-medium">{t("spaces.form.description")}</label>
        <textarea id="space-description" name="description" defaultValue={space?.description ?? ""} maxLength={1000} rows={3}
          className="w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-1.5 text-base outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 md:text-sm dark:bg-input/30" />
      </div>}
      {!space && !template && me && <details>
        <summary>{t("share.access.title")}</summary>
        <Search me={me} groups={groups.data ?? []} taken={new Set(shares.map((row) => `${row.principal_type}:${row.principal_id}:direct`))}
          onAdd={(principal, role) => setShares((rows) => [...rows, { ...principal, role, inherited_from: null }])} />
        {shares.map((row) => <div key={`${row.principal_type}:${row.principal_id}`} className="flex items-center justify-between gap-2">
          <span>{row.principal_name} ({t(`share.role.${row.role}`)})</span>
          <Button type="button" variant="ghost" aria-label={t("share.access.remove", { name: row.principal_name })}
            onClick={() => setShares((rows) => rows.filter((entry) => entry !== row))}>{t("share.access.removeShort")}</Button>
        </div>)}
      </details>}
      <FormError message={error} />
      <DialogFooter>
        <Button type="submit" disabled={pending}>{space ? t("spaces.form.edit.submit") : t("spaces.form.create.submit")}</Button>
      </DialogFooter>
    </form>
  );
}
