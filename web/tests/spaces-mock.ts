import type { Page } from "@playwright/test";
import { me, mockApi, problem, type Handler } from "./mock-api";

export const ALL = ["view", "comment", "change_status", "attach", "edit", "create", "delete", "share"];
export const personal = { id: me.personal_space_id, name: "Ada Lovelace", description: null, kind: "personal", version: 1, my_capabilities: ALL };
export const marketing = { id: "33333333-3333-4333-8333-333333333333", name: "Marketing", description: "Campaigns and launches", kind: "shared", version: 4, my_capabilities: ALL };
export const ops = { id: "44444444-4444-4444-8444-444444444444", name: "Operations", description: null, kind: "shared", version: 2, my_capabilities: ALL };

export const grace = { id: "55555555-5555-4555-8555-555555555555", email: "grace@example.test", first_name: "Grace", last_name: "Hopper", user_type: "regular", status: "active" };
export const team = { id: "66666666-6666-4666-8666-666666666666", name: "Design team", member_ids: [] as string[] };

type Space = typeof marketing;
const NEW_ID = "77777777-7777-4777-8777-777777777777";

/** Stateful fake of the Space endpoints; `extra` handlers win over the defaults. */
export async function mockSpaces(page: Page, opts: { me?: typeof me; spaces?: Space[]; extra?: Record<string, Handler> } = {}) {
  const user = opts.me ?? me;
  const spaces: Space[] = [...(opts.spaces ?? [personal, marketing, ops])];
  const handlers: Record<string, Handler> = {
    "GET /me": { body: user },
    "GET /spaces": () => ({ body: { items: spaces } }),
    "POST /spaces": (body) => {
      const created = { id: NEW_ID, description: null, kind: "shared", version: 1, my_capabilities: ALL, ...body };
      spaces.push(created);
      return { status: 201, body: created };
    },
  };
  for (const s of [...spaces, { id: NEW_ID }]) {
    handlers[`GET /spaces/${s.id}/items`] = { body: { items: [], next_cursor: null } };
    handlers[`GET /spaces/${s.id}/custom-fields`] = { body: { items: [] } };
    handlers[`GET /spaces/${s.id}`] = () => {
      const current = spaces.find((x) => x.id === s.id);
      return current ? { body: current } : problem(404, "space not found", "not_found");
    };
    handlers[`PATCH /spaces/${s.id}`] = (body, req) => {
      const i = spaces.findIndex((x) => x.id === s.id);
      if (req.headers["if-match"] !== `"${spaces[i].version}"`) return problem(409, "version conflict", "version_conflict");
      spaces[i] = { ...spaces[i], ...body, version: spaces[i].version + 1 };
      return { body: spaces[i] };
    };
    handlers[`DELETE /spaces/${s.id}`] = () => {
      spaces.splice(spaces.findIndex((x) => x.id === s.id), 1);
      return { status: 204 };
    };
  }
  const calls = await mockApi(page, { ...handlers, ...opts.extra });
  return { calls, spaces };
}

/** Share-related endpoints for the Marketing space. */
const inheritedFrom = "88888888-8888-4888-8888-888888888888";
export const shareRows = [
  { principal_type: "user", principal_id: me.id, principal_name: "Ada Lovelace", role: "full", inherited_from: null },
  { principal_type: "user", principal_id: "99999999-9999-4999-8999-999999999990", principal_name: "Linus Pauling", role: "editor", inherited_from: null },
  { principal_type: "group", principal_id: "99999999-9999-4999-8999-999999999991", principal_name: "Everyone in Sales", role: "read", inherited_from: inheritedFrom },
];

export function extra(overrides: Record<string, any> = {}) {
  return {
    [`GET /spaces/${marketing.id}/shares`]: () => ({ headers: { ETag: '"7"' }, body: { items: shareRows } }),
    [`PUT /spaces/${marketing.id}/shares`]: (body: any, req) => {
      if (!req.headers["if-match"]) return problem(428, "If-Match required");
      if (req.headers["if-match"] !== '"7"') return problem(409, "version conflict");
      return { headers: { ETag: '"8"' }, body: { items: body.shares.map((s: any) => ({ ...s, principal_name: "x" })) } };
    },
    "GET /users": (_: unknown, req: { query: URLSearchParams }) => {
      const q = (req.query.get("q") ?? "").toLowerCase();
      return { body: { items: [grace].filter((u) => `${u.first_name} ${u.last_name} ${u.email}`.toLowerCase().includes(q)) } };
    },
    "GET /groups": { body: { items: [team] } },
    ...overrides,
  };
}

