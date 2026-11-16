import * as React from "react"
import { Tooltip as TooltipPrimitive } from "@base-ui/react/tooltip"
import { cn } from "@/lib/utils"

/** Tooltip with its own provider, so each use works without a app-wide provider. */
function Tooltip({ delay = 300, ...props }: TooltipPrimitive.Root.Props & { delay?: number }) {
  return <TooltipPrimitive.Provider delay={delay}><TooltipPrimitive.Root {...props} /></TooltipPrimitive.Provider>
}

function TooltipTrigger(props: TooltipPrimitive.Trigger.Props) {
  return <TooltipPrimitive.Trigger {...props} />
}

function TooltipContent({ className, side = "right", sideOffset = 8, children, ...props }: TooltipPrimitive.Popup.Props & { side?: "top" | "right" | "bottom" | "left"; sideOffset?: number }) {
  return <TooltipPrimitive.Portal>
    <TooltipPrimitive.Positioner side={side} sideOffset={sideOffset}>
      <TooltipPrimitive.Popup className={cn("z-50 rounded-md bg-foreground px-2 py-1 text-xs text-background shadow-md", className)} {...props}>{children}</TooltipPrimitive.Popup>
    </TooltipPrimitive.Positioner>
  </TooltipPrimitive.Portal>
}

export { Tooltip, TooltipTrigger, TooltipContent }
