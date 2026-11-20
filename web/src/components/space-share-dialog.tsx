import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, type CurrentUser } from "@/lib/api";
import { spaceKeys, type Share, type Space } from "@/lib/spaces";
import { ShareDialog } from "@/components/share-dialog";

/** Member and access editor for one shared Space. Shared by the Space page and Space settings. */
export function SpaceShareDialog({ open, onOpenChange, space, me, canShare }: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  space: Space;
  me: CurrentUser;
  canShare: boolean;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  return (
    <ShareDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("share.title", { name: space.name })}
      queryKey={spaceKeys.one(space.id)}
      loadShares={async () => {
        let etagValue = "";
        const { items } = await api<{ items: Share[] }>(`/api/v1/spaces/${space.id}/shares`, undefined, (headers) => {
          etagValue = headers.get("ETag") ?? "";
        });
        if (!etagValue) throw new Error("Missing share ETag");
        return { items, etag: etagValue };
      }}
      saveShares={(shares, shareEtag) => api(`/api/v1/spaces/${space.id}/shares`, { method: "PUT", headers: { "If-Match": shareEtag }, body: JSON.stringify({ shares }) })}
      canShare={canShare}
      me={me}
      onSaved={() => queryClient.invalidateQueries({ queryKey: spaceKeys.all })}
    />
  );
}
