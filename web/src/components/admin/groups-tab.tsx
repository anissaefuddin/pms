import { useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { FormError } from "@/components/auth/form-error";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AdminEmptyState, AdminPageHeader, AdminToolbar, InitialsAvatar } from "@/components/admin/admin-ui";
import { MemberPicker, type PickableUser } from "@/components/admin/member-picker";

type User = components["schemas"]["User"];
type Group = components["schemas"]["Group"];
const groupsPath = "/api/v1/groups";

/** Groups list (name, members, actions) with the create and edit form in a dialog, so the list never moves. */
export function GroupsTab({ people }: { people: User[] }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [editing, setEditing] = useState<Group | "new">();
  const [members, setMembers] = useState<string[]>([]);
  const [query, setQuery] = useState("");
  const [removing, setRemoving] = useState<Group>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const groups = useQuery({ queryKey: ["groups"], queryFn: async () => (await api<{ items: Group[] }>(groupsPath)).items });
  const people2: PickableUser[] = people.filter((user) => user.status === "active" || members.includes(user.id))
    .map((user) => ({ id: user.id, name: `${user.first_name} ${user.last_name}`.trim() || user.email, email: user.email }));
  const list = groups.data ?? [];

  async function mutate(path: string, method: string, body?: unknown) {
    setBusy(true);
    setError("");
    try {
      await api(path, { method, body: body === undefined ? undefined : JSON.stringify(body) });
      await Promise.all(["groups", "admin-users", "mention-users"].map((key) => client.invalidateQueries({ queryKey: [key] })));
      return true;
    } catch (err) {
      setError((err as Error).message);
      return false;
    } finally { setBusy(false); }
  }

  function open(next: Group | "new") {
    setEditing(next);
    setMembers(next === "new" ? [] : next.member_ids ?? []);
    setError("");
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editing) return;
    const name = String(new FormData(event.currentTarget).get("name") ?? "").trim();
    const isNew = editing === "new";
    if (await mutate(isNew ? groupsPath : `${groupsPath}/${editing.id}`, isNew ? "POST" : "PUT", { name, member_ids: members })) setEditing(undefined);
  }

  async function remove() {
    if (removing && await mutate(`${groupsPath}/${removing.id}`, "DELETE")) setRemoving(undefined);
  }

  const visible = list.filter((group) => group.name.toLowerCase().includes(query.toLowerCase()));
  return <>
    <AdminPageHeader description={t("settings.users.groupsDescription")}
      action={<Button onClick={() => open("new")}>{t("settings.users.newGroup")}</Button>} />
    <AdminToolbar searchLabel={t("settings.users.searchGroups")} search={query} onSearch={setQuery} />
    <FormError message={error} />
    {groups.isError && <FormError message={t("items.loadError")} />}
    {visible.length ? <div className="overflow-x-auto rounded-lg border">
      <table className="w-full text-start text-sm">
        <caption className="sr-only">{t("settings.users.groups")}</caption>
        <thead className="bg-muted/60 text-xs font-medium text-foreground">
          <tr>
            <th scope="col" className="px-3 py-2 font-medium">{t("settings.users.groupName")}</th>
            <th scope="col" className="px-3 py-2 font-medium">{t("settings.users.members")}</th>
            <th scope="col" className="px-3 py-2"><span className="sr-only">{t("items.actions")}</span></th>
          </tr>
        </thead>
        <tbody className="divide-y">{visible.map((group) => <tr key={group.id}>
          <td className="px-3 py-2.5 font-medium">{group.name}</td>
          <td className="px-3 py-2.5">
            <span className="inline-flex items-center gap-2">
              <span className="flex -space-x-2">{(group.member_ids ?? []).slice(0, 3).map((id) => {
                const user = people.find((entry) => entry.id === id);
                return <InitialsAvatar key={id} name={user ? `${user.first_name} ${user.last_name}` : ""} size="size-6" />;
              })}</span>
              <span className="text-muted-foreground">{t("settings.users.memberCount", { count: group.member_ids?.length ?? 0 })}</span>
            </span>
          </td>
          <td className="px-3 py-2.5 text-end">
            <span className="inline-flex gap-1">
              <Button size="sm" variant="outline" aria-label={t("settings.users.editGroupNamed", { name: group.name })} onClick={() => open(group)}>{t("settings.users.editGroup")}</Button>
              <Button size="sm" variant="ghost" aria-label={t("settings.users.deleteGroupNamed", { name: group.name })} onClick={() => setRemoving(group)}>{t("settings.users.deleteGroup")}</Button>
            </span>
          </td>
        </tr>)}</tbody>
      </table>
    </div> : <AdminEmptyState title={t("settings.users.groupsEmptyTitle")} description={t("settings.users.groupsEmptyDescription")} />}

    <Dialog open={!!editing} onOpenChange={(open) => { if (!open) setEditing(undefined); }}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{editing === "new" ? t("settings.users.newGroup") : t("settings.users.editGroup")}</DialogTitle>
          <DialogDescription>{t("settings.users.groupDialogHint")}</DialogDescription>
        </DialogHeader>
        {editing && <form key={editing === "new" ? "new" : editing.id} className="space-y-4" onSubmit={save}>
          <label className="block space-y-1 text-sm">{t("settings.users.groupName")}<Input name="name" required maxLength={80} defaultValue={editing === "new" ? "" : editing.name} /></label>
          <MemberPicker users={people2} selected={members} onChange={setMembers} />
          <FormError message={error} />
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setEditing(undefined)}>{t("items.cancel")}</Button>
            <Button type="submit" disabled={busy}>{t("items.save")}</Button>
          </DialogFooter>
        </form>}
      </DialogContent>
    </Dialog>

    <ConfirmDialog open={!!removing} onOpenChange={(open) => { if (!open) setRemoving(undefined); }}
      title={t("settings.users.deleteGroupTitle", { name: removing?.name ?? "" })}
      description={t("settings.users.deleteGroupBody")}
      confirmLabel={t("settings.users.deleteGroup")} cancelLabel={t("items.cancel")} busy={busy} error={error}
      onConfirm={() => void remove()} />
  </>;
}
