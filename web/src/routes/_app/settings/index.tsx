import { Navigate, createFileRoute } from "@tanstack/react-router";

// Rendered (not beforeLoad) so the auth guard in _app runs first and keeps `next=/settings`.
export const Route = createFileRoute("/_app/settings/")({ component: () => <Navigate to="/settings/security" replace /> });
