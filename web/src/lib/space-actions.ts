import type { Space } from "@/lib/spaces";

export type SpaceAction = "open" | "copy-link" | "rename" | "members" | "settings" | "archive";

/**
 * Actions for the Space row menu, from the effective capabilities of this Space (D-17.1, D-SA1).
 * A Full share is not a Space Admin: archiving needs the explicit role (is_space_admin); rename and icon need
 * edit; member changes need share. A Personal Space and an archived Space never offer rename, members or archive.
 */
export function spaceActions(space: Space): SpaceAction[] {
  const caps = space.my_capabilities ?? [];
  const personal = space.kind === "personal";
  const actions: SpaceAction[] = ["open", "copy-link"];
  if (!personal && !space.archived && caps.includes("edit")) actions.push("rename");
  if (!personal && !space.archived && caps.includes("share")) actions.push("members");
  actions.push("settings");
  if (!personal && !space.archived && space.is_space_admin) actions.push("archive");
  return actions;
}
