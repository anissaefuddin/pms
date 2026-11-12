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
  constructor(message: string, readonly status: number, readonly code?: string) { super(message); }
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, { credentials: "same-origin", ...init, headers: { "Content-Type": "application/json", ...init?.headers } });
  if (!response.ok) { const body = await response.json().catch(() => ({})); throw new ApiError(body.detail ?? body.title ?? "Request failed", response.status, body.code); }
  return response.status === 204 || response.status === 202 ? (undefined as T) : response.json();
}
