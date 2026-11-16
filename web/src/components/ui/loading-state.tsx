/** Loading message for a work area region. role="status" so screen readers announce it once. */
export function LoadingState({ label, className }: { label: string; className?: string }) {
  return <p role="status" className={className ?? "text-sm text-muted-foreground"}>{label}</p>;
}
