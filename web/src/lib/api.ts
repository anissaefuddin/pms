import type { components, paths } from "@/api/schema";

export type CurrentUser = components["schemas"]["Me"];

export async function checkHealth() {
  const path = "/healthz" satisfies keyof paths;
  const method = "get" satisfies keyof paths[typeof path];
  const success = 200 satisfies keyof paths[typeof path][typeof method]["responses"];
  const response = await fetch(`/api/v1${path}`, { method: method.toUpperCase() });
  if (response.status !== success) throw new Error("Server health check failed");
  return success;
}

export class ApiError extends Error {
  constructor(message: string, readonly status: number, readonly code?: string,
    readonly fields: { field: string; message: string }[] = [], readonly retryAfter?: number) { super(message); }
  get sessionExpired() { return this.status === 401 && this.message === "session expired"; }
}

export async function api<T>(path: string, init?: RequestInit, onHeaders?: (headers: Headers) => void): Promise<T> {
  const headers = new Headers(init?.headers);
  if (!(init?.body instanceof FormData) && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  const response = await fetch(path, { credentials: "same-origin", ...init, headers });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    const fields = Array.isArray(body.errors) ? body.errors.filter((entry: unknown) =>
      !!entry && typeof entry === "object" && "field" in entry && "message" in entry
      && typeof entry.field === "string" && typeof entry.message === "string") : [];
    const retry = response.headers.get("Retry-After");
    const retryAfter = retry && /^\d+$/.test(retry) ? Number(retry) : undefined;
    const error = new ApiError(body.detail ?? body.title ?? "Request failed", response.status, body.code, fields, retryAfter);
    if (error.sessionExpired && typeof window !== "undefined") window.dispatchEvent(new Event("pms:session-expired"));
    throw error;
  }
  onHeaders?.(response.headers);
  if (response.status === 204) return undefined as T;
  // 202 is usually bodiless ("accepted"), but the login 2FA challenge carries JSON.
  return response.status === 202 ? response.json().catch(() => undefined) : response.json();
}
