import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const e = process.env;
test.skip(!e.PMS_FIXTURE_MENU_ADMIN, "Requires the local Go/PostgreSQL create-menu fixture");

test("SPC-14: an admin hides Folder from the Space's '+' menu; the dialog stops offering it, the API still creates one", async ({ browser }) => {
  const baseURL = test.info().project.use.baseURL!;
  const context = await browser.newContext({ baseURL, colorScheme: "light" });
  await context.addCookies([{ name: "pms_session", value: e.PMS_FIXTURE_MENU_ADMIN!.split("=")[1], url: baseURL }]);
  const page = await context.newPage();
  await page.goto(`/spaces/${e.PMS_FIXTURE_MENU_SPACE}`);
  await page.getByRole("main").getByRole("button", { name: "Space actions" }).click();
  await page.getByRole("menuitem", { name: "“+” menu" }).click();
  const dialog = page.getByRole("dialog", { name: "“+” menu of this Space" });
  await dialog.getByRole("checkbox", { name: "Folder" }).uncheck();
  await expect(dialog).toHaveCSS("opacity", "1");
  const axe = await new AxeBuilder({ page }).include('[role="dialog"]').analyze();
  expect(axe.violations.map((v) => v.id)).toEqual([]);
  await dialog.getByRole("button", { name: "Save menu" }).click();
  await expect(dialog).toBeHidden();

  await page.getByRole("main").getByRole("button", { name: "New item" }).click();
  const create = page.getByRole("dialog", { name: "Create item" });
  await expect(create.getByRole("radio", { name: /^Project/ })).toBeVisible();
  await expect(create.getByRole("radio", { name: /^Folder/ })).toHaveCount(0);
  const made = await page.request.post("/api/v1/items", { data: { type: "folder", title: "Via API", locations: [{ space_id: e.PMS_FIXTURE_MENU_SPACE }] } });
  expect(made.status()).toBe(201);
  await context.close();
});
