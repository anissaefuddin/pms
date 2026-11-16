import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

const ada = { id: me.id, name: "Ada Lovelace" };
const f1 = "f0000000-0000-4000-8000-000000000001";

test("admin reaches request forms from Settings, creates one by choosing a Space, and axe is clean", async ({ page }) => {
  const items = [{ id: f1, space_id: marketing.id, space_name: marketing.name, name: "IT request", description: "", access: "private", question_count: 2, updated_by: ada, updated_at: "2026-11-02T10:00:00Z" }];
  await mockSpaces(page, { me: { ...me, user_type: "admin" }, extra: { "GET /forms": () => ({ body: { items } }) } });
  await page.goto("/settings/request-forms");
  await expect(page.getByRole("row", { name: /IT request/ })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  await page.getByRole("button", { name: "Create form" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Space").selectOption({ label: marketing.name });
  await dialog.getByRole("button", { name: "Create form" }).click();
  await expect(page).toHaveURL(new RegExp(`/spaces/${marketing.id}/forms/new$`));
});

test("requesters see request forms as cards", async ({ page }) => {
  await mockSpaces(page, { extra: { "GET /forms/available": () => ({ body: { items: [{ id: f1, name: "IT request", description: "Laptop or access" }] } }) } });
  await page.goto("/requests");
  await expect(page.getByRole("listitem").filter({ hasText: "IT request" }).getByRole("link", { name: "Start IT request" })).toBeVisible();
});
