import { useEffect, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { StarButton } from "@/components/star-button";
import { TopbarTitle } from "@/components/topbar-title";

/** Compact header for Space and Project pages: optional breadcrumb, icon, title with Star, meta line, actions, and a summary slot.
 * titleInTopbar moves the icon, title and Star into the top bar, where the breadcrumb goes. */
export function WorkspaceHeader({ breadcrumb, icon, title, starItemId, meta, actions, children, titleInTopbar = false }: {
  breadcrumb?: ReactNode;
  icon?: ReactNode;
  title: string;
  starItemId?: string;
  meta?: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
  titleInTopbar?: boolean;
}) {
  // The breadcrumb moves into the top bar when it is there; otherwise it stays above the title.
  const [slot, setSlot] = useState<HTMLElement | null>(null);
  useEffect(() => { setSlot(document.getElementById("topbar-breadcrumb")); }, []);
  return <header className="space-y-4 border-b pb-5">
    {breadcrumb && (slot ? createPortal(breadcrumb, slot) : breadcrumb)}
    {titleInTopbar && <TopbarTitle extra={starItemId && <StarButton itemId={starItemId} />}>{icon}{title}</TopbarTitle>}
    <div className="flex flex-wrap items-start justify-between gap-4">
      <div className="min-w-0">
        {!titleInTopbar && <div className="flex min-w-0 items-center gap-2">
          {icon}
          <h1 className="truncate workspace-title">{title}</h1>
          {starItemId && <StarButton itemId={starItemId} />}
        </div>}
        {meta}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
    {children}
  </header>;
}
