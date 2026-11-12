import { useState, type FormEvent } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { formatNumber, useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { FormError } from "@/components/auth/form-error";
import { ConfirmDialog } from "@/components/confirm-dialog";

export const Route = createFileRoute("/_app/settings/users/")({ component: Users });
type User = components["schemas"]["User"];
type Group = components["schemas"]["Group"];
const types = ["admin", "regular", "external", "collaborator", "viewer"] as const;
const usersPath = "/api/v1/users";
const groupsPath = "/api/v1/groups";
const selectClass = "rounded border bg-background px-2 py-1 text-sm";

function Users() {
  const { t } = useTranslation();
  const me = useProfile();
  const client = useQueryClient();
  const [tab, setTab] = useState<"users" | "groups">("users");
  const [inviting, setInviting] = useState(false);
  const [group, setGroup] = useState<Group | "new">();
  const [members, setMembers] = useState<string[]>([]);
  const [confirmation, setConfirmation] = useState<{ title: string; path: string; method: string }>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const seats = useQuery({ queryKey: ["seats"], queryFn: () => api<components["schemas"]["Seats"]>("/api/v1/seats") });
  const users = useInfiniteQuery({
    queryKey: ["admin-users"], initialPageParam: "",
    queryFn: ({ pageParam }) => api<components["schemas"]["UserPage"]>(usersPath + (pageParam ? `?cursor=${encodeURIComponent(pageParam)}` : "")),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
  const groups = useQuery({ queryKey: ["groups"], queryFn: async () => (await api<{ items: Group[] }>(groupsPath)).items });
  const people = users.data?.pages.flatMap((page) => page.items) ?? [];
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

  function editGroup(next: Group | "new") {
    setGroup(next);
    setMembers(next === "new" ? [] : next.member_ids ?? []);
  }

  async function saveGroup(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!group) return;
    const name = new FormData(event.currentTarget).get("name");
    if (await mutate(group === "new" ? groupsPath : `${groupsPath}/${group.id}`, group === "new" ? "POST" : "PUT",
      { name, member_ids: members })) setGroup(undefined);
  }

  return <section className="min-w-0 space-y-4">
    <div className="flex flex-wrap items-center justify-between gap-2">
      <h2 className="text-xl font-semibold">{t("settings.users.title")}</h2>
      <Link to="/settings/users/types" className="underline">{t("settings.users.types")}</Link>
    </div>
    {seats.data && <p>{t("settings.users.seats", { used: number(seats.data.used), pending: number(seats.data.pending),
      available: number(seats.data.available), max: number(seats.data.max) })}</p>}
    <div className="flex gap-2">
      <Button variant="outline" aria-pressed={tab === "users"} onClick={() => setTab("users")}>{t("settings.users.title")}</Button>
      <Button variant="outline" aria-pressed={tab === "groups"} onClick={() => setTab("groups")}>{t("settings.users.groups")}</Button>
    </div>
    {!confirmation && <FormError message={error} />}
    {(seats.isError || users.isError || (tab === "groups" && groups.isError)) && <div>
      <FormError message={t("items.loadError")} />
      <Button variant="link" onClick={() => { void users.refetch(); void seats.refetch(); void groups.refetch(); }}>{t("items.retry")}</Button>
    </div>}
    {tab === "users" ? <>
      <Button disabled={seats.data?.available === 0} onClick={() => setInviting(!inviting)}>{t("settings.users.invite")}</Button>
      {inviting && <form className="space-y-3 rounded border p-3" onSubmit={invite}>
        <label className="block">{t("auth.fields.email")}<Input name="email" type="email" required /></label>
        <label className="flex items-center gap-2">{t("settings.users.type")}
          <select name="user_type" className={selectClass} defaultValue="regular">{types.map((type) => <option key={type}>{type}</option>)}</select>
        </label>
        <label className="block">{t("settings.users.message")}<textarea name="message" maxLength={500} className="block w-full rounded border bg-background p-2" /></label>
        <Button type="submit" disabled={busy}>{t("settings.users.send")}</Button>
      </form>}
      <div className="overflow-x-auto"><table className="w-full text-left text-sm">
        <thead><tr className="border-b">
          <th scope="col">{t("items.name")}</th><th scope="col">{t("auth.fields.email")}</th>
          <th scope="col">{t("settings.users.type")}</th><th scope="col">{t("items.status")}</th><th scope="col">{t("items.actions")}</th>
        </tr></thead>
        <tbody>{people.map((user) => <tr key={user.id} className="border-b">
          <td className="p-2">{name(user)}</td><td className="p-2">{user.email}</td>
          <td className="p-2"><select className={selectClass} value={user.user_type} disabled={busy}
            aria-label={t("settings.users.typeFor", { name: name(user) })}
            onChange={(event) => void mutate(`${usersPath}/${user.id}`, "PATCH", { user_type: event.target.value })}>
            {types.map((type) => <option key={type}>{type}</option>)}</select></td>
          <td className="p-2">{t(`settings.users.status.${user.status}`)}</td>
          <td className="space-x-2 p-2">
            {user.status === "invited" && <Button size="sm" variant="outline" disabled={busy}
              onClick={() => void mutate(`${usersPath}/${user.id}/invitation/resend`, "POST")}>{t("settings.users.resend")}</Button>}
            {user.status === "deactivated" ? <Button size="sm" variant="outline" disabled={busy}
              onClick={() => void mutate(`${usersPath}/${user.id}/reactivate`, "POST")}>{t("settings.users.reactivate")}</Button>
              : <Button size="sm" variant="outline" disabled={busy} onClick={() => setConfirmation({
                title: t("settings.users.deactivate"), path: `${usersPath}/${user.id}/deactivate`, method: "POST",
              })}>{t("settings.users.deactivate")}</Button>}
          </td>
        </tr>)}</tbody>
      </table></div>
    </> : <>
      <Button onClick={() => editGroup("new")}>{t("settings.users.newGroup")}</Button>
      {group && <form className="space-y-3 rounded border p-3" key={group === "new" ? group : group.id} onSubmit={saveGroup}>
        <label className="block">{t("items.name")}<Input name="name" required maxLength={80} defaultValue={group === "new" ? "" : group.name} /></label>
        <fieldset><legend>{t("settings.users.members")}</legend>
          {people.filter((user) => user.status === "active" || members.includes(user.id)).map((user) => <label key={user.id} className="flex items-center gap-2">
            <input type="checkbox" checked={members.includes(user.id)} onChange={(event) => setMembers(event.target.checked
              ? [...members, user.id] : members.filter((id) => id !== user.id))} />{name(user)}
          </label>)}
        </fieldset>
        <Button type="submit" disabled={busy}>{t("items.save")}</Button>
      </form>}
      <ul className="divide-y">{groups.data?.map((entry) => <li key={entry.id} className="flex flex-wrap items-center gap-2 py-2">
        <strong className="mr-auto">{entry.name}</strong>
        <Button size="sm" variant="outline" onClick={() => editGroup(entry)}>{t("settings.users.editGroup")}</Button>
        <Button size="sm" variant="outline" onClick={() => setConfirmation({
          title: t("settings.users.deleteGroup"), path: `${groupsPath}/${entry.id}`, method: "DELETE",
        })}>{t("settings.users.deleteGroup")}</Button>
      </li>)}</ul>
    </>}
    {users.hasNextPage && (tab === "users" || group) && <Button variant="outline" disabled={users.isFetchingNextPage}
      onClick={() => void users.fetchNextPage()}>{t("settings.users.more")}</Button>}
    <ConfirmDialog open={!!confirmation} onOpenChange={(open) => { if (!open) setConfirmation(undefined); }}
      title={confirmation?.title ?? ""} description={t("settings.users.confirm")}
      confirmLabel={confirmation?.title ?? ""} cancelLabel={t("items.cancel")} busy={busy} error={error}
      onConfirm={async () => {
        if (confirmation && await mutate(confirmation.path, confirmation.method)) setConfirmation(undefined);
      }} />
  </section>;
}
