import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { useDebounced } from "@/lib/space-queries";
import type { User } from "@/lib/spaces";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export type PersonOption = { id: string; name: string; status?: User["status"] };

export function ItemUserPicker({ label, selected, onChange, multiple = false, disabled = false, maxSelected, autoFocus }: {
  label: string;
  selected: PersonOption[];
  onChange: (selected: PersonOption[]) => void;
  multiple?: boolean;
  disabled?: boolean;
  maxSelected?: number;
  autoFocus?: boolean;
}) {
  const { t } = useTranslation();
  const [value, setValue] = useState("");
  const q = useDebounced(value.trim(), 200);
  const canSearch = !disabled && (maxSelected === undefined || selected.length < maxSelected);
  const users = useQuery({
    queryKey: ["item-active-users", q],
    queryFn: () => api<{ items: User[] }>(`/api/v1/users?status=active&limit=8&q=${encodeURIComponent(q)}`),
    enabled: canSearch && q.length >= 2,
    retry: false,
  });
  const results = canSearch && q.length >= 2 && q === value.trim()
    ? users.data?.items.filter((user) => user.status === "active" && !selected.some((entry) => entry.id === user.id)) ?? []
    : [];
  const chooseResult = (user: User) => {
    const person = { id: user.id, name: `${user.first_name} ${user.last_name}`.trim() };
    onChange(multiple ? [...selected, person] : [person]);
    setValue("");
  };
  const chip = (person: PersonOption) => {
    const name = person.status && person.status !== "active" ? t("items.customFields.inactivePerson", { name: person.name }) : person.name;
    return <Button key={person.id} type="button" size="sm" variant="outline" disabled={disabled}
      aria-label={t("items.remove", { name })}
      onClick={() => onChange(selected.filter((entry) => entry.id !== person.id))}>{name}</Button>;
  };

  if (!multiple) {
    // One owner: the chosen person and the search share one field, so there is one place to read and change the value.
    return <fieldset className="space-y-2">
      <legend className="font-medium">{label}</legend>
      <div className={`flex min-h-9 flex-wrap items-center gap-1 rounded-md border border-input bg-background px-2 py-1 focus-within:ring-3 focus-within:ring-ring/50 ${disabled ? "opacity-60" : ""}`}>
        {selected.map((person) => {
          const name = person.status && person.status !== "active" ? t("items.customFields.inactivePerson", { name: person.name }) : person.name;
          return <span key={person.id} className="inline-flex items-center gap-1 rounded bg-muted px-2 py-0.5 text-sm">
            {name}
            <Button type="button" size="icon-sm" variant="ghost" disabled={disabled} aria-label={t("items.remove", { name })}
              onClick={() => onChange([])}>×</Button>
          </span>;
        })}
        <input type="search" autoFocus={autoFocus} aria-label={`${label}: ${t("items.searchUsers")}`}
          placeholder={selected.length ? "" : t("items.searchUsers")} value={value} disabled={!canSearch}
          onChange={(event) => setValue(event.target.value)}
          className="min-w-32 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed" />
      </div>
      {users.isError && <p role="status" className="text-sm text-muted-foreground">{t("items.usersUnavailable")}</p>}
      {results.length > 0 && <ul className="space-y-0.5 rounded-md border p-1">
        {results.map((user) => <li key={user.id}>
          <Button type="button" variant="ghost" size="sm" className="w-full justify-start" onClick={() => chooseResult(user)}>
            {`${user.first_name} ${user.last_name}`.trim()}
          </Button>
        </li>)}
      </ul>}
    </fieldset>;
  }

  return <fieldset className="space-y-2">
    <legend className="font-medium">{label}</legend>
    <div className="flex flex-wrap gap-1">{selected.map(chip)}</div>
    <Input type="search" autoFocus={autoFocus} aria-label={`${label}: ${t("items.searchUsers")}`} placeholder={t("items.searchUsers")} value={value} disabled={!canSearch}
      onChange={(event) => setValue(event.target.value)} />
    {users.isError && <p role="status" className="text-sm text-muted-foreground">{t("items.usersUnavailable")}</p>}
    {results.map((user) => <Button key={user.id} type="button" variant="ghost" size="sm" onClick={() => chooseResult(user)}>
      {`${user.first_name} ${user.last_name}`.trim()}
    </Button>)}
  </fieldset>;
}
