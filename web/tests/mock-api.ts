import type { Page, Route } from "@playwright/test";

export const me = {
  id: "11111111-1111-4111-8111-111111111111",
  email: "ada@example.test",
  first_name: "Ada",
  last_name: "Lovelace",
  user_type: "admin",
  status: "active",
  locale: "en",
  timezone: "Asia/Jakarta",
  date_format: "yyyy-mm-dd",
  time_format: "24h",
  week_start: "monday",
  personal_space_id: "22222222-2222-4222-8222-222222222222",
  capabilities: { manage_users: true, create_space: true },
};

export type Reply = { status?: number; body?: unknown };
export type Handler = Reply | ((body: any) => Reply);
export type Calls = { method: string; path: string; body: any }[];

export function problem(status: number, detail: string, code = "problem"): Reply {
  return { status, body: { type: "about:blank", title: detail, status, code, detail, request_id: "req-test" } };
}

/** Mocks /api/v1/* so specs need no backend. Keys look like "POST /auth/login". Unlisted calls get 404. */
export async function mockApi(page: Page, handlers: Record<string, Handler>): Promise<Calls> {
  const calls: Calls = [];
  await page.route(/\/api\/v1\//, async (route: Route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    let body: any;
    try { body = request.postDataJSON(); } catch { body = undefined; }
    calls.push({ method: request.method(), path, body });
    const handler = handlers[`${request.method()} ${path}`];
    const reply = handler === undefined ? problem(404, "not mocked") : typeof handler === "function" ? handler(body) : handler;
    const status = reply.status ?? 200;
    await route.fulfill({
      status,
      contentType: status >= 400 ? "application/problem+json" : "application/json",
      body: reply.body === undefined || status === 204 ? undefined : JSON.stringify(reply.body),
    });
  });
  return calls;
}
