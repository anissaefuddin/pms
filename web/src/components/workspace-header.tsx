import type { ReactNode } from "react";
import { StarButton } from "@/components/star-button";

/** Compact header for Space and Project pages: optional breadcrumb, icon, title with Star, meta line, actions, and a summary slot. */
export function WorkspaceHeader({ breadcrumb, icon, title, starItemId, meta, actions, children }: {
  breadcrumb?: ReactNode;
  icon?: ReactNode;
  title: string;
  starItemId?: string;
  meta?: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
}) {
  return <header className="space-y-4 border-b pb-5">
    {breadcrumb}
    <div className="flex flex-wrap items-start justify-between gap-4">
      <div className="min-w-0">
        <div className="flex min-w-0 items-center gap-2">
          {icon}
          <h1 className="truncate workspace-title">{title}</h1>
          {starItemId && <StarButton itemId={starItemId} />}
        </div>
        {meta}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
    {children}
  </header>;
}
