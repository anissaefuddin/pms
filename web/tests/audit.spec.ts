import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me, problem } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

const event = (id: number, action: string, over: Record<string, unknown> = {}) => ({
  id, at: "2026-11-14T03:00:00Z", actor_id: me.id, actor_name: "ada@example.test", action, target_type: "user", target_id: "u1", ip: "203.0.113.9", details: {}, ...over });

test("admin sees the trail, filters it, loads more, and exports with the same filters", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: {
    "GET /audit-events": (_b: unknown, req: { query: URLSearchParams }) => {
      if (req.query.get("cursor") === "10") return { body: { items: [event(9, "user.deactivated")], next_cursor: null } };
      if (req.query.get("category") === "auth") return { body: { items: [event(5, "auth.login_failed", { actor_id: null, actor_name: "who@example.test", details: { reason: "password" } })], next_cursor: null } };
      return { body: { items: [event(12, "auth.login", { details: { method: "password" } }), event(10, "share.space_changed", { target_type: "space", target_id: "s1" })], next_cursor: "10" } };
    },
  } });
  await page.goto("/settings/audit");
  await expect(page.getByRole("heading", { name: "Security audit", level: 1 })).toBeVisible();
  const rows = page.getByRole("row");
  await expect(rows.nth(1)).toContainText("Signed in");
  await expect(rows.nth(1)).toContainText("ada@example.test");
  await expect(rows.nth(1)).toContainText("203.0.113.9");
  await expect(rows.nth(2)).toContainText("Changed Space sharing");
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  await page.getByRole("button", { name: "Load more" }).click();
  await expect(page.getByRole("row", { name: /Deactivated a user/ })).toBeVisible();
  await expect(page.getByRole("button", { name: "Load more" })).toHaveCount(0);

  await page.getByLabel("Area").selectOption("auth");
  await expect(page.getByRole("row", { name: /Failed sign-in/ })).toContainText("who@example.test");
  await page.getByLabel("From", { exact: true }).fill("2026-11-01");
  await page.getByLabel("To", { exact: true }).fill("2026-11-14");
  await expect.poll(() => calls.filter((c) => c.path === "/audit-events").at(-1)?.query.get("to")).toBe("2026-11-14T23:59:59Z");
  const last = calls.filter((c) => c.path === "/audit-events").at(-1)!.query;
  expect(last.get("category")).toBe("auth");
  expect(last.get("from")).toBe("2026-11-01T00:00:00Z");
  const href = await page.getByRole("link", { name: "Export CSV" }).getAttribute("href");
  expect(href).toContain("/api/v1/audit-events/export?");
  expect(href).toContain("category=auth");
});

test("an empty result and a server error are explained, and a regular user is told it is admin only", async ({ page }) => {
  await mockSpaces(page, { extra: { "GET /audit-events": { body: { items: [], next_cursor: null } } } });
  await page.goto("/settings/audit");
  await expect(page.getByText("No events match these filters.")).toBeVisible();
  await page.unrouteAll();
  await mockSpaces(page, { extra: { "GET /audit-events": problem(500, "boom", "internal_error") } });
  await page.goto("/settings/audit");
  await expect(page.getByRole("alert")).toContainText("The audit trail could not be loaded.");
  await page.unrouteAll();
  await mockSpaces(page, { me: { ...me, user_type: "regular", capabilities: { manage_users: false, create_space: true } } as typeof me });
  await page.goto("/settings/audit");
  await expect(page.getByRole("alert")).toContainText("Only administrators can read the audit trail.");
  await expect(page.getByRole("link", { name: "Security audit" })).toHaveCount(0);
});
