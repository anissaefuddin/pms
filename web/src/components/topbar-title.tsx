import { useEffect, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";

/** A page title shown in the top bar, where the item breadcrumb sits. Without a top bar it stays in the page. */
export function TopbarTitle({ children, extra }: { children: ReactNode; extra?: ReactNode }) {
  const [slot, setSlot] = useState<{ node: HTMLElement | null } | null>(null);
  // Read the slot after mount: the top bar is rendered by the layout, not by the page.
  useEffect(() => { setSlot({ node: document.getElementById("topbar-breadcrumb") }); }, []);
  if (!slot) return null;
  const title = <div className="flex min-w-0 items-center gap-2">
    <h1 className="flex min-w-0 items-center gap-2 text-base font-semibold"><span className="min-w-0 truncate">{children}</span></h1>
    {extra}
  </div>;
  return slot.node ? createPortal(title, slot.node) : title;
}
