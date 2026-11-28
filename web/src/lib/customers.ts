import type { components } from "@/api/schema";
import { ApiError, api } from "@/lib/api";
import { CUSTOMER_PICKER_LIMIT, eligibleCustomersKey, type CustomerLookupScope } from "@/lib/customer-picker";

// Customer Master (B2-07). Types come from the generated OpenAPI schema; nothing here is hand-copied.
export type Customer = components["schemas"]["Client"];
export type CustomerMembership = components["schemas"]["ClientMembership"];
export type CustomerOption = components["schemas"]["ClientOption"];
export type CustomerFilter = "active" | "archived";

/** The backend returns at most this many customers per request and has no cursor (B2-06). */
export const CUSTOMER_PAGE_LIMIT = 200;

export const customerKeys = {
  list: (filter: CustomerFilter) => ["clients", filter] as const,
  detail: (id: string) => ["client", id] as const,
  members: (id: string, history: boolean) => ["client-members", id, history] as const,
  eligible: eligibleCustomersKey,
};

export function listCustomers(filter: CustomerFilter) {
  // "active" and "archived" map to the API status filter; the list is always the first page of at most 200.
  return api<{ items: Customer[] }>(`/api/v1/clients?status=${filter}&limit=${CUSTOMER_PAGE_LIMIT}`);
}

/** Picker options (B2-08). The membership scope is the default; administrative is for configuration screens only. */
export function listEligibleCustomers(scope: CustomerLookupScope, search: string) {
  const params = new URLSearchParams({ scope, limit: String(CUSTOMER_PICKER_LIMIT) });
  if (search) params.set("q", search);
  return api<{ items: CustomerOption[] }>(`/api/v1/clients/eligible?${params}`);
}

export function getCustomer(id: string) {
  return api<Customer>(`/api/v1/clients/${id}`);
}

export function createCustomer(body: { code: string; name: string; short_name?: string }) {
  return api<Customer>("/api/v1/clients", { method: "POST", body: JSON.stringify(body) });
}

/** Only the mutable fields. The code and id are never sent (the API refuses them). */
export function updateCustomer(id: string, body: { name: string; short_name: string }) {
  return api<Customer>(`/api/v1/clients/${id}`, { method: "PATCH", body: JSON.stringify(body) });
}

export function archiveCustomer(id: string) {
  return api<Customer>(`/api/v1/clients/${id}/archive`, { method: "POST" });
}

export function listMembers(id: string, history: boolean) {
  return api<{ items: CustomerMembership[] }>(`/api/v1/clients/${id}/members?include_revoked=${history ? "true" : "false"}`);
}

export function grantMembership(id: string, userId: string) {
  return api<CustomerMembership>(`/api/v1/clients/${id}/members`, { method: "POST", body: JSON.stringify({ user_id: userId }) });
}

export function revokeMembership(id: string, userId: string) {
  return api<CustomerMembership>(`/api/v1/clients/${id}/members/${userId}`, { method: "DELETE" });
}

/**
 * Eligible users for a new membership: active administrators and staff only. The API enforces this too; filtering here only
 * keeps the picker honest. Partner and external users are not offered (K-19).
 */
export function eligibleForMembership<T extends { id: string; user_type: string; status: string }>(users: T[], activeMemberIds: Set<string>): T[] {
  return users.filter((user) => user.status === "active" && (user.user_type === "admin" || user.user_type === "regular") && !activeMemberIds.has(user.id));
}

/** Translation key for a safe, known API problem code. Unknown codes fall back to the server's safe message or a generic one. */
export function customerErrorKey(err: unknown): { key?: string; message?: string } {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "duplicate_code": return { key: "settings.customers.errors.duplicateCode" };
      case "customer_archived": return { key: "settings.customers.errors.archived" };
      case "user_not_eligible": return { key: "settings.customers.errors.notEligible" };
      case "user_not_found": return { key: "settings.customers.errors.userNotFound" };
      case "membership_not_found": return { key: "settings.customers.errors.membershipNotFound" };
      case "validation_failed": return { message: err.message };
      default:
        if (err.status >= 500) return { key: "settings.customers.errors.generic" };
        return { message: err.message };
    }
  }
  return { key: "settings.customers.errors.generic" };
}

/** The text to show for a failed request: a translated message for known codes, the server's safe message otherwise. */
export function customerErrorText(t: (key: string) => string, err: unknown): string {
  const { key, message } = customerErrorKey(err);
  return key ? t(key) : (message ?? t("settings.customers.errors.generic"));
}
