import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { useBlueprints } from "@/lib/blueprints";
import { useSpaces } from "@/lib/space-queries";

export const Route = createFileRoute("/_app/blueprints")({ component: Blueprints });

function Blueprints() {
  const { t } = useTranslation();
  const spaces = (useSpaces().data ?? []).filter((space) => space.kind === "shared");
  const [chosen, setChosen] = useState("");
  const spaceId = chosen || spaces[0]?.id || "";
  const list = useBlueprints(spaceId);
  return <main className="space-y-5 p-8">
    <header className="border-b pb-5">
      <h1 className="workspace-title">{t("blueprints.page.title")}</h1>
      <p className="text-sm text-muted-foreground">{t("blueprints.page.description")}</p>
    </header>
    <label className="block text-sm">{t("blueprints.page.space")}
      <select className="mt-1 block h-9 rounded-md border border-input bg-background px-2" value={spaceId} onChange={(e) => setChosen(e.target.value)}>
        {spaces.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
      </select></label>
    {list.isError && <p role="alert">{t("blueprints.page.error")}</p>}
    {list.data?.length === 0 && <p role="status" className="text-sm text-muted-foreground">{t("blueprints.page.empty")}</p>}
    {!!list.data?.length && <ul className="divide-y rounded-xl border bg-card">{list.data.map((blueprint) => <li key={blueprint.id} className="flex justify-between p-3 text-sm">
      <span>{blueprint.name}</span><span className="text-muted-foreground">{blueprint.item_type}</span></li>)}</ul>}
  </main>;
}
