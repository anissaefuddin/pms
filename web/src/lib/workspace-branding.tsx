import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

export type WorkspaceBranding = {
  name: string;
  brand_color: string;
  icon_image_version: number;
  logo_light_version: number;
  logo_dark_version: number;
};

export const workspaceBrandingKey = ["workspace-branding"] as const;
export const workspaceAssetURL = (kind: "icon" | "logo_light" | "logo_dark", version: number) => `/api/v1/account/branding/${kind}?v=${version}`;
const fallback: WorkspaceBranding = { name: "LENS PMS", brand_color: "#2563EB", icon_image_version: 0, logo_light_version: 0, logo_dark_version: 0 };

export function useWorkspaceBranding() {
  return useQuery({ queryKey: workspaceBrandingKey, queryFn: () => api<WorkspaceBranding>("/api/v1/branding"), staleTime: 300_000 }).data ?? fallback;
}

export function WorkspaceBrandMark({ compact = false, className = "" }: { compact?: boolean; className?: string }) {
  const brand = useWorkspaceBranding();
  const initial = brand.name.trim().charAt(0).toUpperCase() || "L";
  if (compact) return brand.icon_image_version > 0
    ? <img src={workspaceAssetURL("icon", brand.icon_image_version)} alt="" className={`size-8 shrink-0 rounded-md object-contain ${className}`} />
    : <span aria-hidden="true" style={{ backgroundColor: brand.brand_color }} className={`grid size-8 shrink-0 place-items-center rounded-md text-sm font-semibold text-white ${className}`}>{initial}</span>;
  if (brand.logo_light_version > 0 || brand.logo_dark_version > 0) return <div className={className}>
    {brand.logo_light_version > 0 && <img src={workspaceAssetURL("logo_light", brand.logo_light_version)} alt={brand.name} className="h-9 max-w-52 object-contain dark:hidden" />}
    {brand.logo_dark_version > 0 && <img src={workspaceAssetURL("logo_dark", brand.logo_dark_version)} alt={brand.name} className="hidden h-9 max-w-52 object-contain dark:block" />}
  </div>;
  return <div className={`flex items-center gap-2 ${className}`}><WorkspaceBrandMark compact /><span className="font-semibold">{brand.name}</span></div>;
}

export function WorkspaceBrandingEffects() {
  const brand = useWorkspaceBranding();
  useEffect(() => {
    const root = document.documentElement;
    root.style.setProperty("--brand-primary", brand.brand_color);
    root.style.setProperty("--ring", brand.brand_color);
    root.style.setProperty("--sidebar-primary", brand.brand_color);
    document.title = brand.name;
    let icon = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
    if (!icon) { icon = document.createElement("link"); icon.rel = "icon"; document.head.append(icon); }
    const initial = brand.name.trim().charAt(0).toUpperCase() || "L";
    icon.href = brand.icon_image_version > 0 ? workspaceAssetURL("icon", brand.icon_image_version)
      : `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="12" fill="${brand.brand_color}"/><text x="32" y="43" text-anchor="middle" fill="white" font-family="Arial" font-size="36" font-weight="700">${initial}</text></svg>`)}`;
    document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')?.setAttribute("content", brand.brand_color);
  }, [brand]);
  return null;
}

export function workspaceBrandingFromAccount(account: WorkspaceBranding): WorkspaceBranding {
  return { ...account, brand_color: account.brand_color || fallback.brand_color };
}
