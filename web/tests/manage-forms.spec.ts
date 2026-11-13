import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { me, problem } from "./mock-api";
import { marketing, mockSpaces, ops } from "./spaces-mock";

const ada = { id: me.id, name: "Ada Lovelace" };
const f1 = "f0000000-0000-4000-8000-000000000001";
const f2 = "f0000000-0000-4000-8000-000000000002";
const row = (id: string, space: typeof marketing, name: string, access: string) => ({
  id, space_id: space.id, space_name: space.name, name, description: "", access, question_count: 2, updated_by: ada, updated_at: "2026-11-02T10:00:00Z" });

async function mock(page: Page, opts: { items?: any[]; access?: any } = {}) {
  const items = opts.items ?? [row(f1, marketing, "IT request", "private"), row(f2, ops, "Leave request", "internal")];
  const { calls } = await mockSpaces(page, { extra: {
    "GET /forms": () => ({ body: { items } }),
    [`PUT /forms/${f1}/access`]: opts.access ?? ((body: any) => { items[0] = { ...items[0], access: body.access }; return { body: items[0] }; }),
    [`DELETE /forms/${f2}`]: () => { items.splice(1, 1); return { status: 204 }; },
  } });
  return calls;
}

test("table shows columns, link only for internal forms, and filters by name", async ({ page }) => {
  await mock(page);
  await page.goto("/forms");
  await expect(page.getByRole("columnheader")).toHaveText(["Name", "Belongs to", "Access", "Link", "Last modified by", "Actions"]);
  const it = page.getByRole("row", { name: /IT request/ });
  await expect(it.getByRole("link", { name: "IT request" })).toHaveAttribute("href", `/spaces/${marketing.id}/forms/${f1}`);
  await expect(it.getByRole("link", { name: "Marketing" })).toBeVisible();
  await expect(it).toContainText("Only form managers");
  await expect(it).toContainText("Ada Lovelace");
  const leave = page.getByRole("row", { name: /Leave request/ });
  await expect(leave).toContainText("Everyone in the workspace");
  await expect(leave.getByRole("link", { name: "Open form" })).toHaveAttribute("href", new RegExp(`/requests/${f2}$`));
  await page.getByRole("searchbox", { name: "Filter by name" }).fill("leave");
  await expect(it).toHaveCount(0);
  await page.getByRole("searchbox", { name: "Filter by name" }).fill("zzz");
  await expect(page.getByText("No forms match this filter.")).toBeVisible();
});

test("changing access to internal sends the PUT and shows a copyable link", async ({ page, context }) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  const calls = await mock(page);
  await page.goto("/forms");
  await page.getByRole("button", { name: "Actions for IT request" }).click();
  await page.getByRole("menuitem", { name: "Share / Change access" }).click();
  await page.getByRole("radio", { name: /Everyone in the workspace/ }).check();
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByText("Form access updated")).toBeVisible();
  expect(calls.find((c) => c.method === "PUT")!.body).toEqual({ access: "internal" });
  const it = page.getByRole("row", { name: /IT request/ });
  await expect(it.getByRole("link", { name: "Open form" })).toHaveAttribute("href", new RegExp(`/requests/${f1}$`));
  await it.getByRole("button", { name: "Copy link for IT request" }).press("Enter");
  await expect(page.getByText("Link copied")).toBeVisible();
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(`${new URL(page.url()).origin}/requests/${f1}`);
});

test("403 from access shows the friendly message", async ({ page }) => {
  await mock(page, { access: problem(403, "forbidden", "forbidden") });
  await page.goto("/forms");
  await page.getByRole("button", { name: "Actions for IT request" }).click();
  await page.getByRole("menuitem", { name: "Share / Change access" }).click();
  await page.getByRole("radio", { name: /Everyone in the workspace/ }).check();
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("dialog")).toContainText("Only Space owners can change who can fill this form");
});

test("delete asks for confirmation and removes the row", async ({ page }) => {
  const calls = await mock(page);
  await page.goto("/forms");
  await page.getByRole("button", { name: "Actions for Leave request" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await page.getByRole("button", { name: "Delete form" }).click();
  await expect(page.getByText("Form deleted")).toBeVisible();
  expect(calls.some((c) => c.method === "DELETE" && c.path.endsWith(`/forms/${f2}`))).toBe(true);
  await expect(page.getByRole("row", { name: /Leave request/ })).toHaveCount(0);
});

test("empty state", async ({ page }) => {
  await mock(page, { items: [] });
  await page.goto("/forms");
  await expect(page.getByRole("heading", { name: "No forms to manage" })).toBeVisible();
});

for (const scheme of ["light", "dark"] as const) {
  test(`no axe violations (${scheme})`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: scheme });
    await mock(page);
    await page.goto("/forms");
    await expect(page.getByRole("row", { name: /IT request/ })).toBeVisible();
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
    await page.getByRole("button", { name: "Actions for IT request" }).click();
    await page.getByRole("menuitem", { name: "Share / Change access" }).click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await expect(page.locator('[data-slot="dropdown-menu-content"], [data-starting-style]')).toHaveCount(0); // animations settled
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  });
}
