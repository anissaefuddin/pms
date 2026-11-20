import { Link, createFileRoute, redirect } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ApiError, type CurrentUser } from "@/lib/api";
import { useSpace } from "@/lib/space-queries";
import { isSpaceSettingsTab, SpaceSettings } from "@/components/space-settings";
import { LoadingState } from "@/components/ui/loading-state";
import { ErrorState } from "@/components/ui/error-state";
import { buttonVariants } from "@/components/ui/button";

export const Route = createFileRoute("/_app/spaces_/$spaceId/settings/$tab")({
  // An unknown tab goes back to General rather than showing an empty page.
  beforeLoad: ({ params }) => {
    if (!isSpaceSettingsTab(params.tab)) throw redirect({ to: "/spaces/$spaceId/settings/$tab", params: { spaceId: params.spaceId, tab: "general" }, replace: true });
  },
  component: SpaceSettingsRoute,
});

function SpaceSettingsRoute() {
  const { t } = useTranslation();
  const { spaceId, tab } = Route.useParams();
  const space = useSpace(spaceId);
  const queryClient = useQueryClient();
  const me = queryClient.getQueryData<CurrentUser>(["me"]);
  if (space.isPending) return <main className="p-8"><LoadingState label={t("spaces.page.loading")} /></main>;
  if (space.error instanceof ApiError && space.error.status === 404) {
    return <main className="p-8"><div className="max-w-md space-y-3">
      <h1 className="workspace-title">{t("spaces.notFound.title")}</h1>
      <p className="text-muted-foreground">{t("spaces.notFound.description")}</p>
      {me && <Link to="/spaces/$spaceId" params={{ spaceId: me.personal_space_id }} className={buttonVariants({ variant: "outline" })}>{t("spaces.notFound.home")}</Link>}
    </div></main>;
  }
  if (space.isError) return <main className="p-8"><ErrorState message={t("spaces.page.error")} retryLabel={t("spaces.page.retry")} onRetry={() => void space.refetch()} /></main>;
  // Without edit rights the General tab is read-only; the server still decides every change.
  if (!isSpaceSettingsTab(tab)) return null;
  return <SpaceSettings space={space.data} me={me} tab={tab} />;
}
