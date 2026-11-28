import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me, problem, type Handler } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

// Customer Settings (B2-07). The API is mocked with a small in-test state, so every assertion reads what the page sent
// and what it then displayed. Backend authorization is covered by the Go tests; these tests cover the interface.

const ALPHA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const OLD = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const GRACE = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const MIA = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
const EXTERNAL = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee";
const DEACTIVATED = "ffffffff-ffff-4fff-8fff-ffffffffffff";

type Client = Record<string, unknown>;
type Member = Record<string, unknown>;

function customer(id: string, code: string, name: string, status: "active" | "archived", extra: Client = {}): Client {
  return { id, code, name, short_name: null, status, created_at: "2026-11-20T10:00:00Z", created_by: me.id,
    updated_at: "2026-11-20T10:00:00Z", archived_at: status === "archived" ? "2026-11-21T10:00:00Z" : null,
    archived_by: status === "archived" ? me.id : null, ...extra };
}

function membership(id: string, userId: string, userName: string, status: "active" | "revoked"): Member {
  return { id, client_id: ALPHA, user_id: userId, user_name: userName, status, granted_at: "2026-11-20T11:00:00Z",
    granted_by: me.id, revoked_at: status === "revoked" ? "2026-11-22T09:00:00Z" : null, revoked_by: status === "revoked" ? me.id : null };
}

const users = [
  { id: me.id, first_name: "Ada", last_name: "Lovelace", email: "ada@example.test", user_type: "admin", status: "active" },
  { id: MIA, first_name: "Mia", last_name: "Member", email: "mia@example.test", user_type: "regular", status: "active" },
  { id: GRACE, first_name: "Grace", last_name: "Hopper", email: "grace@example.test", user_type: "regular", status: "active" },
  { id: EXTERNAL, first_name: "Ext", last_name: "Partner", email: "ext@example.test", user_type: "external", status: "active" },
  { id: DEACTIVATED, first_name: "Old", last_name: "Person", email: "old@example.test", user_type: "regular", status: "deactivated" },
];

test("admin sees Customers in Settings, lists by state, creates a customer, and sees duplicate codes refused", async ({ page }) => {
  const state = { clients: [customer(ALPHA, "ALPHA", "Alpha Holding", "active"), customer(OLD, "OLD-CO", "Old Co", "archived")] };
  const { calls } = await mockSpaces(page, { extra: {
    "GET /clients": (_body, req) => ({ body: { items: state.clients.filter((c) => c.status === req.query.get("status")) } }),
    "POST /clients": (body: { code: string; name: string }) => {
      if (state.clients.some((c) => c.code === body.code.trim().toUpperCase())) return problem(409, "customer code already exists", "duplicate_code");
      const created = customer("99999999-9999-4999-8999-999999999999", body.code.trim().toUpperCase(), body.name, "active");
      state.clients.push(created);
      return { status: 201, body: created };
    },
  } as Record<string, Handler> });

  await page.goto("/settings/customers");
  await expect(page.getByRole("navigation", { name: "Settings navigation" }).getByRole("link", { name: "Customers" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Alpha Holding" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Old Co" })).toHaveCount(0);

  await page.getByRole("button", { name: "Archived" }).click();
  await expect(page.getByRole("link", { name: "Old Co" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Alpha Holding" })).toHaveCount(0);
  await page.getByRole("button", { name: "Active" }).click();

  await page.getByRole("button", { name: "New customer" }).first().click();
  const dialog = page.getByRole("dialog", { name: "New customer" });
  await dialog.getByLabel("Customer code").fill("  beta-ltd ");
  await dialog.getByLabel("Customer name").fill("Beta Ltd");
  await dialog.getByRole("button", { name: "Create customer" }).click();
  await expect(page.getByRole("link", { name: "Beta Ltd" })).toBeVisible();
  const post = calls.find((c) => c.method === "POST" && c.path === "/clients");
  expect(post?.body).toEqual({ code: "beta-ltd", name: "Beta Ltd" }); // no id, status, or actor field

  await page.getByRole("button", { name: "New customer" }).first().click();
  const again = page.getByRole("dialog", { name: "New customer" });
  await again.getByLabel("Customer code").fill("alpha");
  await again.getByLabel("Customer name").fill("Duplicate");
  await again.getByRole("button", { name: "Create customer" }).click();
  await expect(again.getByRole("alert")).toHaveText("A customer with this code already exists.");
  await expect(again.getByLabel("Customer name")).toHaveValue("Duplicate"); // entered values survive a failed submit
});

test("a regular user sees no Customers entry and the page refuses direct access without calling the API", async ({ page }) => {
  const regular = { ...me, user_type: "regular", capabilities: { ...me.capabilities, manage_users: false } };
  const { calls } = await mockSpaces(page, { me: regular as typeof me, spaces: [{ ...marketing, my_capabilities: ["view"] }] as any });
  await page.goto("/settings/security");
  await expect(page.getByRole("navigation", { name: "Settings navigation" }).getByRole("link", { name: "Customers" })).toHaveCount(0);
  await page.goto("/settings/customers");
  await expect(page.getByRole("alert")).toHaveText("Administrator access is required.");
  expect(calls.some((c) => c.path.startsWith("/clients"))).toBe(false);
});

test("admin edits a customer, adds and revokes members, and archives, sending only permitted fields", async ({ page }) => {
  const state = {
    client: customer(ALPHA, "ALPHA", "Alpha Holding", "active", { short_name: "ALPHA" }),
    members: [membership("m-grace", GRACE, "Grace Hopper", "active")] as Member[],
  };
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /clients/${ALPHA}`]: () => ({ body: state.client }),
    [`PATCH /clients/${ALPHA}`]: (body: Record<string, unknown>) => {
      state.client = { ...state.client, name: body.name, short_name: body.short_name };
      return { body: state.client };
    },
    [`POST /clients/${ALPHA}/archive`]: () => {
      state.client = { ...state.client, status: "archived", archived_at: "2026-11-27T08:00:00Z", archived_by: me.id };
      return { body: state.client };
    },
    [`GET /clients/${ALPHA}/members`]: (_body, req) => ({ body: { items: state.members.filter((m) => m.status === "active" || req.query.get("include_revoked") === "true") } }),
    [`POST /clients/${ALPHA}/members`]: (body: { user_id: string }) => {
      const found = users.find((u) => u.id === body.user_id);
      const row = membership(`m-${body.user_id}`, body.user_id, found ? `${found.first_name} ${found.last_name}` : "", "active");
      state.members.push(row);
      return { body: row };
    },
    [`DELETE /clients/${ALPHA}/members/${GRACE}`]: () => {
      state.members = state.members.map((m) => (m.user_id === GRACE ? { ...m, status: "revoked", revoked_at: "2026-11-27T08:10:00Z", revoked_by: me.id } : m));
      return { body: state.members.find((m) => m.user_id === GRACE) };
    },
    "GET /users": () => ({ body: { items: users, next_cursor: null } }),
  } as Record<string, Handler> });

  await page.goto(`/settings/customers/${ALPHA}`);
  await expect(page.getByRole("heading", { name: "Alpha Holding" })).toBeVisible();
  await expect(page.getByText("ALPHA", { exact: true }).first()).toBeVisible(); // the code is shown read-only
  const nameField = page.getByLabel("Name", { exact: true });
  await nameField.fill("Alpha Group");
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page.getByRole("heading", { name: "Alpha Group" })).toBeVisible();
  const patch = calls.find((c) => c.method === "PATCH");
  expect(patch?.body).toEqual({ name: "Alpha Group", short_name: "ALPHA" });
  expect(Object.keys(patch?.body ?? {})).not.toContain("code");

  await page.getByRole("tab", { name: "Members" }).click();
  await expect(page.getByText("Client membership defines eligibility. Space and item permissions are managed separately.")).toBeVisible();
  await expect(page.getByRole("cell", { name: "Grace Hopper" })).toBeVisible();

  await page.getByRole("button", { name: "Add member" }).click();
  const picker = page.getByRole("dialog", { name: "Add a member" });
  await expect(picker.getByRole("button", { name: "Add Mia Member" })).toBeVisible();
  await expect(picker.getByRole("button", { name: "Add Ext Partner" })).toHaveCount(0); // partners are not offered
  await expect(picker.getByRole("button", { name: "Add Old Person" })).toHaveCount(0); // deactivated users are not offered
  await expect(picker.getByRole("button", { name: "Add Grace Hopper" })).toHaveCount(0); // already an active member
  await picker.getByRole("button", { name: "Add Mia Member" }).click();
  await expect(page.getByRole("cell", { name: "Mia Member" })).toBeVisible();
  expect(calls.find((c) => c.method === "POST" && c.path.endsWith("/members"))?.body).toEqual({ user_id: MIA });

  await page.getByRole("button", { name: "Revoke" }).first().click();
  const confirm = page.getByRole("dialog", { name: "Revoke Grace Hopper?" });
  await expect(confirm).toContainText("does not remove resource permissions");
  await confirm.getByRole("button", { name: "Revoke membership" }).click();
  await expect(page.getByRole("cell", { name: "Grace Hopper" })).toHaveCount(0); // the default view shows active members only
  expect(calls.some((c) => c.method === "DELETE" && c.path === `/clients/${ALPHA}/members/${GRACE}`)).toBe(true);
  await page.getByRole("checkbox", { name: "Show revoked history" }).check();
  await expect(page.getByRole("cell", { name: "Revoked" })).toBeVisible();

  await page.getByRole("tab", { name: "General" }).click();
  await page.getByRole("button", { name: "Archive" }).click();
  const archiveDialog = page.getByRole("dialog", { name: "Archive Alpha Group?" });
  await expect(archiveDialog).toContainText("archived, not deleted");
  await archiveDialog.getByRole("button", { name: "Archive customer" }).click();
  await expect(page.getByLabel("Name", { exact: true })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Save changes" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Archive" })).toHaveCount(0); // no unarchive or delete in this release
  expect(calls.some((c) => c.method === "POST" && c.path === `/clients/${ALPHA}/archive`)).toBe(true);
});

test("the Indonesian interface shows translated Customer copy", async ({ page }) => {
  await mockSpaces(page, { me: { ...me, locale: "id" } as typeof me, extra: {
    "GET /clients": { body: { items: [customer(ALPHA, "ALPHA", "Alpha Holding", "active")] } },
  } as Record<string, Handler> });
  await page.goto("/settings/customers");
  await expect(page.getByRole("heading", { name: "Pelanggan" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Pelanggan baru" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Diarsipkan" })).toBeVisible();
});

test("list failure shows a safe error with retry; archived customers stay read-only for membership", async ({ page }) => {
  let failures = 1;
  const { calls } = await mockSpaces(page, { extra: {
    "GET /clients": () => failures-- > 0 ? problem(500, "database connection refused", "internal_error") : { body: { items: [customer(OLD, "OLD-CO", "Old Co", "archived")] } },
    [`GET /clients/${OLD}`]: () => ({ body: customer(OLD, "OLD-CO", "Old Co", "archived") }),
    [`GET /clients/${OLD}/members`]: () => ({ body: { items: [membership("m-old", MIA, "Mia Member", "revoked")] } }),
  } as Record<string, Handler> });

  await page.goto("/settings/customers");
  const alert = page.getByRole("alert");
  await expect(alert).toHaveText("Something went wrong. Try again.");
  await expect(alert).not.toContainText("database connection refused"); // raw server detail is never shown for 500s
  await page.getByRole("button", { name: "Try again" }).click();
  await expect(page.getByRole("link", { name: "Old Co" })).toBeVisible();

  await page.goto(`/settings/customers/${OLD}`);
  await expect(page.getByRole("heading", { name: "Old Co" })).toBeVisible();
  await expect(page.getByText("This customer is archived. Its history is kept and it is read-only.")).toBeVisible();
  await expect(page.getByLabel("Name", { exact: true })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Save changes" })).toHaveCount(0);
  await page.getByRole("tab", { name: "Members" }).click();
  await expect(page.getByRole("button", { name: "Add member" })).toBeDisabled();
  await expect(page.getByText("Archived customers cannot receive new members.")).toBeVisible();
  await expect(page.getByText(/^Full$|^Editor$|^Read$|^Limited$/)).toHaveCount(0); // no resource role controls
  expect(calls.some((c) => c.method === "PATCH")).toBe(false);
});

test("the customer list, detail and membership picker pass axe", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: {
    "GET /clients": { body: { items: [customer(ALPHA, "ALPHA", "Alpha Holding", "active")] } },
    [`GET /clients/${ALPHA}`]: { body: customer(ALPHA, "ALPHA", "Alpha Holding", "active") },
    [`GET /clients/${ALPHA}/members`]: { body: { items: [membership("m-grace", GRACE, "Grace Hopper", "active")] } },
    "GET /users": { body: { items: users, next_cursor: null } },
  } as Record<string, Handler> });
  await page.goto("/settings/customers");
  await expect(page.getByRole("link", { name: "Alpha Holding" })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByRole("button", { name: "New customer" }).first().click();
  await expect(page.getByRole("dialog", { name: "New customer" })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.goto(`/settings/customers/${ALPHA}`);
  await expect(page.getByRole("heading", { name: "Alpha Holding" })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByRole("tab", { name: "Members" }).click();
  await page.getByRole("button", { name: "Add member" }).click();
  await expect(page.getByRole("dialog", { name: "Add a member" })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  expect(calls.length).toBeGreaterThan(0);
});
