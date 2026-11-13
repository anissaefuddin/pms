import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Star } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";

export type StarredItem = { id: string; title: string; type: "project" | "folder" | "task" };

export const starKeys = { all: ["stars"] as const };
export const useStars = () => useQuery({
  queryKey: starKeys.all, staleTime: 30_000, retry: false,
  queryFn: async () => (await api<{ items: StarredItem[] }>("/api/v1/stars")).items,
});

/** Personal bookmark of one Item; the state comes from the caller's star list. */
export function StarButton({ itemId }: { itemId: string }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const stars = useStars();
  const starred = !!stars.data?.some((item) => item.id === itemId);
  async function toggle() {
    try {
      await api(`/api/v1/items/${itemId}/star`, { method: starred ? "DELETE" : "PUT" });
      await qc.invalidateQueries({ queryKey: starKeys.all });
      await qc.invalidateQueries({ queryKey: ["widget-data"] });
    } catch { toast.error(t("starred.failed")); }
  }
  return <Button variant="ghost" size="icon" aria-pressed={starred} disabled={stars.isPending}
    aria-label={t(starred ? "starred.remove" : "starred.add")} onClick={() => void toggle()}>
    <Star className={starred ? "fill-current" : undefined} /></Button>;
}
