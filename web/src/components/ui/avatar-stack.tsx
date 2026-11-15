import { cn } from "@/lib/utils";

const initials = (name: string) => name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => Array.from(part)[0]).join("").toLocaleUpperCase();

/** The one avatar stack for the work area. Overlapping initials circles; names stay available as the accessible label and title. */
export function AvatarStack({ people, max = 3, className }: { people: { id: string; name: string }[]; max?: number; className?: string }) {
  if (!people.length) return null;
  const shown = people.slice(0, max), extra = people.length - shown.length;
  return <span role="img" aria-label={people.map((person) => person.name).join(", ")} title={people.map((person) => person.name).join(", ")}
    className={cn("inline-flex items-center -space-x-1.5", className)}>
    {shown.map((person) => <span key={person.id} aria-hidden className="grid size-6 place-items-center rounded-full border-2 border-card bg-primary/15 text-[10px] font-semibold text-foreground">{initials(person.name)}</span>)}
    {extra > 0 && <span aria-hidden className="grid size-6 place-items-center rounded-full border-2 border-card bg-muted text-[10px] font-semibold text-foreground">+{extra}</span>}
  </span>;
}
