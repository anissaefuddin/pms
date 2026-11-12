import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Users, User as UserIcon } from "lucide-react";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { api, type CurrentUser } from "@/lib/api";
import { useDebounced } from "@/lib/space-queries";
import { SHARE_ROLES, ownRank, type Group, type Share, type ShareInput, type ShareRole, type User } from "@/lib/spaces";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

export type ShareDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Already translated, e.g. `Share "Marketing"`. */
  title: string;
  /** Unique per share target; the shares query is cached under it and dropped when the dialog closes. */
  queryKey: readonly unknown[];
  loadShares: () => Promise<{ items: Share[]; etag: string }>;
  /** Receives only the direct (non-inherited) rows and must replace the target's whole share list. */
  saveShares: (shares: ShareInput[], etag: string) => Promise<unknown>;
  /** Whether the viewer holds `share` on the target; without it the dialog is a read-only list. */
  canShare: boolean;
  me: CurrentUser;
  onSaved?: () => void;
};

const selectClass = "h-8 rounded-lg border border-input bg-background px-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50";
const keyOf = (r: Pick<Share, "principal_type" | "principal_id" | "inherited_from">) => `${r.principal_type}:${r.principal_id}:${r.inherited_from ?? "direct"}`;

export function ShareDialog({ open, onOpenChange, title, ...rest }: ShareDialogProps) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="text-lg font-semibold">{title}</DialogTitle>
          <DialogDescription>{t("share.description")}</DialogDescription>
        </DialogHeader>
        <Loader {...rest} onClose={() => onOpenChange(false)} />
      </DialogContent>
    </Dialog>
  );
}

type LoaderProps = Omit<ShareDialogProps, "open" | "onOpenChange" | "title"> & { onClose: () => void };

function Loader(props: LoaderProps) {
  const { t } = useTranslation();
  // gcTime 0: the draft is seeded from a fresh load every time the dialog opens.
  const shares = useQuery({ queryKey: [...props.queryKey, "shares"], queryFn: props.loadShares, gcTime: 0, staleTime: Infinity, refetchOnWindowFocus: false });
  if (shares.isError) return <FormError message={t("share.loadError")} />;
  if (!shares.data) return <p role="status" className="text-sm text-muted-foreground">{t("share.loading")}</p>;
  return <Editor {...props} initial={shares.data.items} etag={shares.data.etag} />;
}

function Editor({ initial, etag, saveShares, canShare, me, onSaved, onClose }: LoaderProps & { initial: Share[]; etag: string }) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState(initial);
  const [error, setError] = useState("");
  const [confirming, setConfirming] = useState(false);
  const [pending, setPending] = useState(false);
  const groups = useQuery({ queryKey: ["groups"], queryFn: async () => (await api<{ items: Group[] }>("/api/v1/groups")).items, enabled: canShare });

  const roleLabel = (role: ShareRole) => t(`share.role.${role}`);
  const direct = draft.filter((r) => !r.inherited_from);
  const dirty = JSON.stringify(direct.map(({ principal_type, principal_id, role }) => [principal_type, principal_id, role]))
    !== JSON.stringify(initial.filter((r) => !r.inherited_from).map(({ principal_type, principal_id, role }) => [principal_type, principal_id, role]));
  const losesOwnAccess = ownRank(draft, me.id, groups.data ?? []) < ownRank(initial, me.id, groups.data ?? []);

  const setRole = (row: Share, role: ShareRole) => setDraft((d) => d.map((r) => (keyOf(r) === keyOf(row) ? { ...r, role } : r)));
  const remove = (row: Share) => setDraft((d) => d.filter((r) => keyOf(r) !== keyOf(row)));
  const add = (principal: Pick<Share, "principal_type" | "principal_id" | "principal_name">, role: ShareRole) =>
    setDraft((d) => [...d, { ...principal, role, inherited_from: null }]);

  async function save() {
    setPending(true); setError("");
    try {
      await saveShares(direct.map(({ principal_type, principal_id, role }) => ({ principal_type, principal_id, role })), etag);
      toast.success(t("share.saved"));
      onSaved?.();
      onClose();
    } catch (e) {
      setError((e as Error).message);
      setConfirming(false);
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="space-y-4">
      {canShare && <Search me={me} groups={groups.data ?? []} taken={new Set(direct.map(keyOf))} onAdd={add} />}
      <section aria-labelledby="share-access" className="space-y-2">
        <h3 id="share-access" className="text-sm font-medium">{t("share.access.title")}</h3>
        {draft.length === 0 ? <p className="text-sm text-muted-foreground">{t("share.access.empty")}</p> : (
          <ul className="divide-y rounded-lg border">
            {draft.map((row) => (
              <li key={keyOf(row)} className="flex items-center gap-2 p-2">
                <span className="text-muted-foreground">{row.principal_type === "group" ? <Users /> : <UserIcon />}</span>
                <span className="min-w-0 flex-1 truncate text-sm">{row.principal_name}</span>
                {row.inherited_from ? (
                  <>
                    <span className="rounded-full bg-muted px-2 py-0.5 text-xs" title={t("share.access.inheritedHelp")}>{t("share.access.inherited")}</span>
                    <span className="w-24 text-right text-sm text-muted-foreground">{roleLabel(row.role)}</span>
                  </>
                ) : canShare ? (
                  <>
                    <select className={selectClass} aria-label={t("share.access.role", { name: row.principal_name })} value={row.role} onChange={(e) => setRole(row, e.target.value as ShareRole)}>
                      {SHARE_ROLES.map((r) => <option key={r} value={r}>{roleLabel(r)}</option>)}
                    </select>
                    <Button type="button" variant="ghost" size="sm" aria-label={t("share.access.remove", { name: row.principal_name })} onClick={() => remove(row)}>{t("share.access.removeShort")}</Button>
                  </>
                ) : <span className="w-24 text-right text-sm text-muted-foreground">{roleLabel(row.role)}</span>}
              </li>
            ))}
          </ul>
        )}
      </section>
      {canShare && (
        <section className="space-y-1">
          <h3 className="text-xs font-medium">{t("share.roles.label")}</h3>
          <dl className="space-y-1 text-xs text-muted-foreground">
            {[...SHARE_ROLES].reverse().map((r) => <div key={r} className="flex gap-1"><dt className="font-medium text-foreground">{roleLabel(r)}</dt><dd>{t(`share.role.${r}.help`)}</dd></div>)}
          </dl>
        </section>
      )}
      <FormError message={error} />
      {confirming && <p role="alert" className="rounded-lg border border-warning bg-warning-subtle p-2 text-sm">{t("share.self.warning")}</p>}
      <DialogFooter>
        {!canShare ? <Button variant="outline" onClick={onClose}>{t("share.close")}</Button>
          : confirming ? (
            <>
              <Button variant="outline" onClick={() => setConfirming(false)}>{t("share.self.back")}</Button>
              <Button variant="destructive" disabled={pending} onClick={save}>{t("share.self.confirm")}</Button>
            </>
          ) : (
            <>
              <Button variant="outline" onClick={onClose}>{t("share.cancel")}</Button>
              <Button disabled={!dirty || pending} onClick={() => (losesOwnAccess ? setConfirming(true) : save())}>{pending ? t("share.saving") : t("share.save")}</Button>
            </>
          )}
      </DialogFooter>
    </div>
  );
}

type Found = Pick<Share, "principal_type" | "principal_id" | "principal_name"> & { detail?: string };

export function Search({ me, groups, taken, onAdd }: { me: CurrentUser; groups: Group[]; taken: Set<string>; onAdd: (p: Found, role: ShareRole) => void }) {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const [roles, setRoles] = useState<Record<string, ShareRole>>({});
  const term = useDebounced(q.trim(), 200);
  const active = term.length >= 2;
  const users = useQuery({ queryKey: ["share-users", term], enabled: active, queryFn: () => api<{ items: User[] }>(`/api/v1/users?q=${encodeURIComponent(term)}&limit=8`) });

  const found: Found[] = !active ? [] : [
    ...(users.data?.items ?? []).filter((u) => u.status !== "deactivated")
      .map((u): Found => ({ principal_type: "user", principal_id: u.id, principal_name: `${u.first_name} ${u.last_name}`.trim(), detail: u.email })),
    ...groups.filter((g) => g.name.toLowerCase().includes(term.toLowerCase()))
      .map((g): Found => ({ principal_type: "group", principal_id: g.id, principal_name: g.name })),
  ];
  const results = found.filter((f) => !taken.has(`${f.principal_type}:${f.principal_id}:direct`));
  const settled = active && term === q.trim() && !users.isFetching;
  const unknownEmail = settled && users.isSuccess && term.includes("@") && !users.data.items.length;

  return (
    <div className="space-y-2">
      <label htmlFor="share-search" className="block text-sm font-medium">{t("share.search.label")}</label>
      <Input id="share-search" type="search" autoComplete="off" value={q} placeholder={t("share.search.placeholder")} aria-describedby="share-search-hint" onChange={(e) => setQ(e.target.value)} />
      <p id="share-search-hint" className="text-xs text-muted-foreground">{active ? "" : t("share.search.hint")}</p>
      <div role="status" className="text-sm text-muted-foreground">
        {active && !settled ? t("share.search.loading") : settled && !results.length && !unknownEmail ? t("share.search.empty") : ""}
      </div>
      {settled && results.length > 0 && (
        <ul className="divide-y rounded-lg border">
          {results.map((f) => {
            const key = `${f.principal_type}:${f.principal_id}`;
            return (
              <li key={key} className="flex items-center gap-2 p-2">
                <span className="text-muted-foreground">{f.principal_type === "group" ? <Users /> : <UserIcon />}</span>
                <span className="min-w-0 flex-1 text-sm"><span className="block truncate">{f.principal_name}</span>{f.detail && <span className="block truncate text-xs text-muted-foreground">{f.detail}</span>}</span>
                <select className={selectClass} aria-label={t("share.search.role", { name: f.principal_name })} value={roles[key] ?? "read"} onChange={(e) => setRoles({ ...roles, [key]: e.target.value as ShareRole })}>
                  {SHARE_ROLES.map((r) => <option key={r} value={r}>{t(`share.role.${r}`)}</option>)}
                </select>
                <Button type="button" size="sm" variant="outline" aria-label={t("share.search.add", { name: f.principal_name })} onClick={() => { onAdd(f, roles[key] ?? "read"); setQ(""); }}>{t("share.search.addButton")}</Button>
              </li>
            );
          })}
        </ul>
      )}
      {unknownEmail && (
        <div className="flex items-center justify-between gap-2 rounded-lg border p-2 text-sm">
          <span role="alert">{t("share.search.noUser")}</span>
          {me.user_type === "admin" && (
            // The invite endpoint ships with user management; a disabled button cannot show a tooltip, so the wrapper carries it.
            <span title={t("share.search.inviteSoon")}><Button type="button" size="sm" variant="outline" disabled>{t("share.search.invite")}</Button></span>
          )}
        </div>
      )}
    </div>
  );
}
