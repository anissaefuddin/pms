import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const e = process.env;
test.skip(!e.PMS_FIXTURE_M11A_ADMIN, "Requires the local Go/PostgreSQL M11 analytics fixture");

test("S11.2: a Space analytics board shows real numbers, and its public link works without an account", async ({ browser }) => {
  const baseURL = test.info().project.use.baseURL!;
  const context = await browser.newContext({ baseURL, colorScheme: "light" });
  await context.addCookies([{ name: "pms_session", value: e.PMS_FIXTURE_M11A_ADMIN!.split("=")[1], url: baseURL }]);
  const page = await context.newPage();
  await page.goto(`/spaces/${e.PMS_FIXTURE_M11A_SPACE!}`); // S-UIP.1.8: the board is created from the Space's Tools in the sidebar
  const create = page.getByRole("button", { name: "Create analytics board" });
  if (!(await create.isVisible())) await page.getByRole("button", { name: "Expand M11 Analytics" }).click();
  await create.click();
  await expect(page.getByRole("heading", { name: "Analytics", level: 1 })).toBeVisible();
  const done = page.getByRole("region", { name: "Tasks done" });
  await expect(done.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "33.3"); // 1 of 3 tasks is completed
  const pivot = page.getByRole("region", { name: "Importance by status" });
  await expect(pivot.getByRole("row", { name: /High/ })).toContainText("3"); // 2 active + 1 completed
  await expect(page.getByRole("region", { name: "Burndown, last 30 days" }).locator('[data-chart="burndown"] svg')).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  await page.getByRole("button", { name: "Layout" }).click(); // S-UIP.2.4: layout popover and Share dialog
  await page.getByRole("button", { name: "Vertical" }).click();
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Share" }).click();
  const dialog = page.getByRole("dialog", { name: "Public link" });
  await dialog.getByLabel("Password (optional)").fill("correct horse battery");
  await dialog.getByRole("button", { name: "Turn on" }).click();
  const link = await dialog.getByLabel("Public link address").inputValue();

  const guest = await (await browser.newContext({ baseURL, colorScheme: "light" })).newPage();
  await guest.goto(new URL(link).pathname);
    await expect(guest.getByRole("heading", { name: "This dashboard needs a password" })).toBeVisible();
  await guest.getByLabel("Password (optional)").fill("correct horse battery");
  await guest.getByRole("button", { name: "Open dashboard" }).click();
  await expect(guest.getByRole("heading", { name: "Analytics", level: 1 })).toBeVisible();
  await expect(guest.getByRole("region", { name: "Tasks done" }).getByRole("progressbar")).toHaveAttribute("aria-valuenow", "33.3");
  expect((await new AxeBuilder({ page: guest }).analyze()).violations).toEqual([]);

  await dialog.getByRole("button", { name: "Turn off" }).click();
  await expect(dialog.getByRole("status").first()).toContainText("The link is off.");
  await guest.reload();
  await expect(guest.getByRole("heading", { name: "This dashboard is unavailable" })).toBeVisible();
});
