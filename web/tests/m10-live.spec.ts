import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const e = process.env;
test.skip(!e.PMS_FIXTURE_M10_ADMIN, "Requires the local Go/PostgreSQL M10 fixture");

test("E2E-PAR-08: a saved baseline stays put while the plan slips, and the variance shows", async ({ browser }) => {
  const baseURL = test.info().project.use.baseURL!;
  const context = await browser.newContext({ baseURL, colorScheme: "light" });
  await context.addCookies([{ name: "pms_session", value: e.PMS_FIXTURE_M10_ADMIN!.split("=")[1], url: baseURL }]);
  const page = await context.newPage();
  await page.goto(`/tasks/${e.PMS_FIXTURE_M10_PROJECT}`);
  await page.getByRole("radio", { name: "Plan", exact: true }).click();
  await page.getByRole("button", { name: "Save baseline" }).click();
  await page.getByLabel("Baseline name").fill("Kickoff");
  await page.getByRole("button", { name: "Create baseline" }).click();
  await expect(page.getByRole("combobox", { name: "Baseline" })).toHaveValue(/.+/);

  // The plan slips by a week; the baseline must not follow it.
  const moved = await page.request.patch(`/api/v1/items/${e.PMS_FIXTURE_M10_TASK}`, { data: { start_date: "2026-11-09", due_date: "2026-11-11" }, headers: { "If-Match": '"1"' } });
  expect(moved.status()).toBe(200);
  await page.reload();
  await page.getByRole("radio", { name: "Plan", exact: true }).click();
  await expect(page.locator(".wx-bar").first()).toBeVisible();
  await page.getByRole("combobox", { name: "Baseline" }).selectOption({ label: "Kickoff" });
  await expect(page.getByRole("gridcell", { name: "+7 d" })).toBeVisible();
  await expect(page.getByRole("gridcell", { name: "2026-11-02 – 2026-11-04" })).toBeVisible(); // the frozen dates, not the moved ones
  const list = (await (await page.request.get(`/api/v1/items/${e.PMS_FIXTURE_M10_PROJECT}/baselines`)).json()).items as { name: string; item_count: number }[];
  expect(list.map((b) => b.name)).toEqual(["Kickoff"]);
  expect(list[0].item_count).toBe(1);
  expect((await new AxeBuilder({ page }).include(".gantt-chart").analyze()).violations).toEqual([]);
});

test("SPC-16: a public snapshot opens without an account, stays frozen, and stops working once revoked", async ({ browser }) => {
  const baseURL = test.info().project.use.baseURL!;
  const context = await browser.newContext({ baseURL, colorScheme: "light" });
  await context.addCookies([{ name: "pms_session", value: e.PMS_FIXTURE_M10_ADMIN!.split("=")[1], url: baseURL }]);
  const page = await context.newPage();
  await page.goto(`/tasks/${e.PMS_FIXTURE_M10_PROJECT}`);
  await page.getByRole("radio", { name: "Plan", exact: true }).click();
  await page.getByRole("button", { name: "Share snapshot" }).click();
  const dialog = page.getByRole("dialog", { name: "Share a Gantt snapshot" });
  await dialog.getByLabel("Snapshot name").fill("For the client");
  await dialog.getByRole("button", { name: "Create link" }).click();
  const link = await dialog.getByLabel("Snapshot link").inputValue();
  expect(link).toMatch(/\/g\/[\w-]{43}$/);
  await expect(dialog).toHaveCSS("opacity", "1");
  expect((await new AxeBuilder({ page }).include('[role="dialog"]').analyze()).violations).toEqual([]);

  const guest = await (await browser.newContext({ baseURL, colorScheme: "light" })).newPage(); // no cookie
  await guest.goto(new URL(link).pathname);
  await expect(guest.getByRole("heading", { name: "Launch" })).toBeVisible();
  await expect(guest.getByText("Design").first()).toBeVisible();
  expect((await new AxeBuilder({ page: guest }).analyze()).violations).toEqual([]);

  await dialog.getByRole("button", { name: "Revoke For the client" }).click();
  await expect(dialog.getByText("No snapshots yet.")).toBeVisible();
  await guest.reload();
  await expect(guest.getByRole("heading", { name: "This snapshot is unavailable" })).toBeVisible();
});
