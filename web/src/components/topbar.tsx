import { formatNumber, useProfile } from "@/lib/profile";
import type { ReactNode } from "react";
import { Bell, CircleUser, Plus } from "lucide-react";
import { Link, useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { listNotifications, notificationKey } from "@/lib/notifications";
import { api } from "@/lib/api";
import { useState } from "react";
import { FeedbackDialog } from "@/components/feedback-dialog";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";

export function Topbar({ onCreate, live, search, navigation }: { onCreate?: () => void; live: boolean; search?: ReactNode; navigation?: ReactNode }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState(false);
  const unread = useQuery({ queryKey: [...notificationKey, "badge"], queryFn: () => listNotifications(1) }).data?.unread_count;
  async function logout() {
    await api("/api/v1/auth/logout", { method: "POST" }).catch(() => undefined);
    queryClient.clear();
    location.assign("/login");
  }
  return (
    <header className="flex h-12 shrink-0 items-center justify-end border-b px-2 sm:px-4">
      {navigation}
      <span role="status" className="mr-auto text-xs text-muted-foreground">{t(live ? "nav.realtime.live" : "nav.realtime.reconnecting")}</span>
      {search}
      <Link to="/inbox" className="relative rounded p-2 hover:bg-accent" aria-label={unread ? t("nav.inbox.badge", { count: unread }) : t("nav.inbox.label")}>
        <Bell className="size-5" />
        {!!unread && <span className="absolute -top-1 right-0 rounded-full bg-primary px-1 text-[10px] text-primary-foreground">{formatNumber(unread, profile)}</span>}
      </Link>
      {onCreate && <Button variant="ghost" size="icon" aria-label={t("items.new")} onClick={onCreate}><Plus /></Button>}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon" aria-label={t("nav.profile.label")} />}><CircleUser /></DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-44">
          <DropdownMenuItem onClick={() => navigate({ to: "/settings/profile" })}>{t("settings.profile.title")}</DropdownMenuItem>
          <DropdownMenuItem onClick={() => navigate({ to: "/settings/security" })}>{t("nav.profile.security")}</DropdownMenuItem>
          <DropdownMenuItem onClick={() => navigate({ to: "/settings/profile" })}>{t("settings.profile.appearance")}</DropdownMenuItem>
          <DropdownMenuItem onClick={() => setFeedback(true)}>{t("nav.feedback.menu")}</DropdownMenuItem>
          <DropdownMenuItem onClick={logout}>{t("nav.profile.logout")}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <FeedbackDialog open={feedback} onOpenChange={setFeedback} />
    </header>
  );
}
