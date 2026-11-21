import { createFileRoute, Navigate } from "@tanstack/react-router";

export const Route = createFileRoute("/_app/")({ component: Workspace });

// The home page is Dashboards. Login sends users to "/" when no other page was requested, so this is what they see first.
function Workspace() {
  return <Navigate to="/dashboards" replace />;
}
