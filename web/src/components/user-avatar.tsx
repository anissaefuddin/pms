import type { CurrentUser } from "@/lib/api";
import { cn } from "@/lib/utils";

/** The person's profile photo, or their initials until a photo is uploaded. The version in the URL refreshes the image after each change. */
export function UserAvatar({ profile, className }: { profile?: CurrentUser; className: string }) {
  if (profile?.avatar_version) {
    return <img src={`/api/v1/me/avatar?v=${profile.avatar_version}`} alt="" aria-hidden="true" className={cn("shrink-0 object-cover", className)} />;
  }
  const initials = `${profile?.first_name?.[0] ?? ""}${profile?.last_name?.[0] ?? ""}`.toUpperCase();
  return <span aria-hidden="true" className={cn("grid shrink-0 place-items-center bg-primary font-semibold text-primary-foreground", className)}>{initials}</span>;
}
