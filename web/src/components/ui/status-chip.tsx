import { cn } from "@/lib/utils";
import { statusDotClass } from "@/lib/status-colors";

/** Static class strings so Tailwind keeps them; dark text on a light tint keeps AA contrast. */
export const statusTint: Record<string, string> = {
  slate: "bg-slate-100 text-slate-900", blue: "bg-blue-100 text-blue-900", sky: "bg-sky-100 text-sky-900", teal: "bg-teal-100 text-teal-900",
  green: "bg-green-100 text-green-900", lime: "bg-lime-100 text-lime-900", amber: "bg-amber-100 text-amber-900", orange: "bg-orange-100 text-orange-900",
  red: "bg-red-100 text-red-900", pink: "bg-pink-100 text-pink-900", purple: "bg-purple-100 text-purple-900", indigo: "bg-indigo-100 text-indigo-900",
};
export const tintOf = (color?: string) => statusTint[color ?? ""] ?? statusTint.slate;

/** Workflow status as a coloured chip (dot + name). The name is always text, never colour alone. */
export function StatusChip({ status, className }: { status: { name: string; color: string }; className?: string }) {
  return <span data-status-chip className={cn("inline-flex max-w-full items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium", tintOf(status.color), className)}>
    <span aria-hidden className={cn("size-2 shrink-0 rounded-full", statusDotClass(status.color))} />
    <span className="truncate">{status.name}</span>
  </span>;
}
