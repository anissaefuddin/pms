import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

const tab = "rounded-md px-3 py-1.5 text-sm font-medium text-muted-foreground hover:text-foreground aria-[current=page]:bg-muted aria-[current=page]:text-foreground";

export function RequestTabs() {
  const { t } = useTranslation();
  return <nav aria-label={t("forms.mine.tabs")} className="flex gap-1">
    <Link to="/requests" activeOptions={{ exact: true }} className={tab}>{t("forms.mine.tabForms")}</Link>
    <Link to="/requests/mine" className={tab}>{t("forms.mine.tabMine")}</Link>
  </nav>;
}
