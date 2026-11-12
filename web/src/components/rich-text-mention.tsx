import { useState } from "react";
import { Node, type Editor } from "@tiptap/core";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import type { Group, User } from "@/lib/spaces";
import { useDebounced } from "@/lib/space-queries";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export const mention = Node.create({
  name: "mention", group: "inline", inline: true, atom: true,
  addAttributes() { return { kind: { default: null }, id: { default: null }, label: { default: "" } }; },
  renderHTML({ node }) { return ["span", { class: "font-medium text-primary" }, `@${node.attrs.label}`]; },
  renderText({ node }) { return `@${node.attrs.label}`; },
});
export function MentionPicker({ editor, busy }: { editor: Editor | null; busy: boolean }) {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const q = useDebounced(search.trim(), 200);
  const users = useQuery({ queryKey: ["mention-users", q], enabled: q.length >= 2,
    queryFn: () => api<{ items: User[] }>(`/api/v1/users?status=active&limit=8&q=${encodeURIComponent(q)}`) });
  const groups = useQuery({ queryKey: ["groups"], enabled: q.length >= 2,
    queryFn: async () => (await api<{ items: Group[] }>("/api/v1/groups")).items });
  function insert(kind: string, label: string, id?: string) {
    editor?.chain().focus().insertContent([{ type: "mention", attrs: { kind, label, id } }, { type: "text", text: " " }]).run();
    setSearch("");
  }
  return <>
      <Input value={search} aria-label={t("items.mentionSearch")} onChange={(event) => setSearch(event.target.value)} />
      <div className="flex flex-wrap gap-1">
        <Button variant="outline" size="sm" disabled={busy} onClick={() => insert("followers", t("items.followersLabel"))}>{t("items.followers")}</Button>
        {q.length >= 2 && q === search.trim() && users.data?.items.map((user) => {
          const label = `${user.first_name} ${user.last_name}`.trim();
          return <Button key={user.id} variant="outline" size="sm" disabled={busy} onClick={() => insert("user", label, user.id)}>{label}</Button>;
        })}
        {q.length >= 2 && groups.data?.filter((g) => g.name.toLowerCase().includes(q.toLowerCase())).map((g) =>
          <Button key={g.id} variant="outline" size="sm" disabled={busy} onClick={() => insert("group", g.name, g.id)}>{g.name}</Button>)}
      </div>
  </>;
}
