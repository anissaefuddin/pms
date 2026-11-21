import { createFileRoute, redirect } from "@tanstack/react-router";
import { SpaceSettingsPage, isSpaceSettingsTab } from "@/components/space-settings";

export const Route = createFileRoute("/_app/spaces_/$spaceId/settings/$tab")({
  // An unknown tab goes back to General rather than showing an empty page.
  beforeLoad: ({ params }) => {
    if (!isSpaceSettingsTab(params.tab)) throw redirect({ to: "/spaces/$spaceId/settings/$tab", params: { spaceId: params.spaceId, tab: "general" }, replace: true });
  },
  component: SpaceSettingsRoute,
});

function SpaceSettingsRoute() {
  const { spaceId, tab } = Route.useParams();
  if (!isSpaceSettingsTab(tab)) return null;
  return <SpaceSettingsPage spaceId={spaceId} tab={tab} context="space" />;
}
