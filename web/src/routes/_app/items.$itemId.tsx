import { createFileRoute } from "@tanstack/react-router";
import { ItemWorkspace } from "./tasks.$itemId";

export const Route = createFileRoute("/_app/items/$itemId")({ component: ContainerPage });

function ContainerPage() {
  return <ItemWorkspace itemId={Route.useParams().itemId} />;
}
