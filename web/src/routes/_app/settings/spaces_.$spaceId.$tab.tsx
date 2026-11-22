import { createFileRoute, redirect } from "@tanstack/react-router";
import { SpaceSettingsPage, isSpaceSettingsTab } from "@/components/space-settings";

// Opened from the Workspace Spaces registry: stays in the Settings shell, same content as the Space work area route.
export const Route = createFileRoute("/_app/settings/spaces_/$spaceId/$tab")({
  beforeLoad: ({ params }) => {
    if (!isSpaceSettingsTab(params.tab)) throw redirect({ to: "/settings/spaces/$spaceId/$tab", params: { spaceId: params.spaceId, tab: "general" }, replace: true });
  },
  component: WorkspaceSpaceSettingsRoute,
});

function WorkspaceSpaceSettingsRoute() {
  const { spaceId, tab } = Route.useParams();
  if (!isSpaceSettingsTab(tab)) return null;
  return <SpaceSettingsPage spaceId={spaceId} tab={tab} context="workspace" />;
}
