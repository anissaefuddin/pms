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

function relativeLuminance(hex: string) {
  const value = hex.replace("#", "");
  if (!/^[0-9a-f]{6}$/i.test(value)) return 0;
  const channels = [0, 2, 4].map((offset) => Number.parseInt(value.slice(offset, offset + 2), 16) / 255);
  return channels.map((channel) => channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4)
    .reduce((sum, channel, index) => sum + channel * [0.2126, 0.7152, 0.0722][index], 0);
}

function readableForeground(hex: string) {
  const luminance = relativeLuminance(hex);
  const whiteContrast = 1.05 / (luminance + 0.05);
  const blackContrast = (luminance + 0.05) / 0.05;
  return whiteContrast >= blackContrast ? "#FFFFFF" : "#111827";
}

function darkModeBrandColor(hex: string) {
  const value = hex.replace("#", "");
  if (!/^[0-9a-f]{6}$/i.test(value)) return "#93C5FD";
  const source = [0, 2, 4].map((offset) => Number.parseInt(value.slice(offset, offset + 2), 16));
  const background = relativeLuminance("#1F2937");
  for (let amount = 0; amount <= 1; amount += 0.05) {
    const candidate = `#${source.map((channel) => Math.round(channel + (255 - channel) * amount).toString(16).padStart(2, "0")).join("")}`;
    const contrast = (relativeLuminance(candidate) + 0.05) / (background + 0.05);
    if (contrast >= 4.5) return candidate;
  }
  return "#FFFFFF";
}

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
    const applyThemeColor = () => {
      const color = root.classList.contains("dark") ? darkModeBrandColor(brand.brand_color) : brand.brand_color;
      root.style.setProperty("--brand-primary", color);
      root.style.setProperty("--brand-primary-foreground", readableForeground(color));
      root.style.setProperty("--ring", color);
      root.style.setProperty("--sidebar-primary", color);
    };
    applyThemeColor();
    const themeObserver = new MutationObserver(applyThemeColor);
    themeObserver.observe(root, { attributes: true, attributeFilter: ["class"] });
    document.title = brand.name;
    let icon = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
    if (!icon) { icon = document.createElement("link"); icon.rel = "icon"; document.head.append(icon); }
    const initial = brand.name.trim().charAt(0).toUpperCase() || "L";
    icon.href = brand.icon_image_version > 0 ? workspaceAssetURL("icon", brand.icon_image_version)
      : `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="12" fill="${brand.brand_color}"/><text x="32" y="43" text-anchor="middle" fill="white" font-family="Arial" font-size="36" font-weight="700">${initial}</text></svg>`)}`;
    document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')?.setAttribute("content", brand.brand_color);
    return () => themeObserver.disconnect();
  }, [brand]);
  return null;
}

export function workspaceBrandingFromAccount(account: WorkspaceBranding): WorkspaceBranding {
  return { ...account, brand_color: account.brand_color || fallback.brand_color };
}
