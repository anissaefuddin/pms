import { useState, type FormEvent } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { MoreHorizontal } from "lucide-react";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { formatNumber, useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { FormError } from "@/components/auth/form-error";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { AdminEmptyState, AdminMetrics, AdminPageHeader, AdminStatus, AdminToolbar, InitialsAvatar } from "@/components/admin/admin-ui";
import { GroupsTab } from "@/components/admin/groups-tab";

export const Route = createFileRoute("/_app/settings/users/")({ component: Users });
type User = components["schemas"]["User"];
const types = ["admin", "regular", "external", "collaborator", "viewer"] as const;
const usersPath = "/api/v1/users";
const groupsPath = "/api/v1/groups";
const selectClass = "h-9 rounded-md border border-input bg-background px-2 text-sm";
const STATUS_TONE = { active: "success", invited: "warning", deactivated: "neutral" } as const;
const USER_STATUSES = ["active", "invited", "deactivated"] as const;

/** Users list: read first, change on demand. Type changes and deactivation open a dialog that names the person. */
function Users() {
  const { t } = useTranslation();
  const me = useProfile();
  const client = useQueryClient();
  const [tab, setTab] = useState<"users" | "groups">("users");
  const [inviting, setInviting] = useState(false);
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [typeChange, setTypeChange] = useState<{ user: User; type: string }>();
  const [confirmation, setConfirmation] = useState<{ title: string; body: string; confirm: string; path: string; method: string; user?: User }>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const seats = useQuery({ queryKey: ["seats"], queryFn: () => api<components["schemas"]["Seats"]>("/api/v1/seats") });
  const users = useInfiniteQuery({
    queryKey: ["admin-users"], initialPageParam: "",
    queryFn: ({ pageParam }) => api<components["schemas"]["UserPage"]>(usersPath + (pageParam ? `?cursor=${encodeURIComponent(pageParam)}` : "")),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
  const people = users.data?.pages.flatMap((page) => page.items) ?? [];
  const visible = people.filter((user) => {
    const text = `${user.first_name} ${user.last_name} ${user.email}`.toLowerCase();
    return (!query || text.includes(query.toLowerCase()))
      && (!statusFilter || user.status === statusFilter)
      && (!typeFilter || user.user_type === typeFilter);
  });
  const name = (user: User) => `${user.first_name} ${user.last_name}`.trim() || user.email;
  const number = (value: number) => formatNumber(value, me);

  async function mutate(path: string, method: string, body?: unknown) {
    if (busy) return false;
    setBusy(true);
    setError("");
    try {
      await api(path, { method, body: body === undefined ? undefined : JSON.stringify(body) });
      await Promise.all(["admin-users", "seats", "groups", "mention-users"].map((key) => client.invalidateQueries({ queryKey: [key] })));
      return true;
    } catch (err) {
      setError((err as Error).message);
      return false;
    } finally { setBusy(false); }
  }

  async function invite(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const fields = Object.fromEntries(new FormData(event.currentTarget));
    if (await mutate(usersPath + "/invitations", "POST", fields)) setInviting(false);
  }

  async function changeType() {
    if (!typeChange) return;
    if (await mutate(`${usersPath}/${typeChange.user.id}`, "PATCH", { user_type: typeChange.type })) setTypeChange(undefined);
  }

  async function confirmAction() {
    if (confirmation && await mutate(confirmation.path, confirmation.method)) setConfirmation(undefined);
  }

  const invitePanel = <Dialog open={inviting} onOpenChange={setInviting}>
    <DialogContent className="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{t("settings.users.invite")}</DialogTitle>
        <DialogDescription>{t("settings.users.inviteDescription", { workspace: me?.account_name ?? "" })}</DialogDescription>
      </DialogHeader>
      <form className="space-y-4" onSubmit={invite}>
        <label className="block space-y-1 text-sm">{t("settings.users.inviteEmail")}<Input name="email" type="email" required placeholder={t("settings.users.emailPlaceholder")} /></label>
        <TypeSelect name="user_type" defaultValue="regular" />
        <label className="block space-y-1 text-sm">{t("settings.users.message")}<textarea name="message" maxLength={500} className="block w-full rounded-md border border-input bg-background p-2 text-sm" /></label>
        <p className="text-sm text-muted-foreground">{t("settings.users.seatsAvailable", { count: seats.data?.available ?? 0 })}</p>
        <FormError message={error} />
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => setInviting(false)}>{t("items.cancel")}</Button>
          <Button type="submit" disabled={busy || seats.data?.available === 0}>{t("settings.users.send")}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>;

  return <section className="min-w-0 space-y-6">
    <AdminPageHeader description={t("settings.users.description")}
      action={tab === "users" ? <Button onClick={() => setInviting(true)} disabled={seats.data?.available === 0}>{t("settings.users.invite")}</Button> : undefined} />

    {seats.data && <AdminMetrics usage={{ used: seats.data.used, max: seats.data.max, label: t("settings.users.capacity") }}
      items={[
        { label: t("settings.users.activeUsers"), value: number(seats.data.used) },
        { label: t("settings.users.invitedUsers"), value: number(seats.data.pending) },
        { label: t("settings.users.availableSeats"), value: number(seats.data.available) },
      ]} />}

    <nav aria-label={t("settings.users.sections")} className="flex gap-4 border-b text-sm">
      <button type="button" aria-current={tab === "users" ? "page" : undefined} onClick={() => setTab("users")}
        className={`-mb-px border-b-2 px-1 pb-2 ${tab === "users" ? "border-primary font-medium" : "border-transparent text-muted-foreground hover:text-foreground"}`}>{t("settings.users.title")}</button>
      <button type="button" aria-current={tab === "groups" ? "page" : undefined} onClick={() => setTab("groups")}
        className={`-mb-px border-b-2 px-1 pb-2 ${tab === "groups" ? "border-primary font-medium" : "border-transparent text-muted-foreground hover:text-foreground"}`}>{t("settings.users.groups")}</button>
      <Link to="/settings/users/types" className="border-b-2 border-transparent px-1 pb-2 text-muted-foreground hover:text-foreground">{t("settings.users.types")}</Link>
    </nav>

    <FormError message={error} />
    {(seats.isError || users.isError) && <div>
      <FormError message={t("items.loadError")} />
      <Button variant="link" onClick={() => { void users.refetch(); void seats.refetch(); }}>{t("items.retry")}</Button>
    </div>}

    {tab === "users" ? <>
      <AdminToolbar searchLabel={t("settings.users.searchPlaceholder")} search={query} onSearch={setQuery}>
        <select aria-label={t("settings.users.statusFilter")} className={selectClass} value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
          <option value="">{t("settings.users.allStatuses")}</option>
          {USER_STATUSES.map((value) => <option key={value} value={value}>{t(`settings.users.status.${value}`)}</option>)}
        </select>
        <select aria-label={t("settings.users.typeFilter")} className={selectClass} value={typeFilter} onChange={(event) => setTypeFilter(event.target.value)}>
          <option value="">{t("settings.users.allTypes")}</option>
          {types.map((type) => <option key={type} value={type}>{t(`settings.users.types.${type}`)}</option>)}
        </select>
      </AdminToolbar>

      {visible.length ? <div className="overflow-x-auto rounded-lg border">
        <table className="w-full text-start text-sm">
          <caption className="sr-only">{t("settings.users.title")}</caption>
          <thead className="sticky top-0 bg-muted/60 text-xs font-medium text-foreground">
            <tr>
              <th scope="col" className="px-3 py-2 font-medium">{t("settings.users.colUser")}</th>
              <th scope="col" className="px-3 py-2 font-medium">{t("settings.users.type")}</th>
              <th scope="col" className="px-3 py-2 font-medium">{t("items.status")}</th>
              <th scope="col" className="w-12 px-3 py-2"><span className="sr-only">{t("items.actions")}</span></th>
            </tr>
          </thead>
          <tbody className="divide-y">{visible.map((user) => <tr key={user.id} className="hover:bg-accent/40">
            <td className="px-3 py-2.5">
              <div className="flex items-center gap-3">
                <InitialsAvatar name={name(user)} />
                <div className="min-w-0"><p className="truncate font-medium">{name(user)}</p><p className="truncate text-xs text-muted-foreground">{user.email}</p></div>
              </div>
            </td>
            <td className="px-3 py-2.5">
              <span className="inline-flex items-center gap-2">{t(`settings.users.types.${user.user_type}`)}
                <Button size="sm" variant="ghost" disabled={busy} aria-label={t("settings.users.changeTypeFor", { name: name(user) })}
                  onClick={() => setTypeChange({ user, type: user.user_type })}>{t("settings.users.changeType")}</Button>
              </span>
            </td>
            <td className="px-3 py-2.5"><AdminStatus tone={STATUS_TONE[user.status as keyof typeof STATUS_TONE] ?? "neutral"} label={t(`settings.users.status.${user.status}`)} /></td>
            <td className="px-3 py-2.5 text-end">
              <DropdownMenu>
                <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label={t("settings.users.moreActions", { name: name(user) })} />}>
                  <MoreHorizontal />
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  {user.status === "invited" && <DropdownMenuItem onClick={() => void mutate(`${usersPath}/${user.id}/invitation/resend`, "POST")}>{t("settings.users.resend")}</DropdownMenuItem>}
                  {user.status === "deactivated"
                    ? <DropdownMenuItem onClick={() => void mutate(`${usersPath}/${user.id}/reactivate`, "POST")}>{t("settings.users.reactivate")}</DropdownMenuItem>
                    : <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => setConfirmation({
                      title: t("settings.users.deactivateTitle", { name: name(user) }),
                      body: t("settings.users.deactivateBody", { name: name(user), email: user.email }),
                      confirm: t("settings.users.deactivateConfirm"),
                      path: `${usersPath}/${user.id}/deactivate`, method: "POST", user,
                    })}>{t("settings.users.deactivate")}</DropdownMenuItem>}
                </DropdownMenuContent>
              </DropdownMenu>
            </td>
          </tr>)}</tbody>
        </table>
      </div> : <AdminEmptyState title={t("settings.users.emptyTitle")} description={t("settings.users.emptyDescription")} />}
      {users.hasNextPage && <Button variant="outline" disabled={users.isFetchingNextPage} onClick={() => void users.fetchNextPage()}>{t("settings.users.more")}</Button>}
    </> : <GroupsTab people={people} />}

    {invitePanel}

    <Dialog open={!!typeChange} onOpenChange={(open) => { if (!open) setTypeChange(undefined); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("settings.users.changeTypeTitle")}</DialogTitle>
          <DialogDescription>{typeChange && t("settings.users.changeTypeBody", { name: name(typeChange.user) })}</DialogDescription>
        </DialogHeader>
        {typeChange && <div className="space-y-4">
          <TypeSelect name="type" value={typeChange.type} onChange={(value) => setTypeChange({ ...typeChange, type: value })} />
          {typeChange.type === "admin" && <p role="note" className="rounded-md bg-amber-500/15 px-3 py-2 text-sm">{t("settings.users.adminConsequence")}</p>}
          <FormError message={error} />
        </div>}
        <DialogFooter>
          <Button variant="outline" onClick={() => setTypeChange(undefined)}>{t("items.cancel")}</Button>
          <Button disabled={busy || !typeChange || typeChange.type === typeChange.user.user_type} onClick={() => void changeType()}>{t("settings.users.saveType")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <ConfirmDialog open={!!confirmation} onOpenChange={(open) => { if (!open) setConfirmation(undefined); }}
      title={confirmation?.title ?? ""} description={confirmation?.body ?? ""}
      confirmLabel={confirmation?.confirm ?? ""} cancelLabel={t("items.cancel")} busy={busy} error={error}
      onConfirm={() => void confirmAction()} />
  </section>;
}

/** Labelled user type select with the meaning of the chosen type shown beneath it. */
function TypeSelect({ name, defaultValue, value, onChange }: { name?: string; defaultValue?: string; value?: string; onChange?: (value: string) => void }) {
  const { t } = useTranslation();
  const current = value ?? defaultValue ?? "regular";
  return <label className="block space-y-1 text-sm">{t("settings.users.type")}
    <select name={name} className="block h-9 w-full rounded-md border border-input bg-background px-2 text-sm" value={value} defaultValue={value === undefined ? defaultValue : undefined}
      onChange={(event) => onChange?.(event.target.value)}>
      {types.map((type) => <option key={type} value={type}>{t(`settings.users.types.${type}`)}</option>)}
    </select>
    <span className="block text-xs text-muted-foreground">{t(`settings.users.typeHints.${current}`)}</span>
  </label>;
}

