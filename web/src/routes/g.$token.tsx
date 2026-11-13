/* eslint-disable i18next/no-literal-string */
import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { PublicGanttSnapshot, type PublicGantt } from "@/components/public-gantt-snapshot";
import { api } from "@/lib/api";

export const Route = createFileRoute("/g/$token")({ component: PublicGanttPage });

function PublicGanttPage() {
  const { token } = Route.useParams();
  const snapshot = useQuery({
    queryKey: ["public-gantt", token],
    retry: false,
    queryFn: () => api<PublicGantt>(`/api/v1/public/gantt/${encodeURIComponent(token)}`),
  });
  if (snapshot.isPending) return <main className="p-8">Loading shared plan…</main>;
  if (!snapshot.data) return <main className="p-8"><h1 className="text-2xl font-semibold">This snapshot is unavailable</h1><p className="mt-2 text-muted-foreground">The link may be invalid or expired.</p></main>;
  return <PublicGanttSnapshot snapshot={snapshot.data} />;
}
