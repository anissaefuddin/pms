import { statusDotClass } from "@/lib/status-colors";

// Minimal local copies; swap for lane2's StatusChip/AvatarStack once merged.
const groupColor = { active: "blue", completed: "green", deferred: "amber", cancelled: "slate" } as const;

export function StatusPill({ name, color, group }: { name: string; color?: string; group?: string }) {
  const token = color ?? groupColor[group as keyof typeof groupColor] ?? "slate";
  return <span className="inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs">
    <span aria-hidden="true" className={`size-2 rounded-full ${statusDotClass(token)}`} />{name}</span>;
}

export function Avatars({ people }: { people: { id: string; name: string }[] }) {
  if (people.length === 0) return null;
  return <span className="flex -space-x-1" title={people.map((p) => p.name).join(", ")}>
    {people.slice(0, 3).map((p) => <span key={p.id} aria-hidden="true"
      className="grid size-6 place-items-center rounded-full border bg-muted text-[10px] font-medium">{p.name.slice(0, 1).toUpperCase()}</span>)}
    <span className="sr-only">{people.map((p) => p.name).join(", ")}</span></span>;
}
