import { useQuery } from "@tanstack/react-query";
import { ExternalLink, Settings2 } from "lucide-react";
import { useId } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import type { Item } from "@/lib/items";
import { Button } from "@/components/ui/button";
import { PydioProjectFolder } from "@/components/pydio-project-folder";

type PydioReference = { state: "linked" | "not_configured" | "roots_unavailable" | "invalid"; open_url?: string };

/**
 * Project Detail document section (PROMPT 033). The server decides the state and is the only source of the address: the browser
 * never builds a link from the stored reference, and no folder name is shown. Only "linked" offers Open in Pydio.
 */
export function ProjectDocuments({ item, onConfigure }: { item: Item; onConfigure: () => void }) {
  const { t } = useTranslation();
  const heading = useId();
  const reference = useQuery({
    queryKey: ["pydio-reference", item.id, item.version],
    queryFn: () => api<PydioReference>(`/api/v1/items/${item.id}/pydio-reference`),
  });
  const canEdit = item.my_capabilities.includes("edit");
  const data = reference.data;
  return <section aria-labelledby={heading} className="space-y-3 rounded-lg border bg-card p-4">
    <h2 id={heading} className="font-medium">{t("views.documents.title")}</h2>
    {reference.isPending && <p role="status" className="text-sm text-muted-foreground">{t("views.documents.loading")}</p>}
    {reference.isError && <p role="alert" className="text-sm text-destructive">{t("views.documents.loadError")}</p>}
    {data && <div className="flex flex-wrap items-center gap-3 text-sm">
      {data.state === "linked" && data.open_url && <>
        <span className="text-muted-foreground">{t("views.documents.linked")}</span>
        <a href={data.open_url} target="_blank" rel="noopener noreferrer"
          className="inline-flex items-center gap-1 rounded-md border px-3 py-1.5 font-medium hover:bg-muted">
          <ExternalLink aria-hidden="true" className="size-4" />{t("views.documents.open")}
        </a>
      </>}
      {data.state === "not_configured" && <span className="text-muted-foreground">{t("views.documents.notConfigured")}</span>}
      {data.state === "roots_unavailable" && <span className="text-muted-foreground">{t("views.documents.rootsUnavailable")}</span>}
      {data.state === "invalid" && <span role="status" className="text-destructive">{t("views.documents.invalid")}</span>}
    </div>}
    {canEdit && <Button variant="outline" size="sm" onClick={onConfigure}><Settings2 />{t("views.documents.configure")}</Button>}
    <PydioProjectFolder itemId={item.id} version={item.version} />
  </section>;
}
