import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const cookie = process.env.PMS_FIXTURE_WORKLOAD;
test.skip(!cookie, "requires the PostgreSQL workload fixture");

test("E2E-PAR-07: leave changes capacity and reassign exposes over-allocation", async ({ page, context, baseURL }) => {
  await context.addCookies([{ name: "pms_session", value: cookie!.split("=")[1], url: baseURL! }]);
  await page.goto("/settings/resources");
  await page.getByLabel("Add leave date").fill("2026-11-17");
  await page.getByRole("button", { name: "Save capacity" }).click();
  await expect.poll(async () => (await (await page.request.get("/api/v1/workload?from=2026-11-17&to=2026-11-17")).json()).people.find((p: { name: string }) => p.name === "Capacity Reviewer").capacity).toEqual([0]);
  await page.getByRole("button", { name: "More", exact: true }).click();
  await page.getByRole("menuitem", { name: "Workload" }).click();
  await page.getByLabel("From", { exact: true }).fill("2026-11-17");
  await page.getByLabel("To", { exact: true }).fill("2026-11-17");
  await expect(page.getByText("Review scope", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Reassign" }).click();
  await page.getByRole("searchbox", { name: /Replacement assignees/ }).fill("Capacity");
  await page.getByRole("button", { name: "Capacity Reviewer", exact: true }).click();
  await page.getByRole("button", { name: "Save assignment" }).click();
  await expect(page.getByRole("heading", { name: "Assigned tasks" }).locator("..")).toContainText("Review scope");
  await expect(page.getByRole("table", { name: "Workload and capacity" }).getByRole("row", { name: /Capacity Reviewer/ })).toContainText("0 + 0 / 0");
  await page.goto("/settings/resources");
  await page.getByRole("button", { name: "Remove", exact: true }).click();
  await page.getByRole("button", { name: "Save capacity" }).click();
  await expect.poll(async () => (await (await page.request.get("/api/v1/workload?from=2026-11-17&to=2026-11-17")).json()).people.find((p: { name: string }) => p.name === "Capacity Reviewer").capacity).toEqual([480]);
  await page.goto("/workload");
  await page.getByLabel("From", { exact: true }).fill("2026-11-17");
  await page.getByLabel("To", { exact: true }).fill("2026-11-17");
  await expect(page.getByRole("table", { name: "Workload and capacity" }).getByRole("row", { name: /Capacity Reviewer/ })).toContainText("Over capacity");
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});
