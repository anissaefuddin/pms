import { createFileRoute } from "@tanstack/react-router";
import { CustomFieldRegistry } from "@/components/custom-field-registry";

export const Route = createFileRoute("/_app/settings/custom-fields")({ component: CustomFieldsSettings });

// Work configuration > Custom fields. Administrators and Space owners manage fields here; no Personal Space detour.
function CustomFieldsSettings() {
  return <CustomFieldRegistry />;
}
