import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { useProfile } from "@/lib/profile";
import { useSpaces } from "@/lib/space-queries";
import { can } from "@/lib/spaces";
import { SpaceAutomationPanel } from "@/components/space-automation";
import { WorkspaceRulesPanel } from "@/components/workspace-rules-panel";
import { SegmentedControl } from "@/components/ui/segmented-control";

export const Route = createFileRoute("/_app/settings/automation")({ component: AutomationSettings });

type Tab = "workspace" | "space";
const selectClass = "h-9 rounded-lg border border-input bg-background px-2 text-sm";

/**
 * Settings > Automation: one page for both scopes. Workspace rules need a workspace administrator; a Space's rules need
 * the Space's edit role, and the Space selector lists only the Spaces the person may edit. The server checks every call.
 */
function AutomationSettings() {
  const { t } = useTranslation();
  const isAdmin = useProfile()?.user_type === "admin";
  const spaces = useSpaces();
  const manageable = (spaces.data ?? []).filter((space) => can(space, "edit"));
  const [tab, setTab] = useState<Tab>();
  const current: Tab = tab ?? (isAdmin ? "workspace" : "space");
  const [spaceId, setSpaceId] = useState("");
  const chosen = manageable.find((space) => space.id === spaceId) ?? manageable[0];

  if (spaces.isSuccess && !isAdmin && manageable.length === 0) return <p role="alert">{t("settings.noManageableSpace")}</p>;

  return <section className="space-y-5">
    <p className="text-sm text-muted-foreground">{t("automation.settingsDescription")}</p>
    <SegmentedControl aria-label={t("automation.tabsLabel")} value={current}
      items={(["workspace", "space"] as const).map((value) => ({ value, label: t(`automation.tab.${value}`) }))}
      onChange={(value) => setTab(value as Tab)} />

    {current === "workspace" && (isAdmin
      ? <WorkspaceRulesPanel />
      : <p role="status" className="rounded-md border p-4 text-sm text-muted-foreground">{t("automation.workspaceAdminOnly")}</p>)}

    {current === "space" && <div className="space-y-5">
      {manageable.length === 0 && spaces.isSuccess && <p role="status" className="rounded-md border p-4 text-sm text-muted-foreground">{t("automation.noSpaces")}</p>}
      {chosen && <>
        <div className="flex flex-wrap items-center gap-3">
          <label className="flex items-center gap-2 text-sm">
            <span>{t("automation.spaceLabel")}</span>
            <select className={selectClass} value={chosen.id} onChange={(event) => setSpaceId(event.target.value)}>
              {manageable.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
            </select>
          </label>
          <Link to="/spaces/$spaceId" params={{ spaceId: chosen.id }} className="text-sm text-muted-foreground underline">{t("automation.openSpace")}</Link>
        </div>
        <SpaceAutomationPanel key={chosen.id} spaceId={chosen.id} />
      </>}
    </div>}
  </section>;
}
