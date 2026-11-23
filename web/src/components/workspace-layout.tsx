import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * Shared frame for work pages (Space, Project, Task): header, then view navigation, then the contextual toolbar, then content.
 * Slots are optional; the pages control vertical spacing through className.
 */
export function WorkspaceLayout({ children, className, header, nav, toolbar }: {
  children?: ReactNode;
  className?: string;
  header?: ReactNode;
  nav?: ReactNode;
  toolbar?: ReactNode;
}) {
  return <main className={cn("mx-auto w-full p-3 sm:p-4", className)}>
    {header}
    {nav}
    {toolbar}
    {children}
  </main>;
}
