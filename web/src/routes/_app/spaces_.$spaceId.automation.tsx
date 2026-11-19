import { Link, createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { useSpace } from "@/lib/space-queries";
import { SpaceAutomationPanel } from "@/components/space-automation";

// Deep link kept for existing bookmarks. The same Space panel also renders in Settings > Automation > Space.
export const Route = createFileRoute("/_app/spaces_/$spaceId/automation")({
  validateSearch: (search: Record<string, unknown>): { container?: string } => ({ container: typeof search.container === "string" && /^[0-9a-f-]{36}$/i.test(search.container) ? search.container : undefined }),
  component: AutomationPage,
});

function AutomationPage() {
  const { t } = useTranslation();
  const { spaceId } = Route.useParams();
  const { container } = Route.useSearch();
  const space = useSpace(spaceId);
  return <main className="space-y-5 p-8">
    <header className="border-b pb-5">
      <Link to="/spaces/$spaceId" params={{ spaceId }} className="text-sm text-muted-foreground underline">{t("automation.back", { name: space.data?.name ?? "" })}</Link>
      <h1 className="mt-1 workspace-title">{t("automation.title")}</h1>
      <p className="text-sm text-muted-foreground">{t("automation.description")}</p>
    </header>
    <SpaceAutomationPanel spaceId={spaceId} container={container} />
  </main>;
}
