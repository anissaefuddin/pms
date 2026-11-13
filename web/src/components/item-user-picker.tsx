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
  return <fieldset className="space-y-2">
    <legend className="font-medium">{label}</legend>
    <div className="flex flex-wrap gap-1">
      {selected.map((person) => {
        const name = person.status && person.status !== "active" ? t("items.customFields.inactivePerson", { name: person.name }) : person.name;
        return <Button key={person.id} type="button" size="sm" variant="outline" disabled={disabled}
          aria-label={t("items.remove", { name })}
          onClick={() => onChange(selected.filter((entry) => entry.id !== person.id))}>{name}</Button>;
      })}
    </div>
    <Input type="search" autoFocus={autoFocus} aria-label={`${label}: ${t("items.searchUsers")}`} value={value} disabled={!canSearch}
      onChange={(event) => setValue(event.target.value)} />
    {users.isError && <p role="status" className="text-sm text-muted-foreground">{t("items.usersUnavailable")}</p>}
    {canSearch && q.length >= 2 && q === value.trim() && users.data?.items.filter((user) => user.status === "active" && !selected.some((entry) => entry.id === user.id)).map((user) => {
      const person = { id: user.id, name: `${user.first_name} ${user.last_name}`.trim() };
      return <Button key={user.id} type="button" variant="ghost" size="sm" onClick={() => {
        onChange(multiple ? [...selected, person] : [person]);
        setValue("");
      }}>{person.name}</Button>;
    })}
  </fieldset>;
}
