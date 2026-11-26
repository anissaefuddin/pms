import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ErrorState } from "@/components/ui/error-state";
import { LoadingState } from "@/components/ui/loading-state";
import { useProfile } from "@/lib/profile";
import { CUSTOMER_PICKER_LIMIT, isTruncated, selectionStatus, type CustomerLookupScope } from "@/lib/customer-picker";
import { customerErrorText, customerKeys, listEligibleCustomers, type CustomerOption } from "@/lib/customers";

/**
 * Single Customer selection for ownership forms (B2-08). It shows only what the authorized lookup returned.
 * Eligibility here is a convenience, not authority: the server re-checks any ownership change (B3).
 * Not yet placed in Space or Project forms.
 */
export function CustomerPicker({ label, value, selected, onChange, scope = "membership", disabled = false, clearable = false, placeholder }: {
  label: string;
  /** Canonical Customer UUID of the current selection, or null. */
  value: string | null;
  /** Display metadata the parent already holds for `value`. Shown only as unconfirmed, never as valid. */
  selected?: CustomerOption | null;
  onChange: (customer: CustomerOption | null) => void;
  scope?: CustomerLookupScope;
  disabled?: boolean;
  clearable?: boolean;
  placeholder?: string;
}) {
  const { t } = useTranslation();
  const me = useProfile();
  const [search, setSearch] = useState("");
  const needle = search.trim();
  // The administrative scope is a configuration lookup; a non-admin never sends the request.
  const allowed = !!me && (scope === "membership" || me.user_type === "admin");
  const lookup = useQuery({
    // The user id is part of the key: options fetched for one person are never shown to another.
    queryKey: customerKeys.eligible(me?.id ?? "", scope, needle),
    queryFn: () => listEligibleCustomers(scope, needle),
    enabled: allowed && !disabled,
  });
  const options = lookup.data?.items ?? [];
  const truncated = isTruncated(options.length, CUSTOMER_PICKER_LIMIT);
  const status = selectionStatus(value, options, { loaded: lookup.isSuccess, searching: needle !== "", truncated });
  const chosen = status === "selectable" ? options.find((option) => option.id === value) : undefined;

  return <fieldset className="space-y-2">
    <legend className="text-sm font-medium">{label}</legend>

    {status === "selectable" && chosen && <p className="text-sm">{t("settings.customers.picker.selected", { name: chosen.name, code: chosen.code })}</p>}
    {status === "unknown" && <p className="text-sm text-muted-foreground">
      {selected?.id === value ? t("settings.customers.picker.unconfirmed", { name: selected.name, code: selected.code }) : t("settings.customers.picker.unconfirmedNoName")}
    </p>}
    {status === "unavailable" && <p role="alert" className="text-sm text-destructive">{t("settings.customers.picker.unavailable")}</p>}
    {clearable && value && !disabled && <Button type="button" variant="outline" size="sm" onClick={() => onChange(null)}>{t("settings.customers.picker.clear")}</Button>}

    <Input type="search" aria-label={t("settings.customers.picker.search")} placeholder={placeholder ?? t("settings.customers.picker.search")}
      value={search} onChange={(event) => setSearch(event.target.value)} disabled={disabled} />

    {!allowed ? <p className="text-sm text-muted-foreground">{t("settings.customers.picker.adminOnly")}</p>
      : disabled ? null
      : lookup.isPending ? <LoadingState label={t("settings.customers.picker.loading")} />
      : lookup.isError ? <ErrorState message={customerErrorText(t, lookup.error)} retryLabel={t("settings.customers.retry")} onRetry={() => void lookup.refetch()} />
      : options.length === 0 ? <p className="text-sm text-muted-foreground">{needle ? t("settings.customers.noMatch") : t("settings.customers.picker.empty")}</p>
      : <ul aria-label={label} className="max-h-64 divide-y overflow-y-auto rounded-md border">
        {options.map((option) => {
          const on = option.id === value;
          return <li key={option.id}>
            <button type="button" aria-pressed={on} onClick={() => onChange(option)}
              className="flex w-full items-center gap-3 px-3 py-2 text-start hover:bg-accent">
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium">{option.name}</span>
                <span className="block truncate font-mono text-xs text-muted-foreground">{option.code}</span>
              </span>
              {on && <Check aria-hidden="true" className="size-4 shrink-0 text-primary" />}
            </button>
          </li>;
        })}
      </ul>}
    {truncated && !disabled && <p className="text-xs text-muted-foreground">{t("settings.customers.picker.truncated", { count: CUSTOMER_PICKER_LIMIT })}</p>}
  </fieldset>;
}
