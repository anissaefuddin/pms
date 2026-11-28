// Test-only harness for the Customer Picker (B2-08). It is not linked from the app, not in the router, and not part of
// the production build (vite builds index.html only). It mounts the real picker with a fixed profile and records every change.
import { useState } from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import i18n from "../../../src/i18n";
import "../../../src/index.css";
import { ProfileContext } from "../../../src/lib/profile";
import { CustomerPicker } from "../../../src/components/customer-picker";
import type { CustomerLookupScope } from "../../../src/lib/customer-picker";
import type { CustomerOption } from "../../../src/lib/customers";

const params = new URLSearchParams(location.search);
const USER_ONE = "11111111-1111-4111-8111-111111111111";
const USER_TWO = "33333333-3333-4333-8333-333333333333";

declare global {
  interface Window { __changes: (string | null)[] }
}
window.__changes = [];

if (params.get("lang") === "id") void i18n.changeLanguage("id");

function Harness() {
  const [user, setUser] = useState({ id: USER_ONE, user_type: params.get("user_type") ?? "admin" });
  const [value, setValue] = useState<string | null>(params.get("value"));
  const [selected, setSelected] = useState<CustomerOption | null>(null);
  const scope: CustomerLookupScope = params.get("scope") === "administrative" ? "administrative" : "membership";
  const profile = user as unknown as React.ContextType<typeof ProfileContext>;
  return <ProfileContext.Provider value={profile}>
    <main className="max-w-md space-y-4 p-6">
      <h1 className="text-lg font-semibold">Customer picker</h1>
      <CustomerPicker label="Customer" value={value} selected={selected} scope={scope}
        clearable={params.get("clearable") !== "0"} disabled={params.get("disabled") === "1"}
        onChange={(customer) => {
          window.__changes.push(customer ? customer.id : null);
          setValue(customer ? customer.id : null);
          setSelected(customer);
        }} />
      <button type="button" onClick={() => setUser({ id: USER_TWO, user_type: "regular" })}>Switch user</button>
    </main>
  </ProfileContext.Provider>;
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <Harness />
  </QueryClientProvider>,
);
