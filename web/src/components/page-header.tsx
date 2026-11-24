import type { ReactNode } from "react";

/** Width, gutter and vertical rhythm shared by every top-level page (Inbox, Stream, Requests, Reports, ...). */
export const pageFrame = "mx-auto w-full max-w-7xl space-y-6 p-4 sm:p-8";

/** Title, optional description and actions, styled like the Space and Project header so every page starts the same way. */
export function PageHeader({ title, description, actions }: { title: string; description?: string; actions?: ReactNode }) {
  return <header className="flex flex-wrap items-start justify-between gap-4 border-b pb-5">
    <div className="min-w-0">
      <h1 className="workspace-title">{title}</h1>
      {description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
    </div>
    {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
  </header>;
}
