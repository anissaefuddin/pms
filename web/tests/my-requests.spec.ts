import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { problem } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

const itemId = "b0000000-0000-4000-8000-000000000001";
const status = (name: string, color: string, group = "active") => ({ id: `e0000000-0000-4000-8000-00000000000${name.length}`, name, color, group });
const row = (n: number, over: any = {}) => ({ id: `a0000000-0000-4000-8000-00000000000${n}`, form_name: "IT request", item_id: n === 1 ? itemId : `c0000000-0000-4000-8000-00000000000${n}`,
  title: `Laptop ${n}`, status: status("In review", "amber"), created_at: "2026-11-01T10:00:00Z", updated_at: "2026-11-05T10:00:00Z", ...over });

async function setup(page: Page, mine: any, extra: Record<string, any> = {}) {
  return mockSpaces(page, { extra: { "GET /me/requests": mine, "GET /forms/available": { body: { items: [] } }, ...extra } });
}

test("lists requests with a text status badge and no link to the created item", async ({ page }) => {
  await setup(page, { body: { items: [row(1), row(2, { title: "Phone", status: status("Done", "green", "completed") })], next_cursor: null } });
  await page.goto("/requests/mine");
  await page.getByRole("button", { name: /^All/ }).click();
  await expect(page.getByRole("heading", { name: "My requests" })).toBeVisible();
  const first = page.getByRole("listitem").filter({ hasText: "Laptop 1" });
  await expect(first.getByText("In review")).toBeVisible();
  await expect(first.getByText("Form: IT request")).toBeVisible();
  await expect(first.getByText("Submitted 2026-11-01")).toBeVisible();
  await expect(first.getByText("Updated 2026-11-05")).toBeVisible();
  await expect(page.getByRole("listitem").filter({ hasText: "Phone" }).getByText("Done")).toBeVisible();
  await expect(first.getByText("Laptop 1", { exact: true })).toBeVisible(); // RF-08: no link to the created Item
  await expect(page.getByRole("button", { name: "Load more" })).toHaveCount(0);
});

test("Load more sends the cursor", async ({ page }) => {
  const { calls } = await setup(page, (_b: any, req: any) => req.query.get("cursor") === "next-1"
    ? { body: { items: [row(3)], next_cursor: null } } : { body: { items: [row(1), row(2)], next_cursor: "next-1" } });
  await page.goto("/requests/mine");
  await page.getByRole("button", { name: "Load more" }).click();
  await expect(page.getByText("Laptop 3", { exact: true })).toBeVisible();
  await expect(page.getByText("Laptop 1", { exact: true })).toBeVisible();
  const reqs = calls.filter((c) => c.path === "/me/requests");
  expect(reqs[0].query.get("cursor")).toBeNull();
  expect(reqs[1].query.get("cursor")).toBe("next-1");
  await expect(page.getByRole("button", { name: "Load more" })).toHaveCount(0);
});

test("empty state links to the forms list", async ({ page }) => {
  await setup(page, { body: { items: [], next_cursor: null } });
  await page.goto("/requests/mine");
  await expect(page.getByRole("heading", { name: "You have not sent any requests yet" })).toBeVisible();
  await page.getByRole("link", { name: "Browse request forms" }).click();
  await expect(page).toHaveURL("/requests");
});

test("error state retries", async ({ page }) => {
  let fail = true;
  await setup(page, () => fail ? problem(500, "boom") : { body: { items: [row(1)], next_cursor: null } });
  await page.goto("/requests/mine");
  await expect(page.getByRole("alert")).toContainText("Could not load your requests");
  fail = false;
  await page.getByRole("button", { name: "Try again" }).click();
  await expect(page.getByText("Laptop 1", { exact: true })).toBeVisible();
});

test("tabs switch between forms and my requests", async ({ page }) => {
  await setup(page, { body: { items: [], next_cursor: null } });
  await page.goto("/requests");
  const tabs = page.getByRole("navigation", { name: "Requests" });
  await expect(tabs.getByRole("link", { name: "Request forms" })).toHaveAttribute("aria-current", "page");
  await tabs.getByRole("link", { name: "My requests" }).click();
  await expect(page).toHaveURL("/requests/mine");
  await expect(tabs.getByRole("link", { name: "My requests" })).toHaveAttribute("aria-current", "page");
  await expect(tabs.getByRole("link", { name: "Request forms" })).not.toHaveAttribute("aria-current", "page");
});

for (const colorScheme of ["light", "dark"] as const) {
  test.describe(colorScheme, () => {
    test.use({ colorScheme });
    test("my requests list has no axe violations", async ({ page }) => {
      await setup(page, { body: { items: [row(1), row(2, { status: status("Done", "green", "completed") })], next_cursor: "x" } });
      await page.goto("/requests/mine");
      await expect(page.getByText("Laptop 1", { exact: true })).toBeVisible();
      expect((await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze()).violations).toEqual([]);
    });
  });
}

test("Active is the default view; Closed shows the rest and All shows everything", async ({ page }) => {
  await setup(page, { body: { items: [row(1), row(2, { title: "Phone", status: status("Done", "green", "completed") })], next_cursor: null } });
  await page.goto("/requests/mine");
  await expect(page.getByText("Laptop 1", { exact: true })).toBeVisible();
  await expect(page.getByText("Phone", { exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: /^Closed/ }).click();
  await expect(page.getByText("Phone", { exact: true })).toBeVisible();
  await expect(page.getByText("Laptop 1", { exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: /^All/ }).click();
  await expect(page.getByText("Laptop 1", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: /^All/ })).toHaveAttribute("aria-pressed", "true");
});
