import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { Button } from "@/components/ui/button";
import { api, ApiError, type CurrentUser } from "@/lib/api";
import { formatDateTime } from "@/lib/profile";

type Listing = components["schemas"]["PydioListing"];

function size(bytes: number, locale: string) {
  const [value, unit] = bytes < 1024 ? [bytes, "byte"] : bytes < 1024 ** 2 ? [bytes / 1024, "kilobyte"] : [bytes / 1024 ** 2, "megabyte"];
  return new Intl.NumberFormat(locale, { style: "unit", unit, maximumFractionDigits: 1 }).format(value);
}

// PydioFolderFiles lists, on demand, the Pydio folder a text field links to (MVP-13). Read-only: each entry opens in
// Pydio, which applies its own access rules; PMS shows names and metadata only.
export function PydioFolderFiles({ itemId, fieldId, profile }: { itemId: string; fieldId: string; profile: CurrentUser }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const listing = useQuery({
    queryKey: ["pydio-files", itemId, fieldId],
    queryFn: () => api<Listing>(`/api/v1/items/${itemId}/pydio-files?field=${encodeURIComponent(fieldId)}`),
    enabled: open, retry: false, staleTime: 30_000,
  });
  const failure = listing.error instanceof ApiError && ["pydio_not_configured", "pydio_link_not_allowed", "pydio_folder_not_found"].includes(listing.error.code ?? "")
    ? t(`items.customFields.pydio.${listing.error.code}`) : t("items.customFields.pydio.error");
  return <div className="mt-1 text-sm">
    <Button type="button" variant="link" size="sm" className="h-auto px-0" aria-expanded={open} onClick={() => setOpen(!open)}>
      {t(open ? "items.customFields.pydio.hide" : "items.customFields.pydio.show")}
    </Button>
    {open && (listing.isPending ? <p className="text-muted-foreground">{t("items.customFields.pydio.loading")}</p>
      : listing.isError ? <p className="text-destructive" role="alert">{failure}</p>
      : listing.data.items.length === 0 ? <p className="text-muted-foreground">{t("items.customFields.pydio.empty")}</p>
      : <ul className="mt-1 space-y-1" aria-label={t("items.customFields.pydio.listLabel")}>
        {listing.data.items.map((entry) => <li key={entry.url} className="flex flex-wrap items-baseline gap-x-2">
          <a href={entry.url} target="_blank" rel="noopener noreferrer" className="break-all text-primary underline-offset-2 hover:underline">
            {entry.folder ? `${entry.name}/` : entry.name}
          </a>
          {!entry.folder && <span className="text-xs text-muted-foreground">
            {entry.size != null && size(entry.size, profile.locale)}{entry.modified_at && ` · ${formatDateTime(entry.modified_at, profile)}`}
          </span>}
        </li>)}
        {listing.data.truncated && <li className="text-muted-foreground">
          <a href={listing.data.folder_url} target="_blank" rel="noopener noreferrer" className="underline-offset-2 hover:underline">{t("items.customFields.pydio.truncated")}</a>
        </li>}
      </ul>)}
  </div>;
}
