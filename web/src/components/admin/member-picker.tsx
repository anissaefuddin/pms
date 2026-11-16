import { useState } from "react";
import { Check } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { InitialsAvatar } from "@/components/admin/admin-ui";

export type PickableUser = { id: string; name: string; email: string };

/**
 * Searchable member selection. Each row shows the name with the email as a secondary identifier;
 * the user id is the identity used for the selection, so people with the same name stay distinct.
 */
export function MemberPicker({ users, selected, onChange }: { users: PickableUser[]; selected: string[]; onChange: (ids: string[]) => void }) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const shown = users.filter((user) => `${user.name} ${user.email}`.toLowerCase().includes(query.toLowerCase()));
  const toggle = (id: string) => onChange(selected.includes(id) ? selected.filter((entry) => entry !== id) : [...selected, id]);
  return <fieldset className="space-y-2">
    <legend className="text-sm font-medium">{t("settings.users.members")}</legend>
    <p className="text-xs text-muted-foreground">{t("settings.users.selectedCount", { count: selected.length })}</p>
    <Input type="search" aria-label={t("settings.users.memberSearch")} placeholder={t("settings.users.memberSearch")} value={query} onChange={(event) => setQuery(event.target.value)} />
    <ul role="group" aria-label={t("settings.users.members")} className="max-h-64 divide-y overflow-y-auto rounded-md border">
      {shown.map((user) => {
        const on = selected.includes(user.id);
        return <li key={user.id}>
          <button type="button" role="checkbox" aria-checked={on} onClick={() => toggle(user.id)}
            className="flex w-full items-center gap-3 px-3 py-2 text-start hover:bg-accent">
            <InitialsAvatar name={user.name} size="size-7" />
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">{user.name}</span>
              <span className="block truncate text-xs text-muted-foreground">{user.email}</span>
            </span>
            <span aria-hidden="true" className={`grid size-5 place-items-center rounded border ${on ? "border-primary bg-primary text-primary-foreground" : "border-input"}`}>
              {on && <Check className="size-3.5" />}
            </span>
          </button>
        </li>;
      })}
      {!shown.length && <li className="px-3 py-4 text-center text-sm text-muted-foreground">{t("settings.users.memberNoResults")}</li>}
    </ul>
  </fieldset>;
}
