import { CircleUser, Plus } from "lucide-react";
import { useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";

/** Empty topbar for M0: only the profile menu. Breadcrumb, search and the rest arrive in M1. */
export function Topbar({ onCreate }: { onCreate?: () => void }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  async function logout() {
    await api("/api/v1/auth/logout", { method: "POST" }).catch(() => undefined);
    queryClient.clear();
    location.assign("/login");
  }
  return (
    <header className="flex h-12 shrink-0 items-center justify-end border-b px-4">
      {onCreate && <Button variant="ghost" size="icon" aria-label={t("items.new")} onClick={onCreate}><Plus /></Button>}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon" aria-label={t("nav.profile.label")} />}><CircleUser /></DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-44">
          <DropdownMenuItem onClick={() => navigate({ to: "/settings/security" })}>{t("nav.profile.security")}</DropdownMenuItem>
          <DropdownMenuItem onClick={logout}>{t("nav.profile.logout")}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </header>
  );
}
