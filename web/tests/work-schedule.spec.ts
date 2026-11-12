import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me, problem } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

const initial = { working_days: [1, 2, 3, 4, 5], daily_hours: 8, exceptions: [], version: 1 };

test("admin edits the work schedule with If-Match and sees a conflict reload", async ({ page }) => {
  let current: typeof initial = { ...initial };
  let conflict = false;
  const { calls } = await mockSpaces(page, { extra: {
    "GET /settings/work-schedule": () => ({ body: current, headers: { ETag: `"${current.version}"` } }),
    "PUT /settings/work-schedule": (body, req) => {
      if (conflict) { current = { ...current, daily_hours: 4, version: current.version + 1 }; return problem(409, "version conflict", "version_conflict"); }
      expect(req.headers["if-match"]).toBe(`"${current.version}"`);
      current = { ...body, version: current.version + 1 };
      return { body: current, headers: { ETag: `"${current.version}"` } };
    },
  } });
  await page.goto("/settings/work-schedule");
  await page.getByLabel("Saturday").check();
  await page.getByLabel("Friday").uncheck();
  await page.getByLabel("Hours per working day").fill("6");
  await page.getByLabel("Holiday date").fill("2026-12-25");
  await page.getByLabel("Holiday name").fill("Christmas");
  await page.getByRole("button", { name: "Add holiday" }).click();
  await page.getByRole("button", { name: "Save work schedule" }).click();
  await expect(page.getByText("Work schedule saved")).toBeVisible();
  expect(calls.find((call) => call.method === "PUT")?.body).toEqual({
    working_days: [1, 2, 3, 4, 6], daily_hours: 6, exceptions: [{ date: "2026-12-25", name: "Christmas" }] });

  conflict = true;
  await page.getByLabel("Hours per working day").fill("7");
  await page.getByRole("button", { name: "Save work schedule" }).click();
  await expect(page.getByText(/Someone else changed the work schedule/)).toBeVisible();
  await expect(page.getByLabel("Hours per working day")).toHaveValue("4"); // reloaded from the server
});

test("a regular user sees the schedule read-only", async ({ page }) => {
  await mockSpaces(page, { extra: {
    "GET /me": { body: { ...me, user_type: "regular" } },
    "GET /settings/work-schedule": { body: initial, headers: { ETag: '"1"' } },
  } });
  await page.goto("/settings/work-schedule");
  await expect(page.getByText("Only administrators can change the work schedule.")).toBeVisible();
  await expect(page.getByLabel("Hours per working day")).toBeDisabled();
  await expect(page.getByRole("button", { name: "Save work schedule" })).toHaveCount(0);
});

for (const scheme of ["light", "dark"] as const) {
  test(`work schedule has no axe violations (${scheme})`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: scheme });
    await mockSpaces(page, { extra: { "GET /settings/work-schedule": { body: { ...initial, exceptions: [{ date: "2026-12-25", name: "Christmas" }] }, headers: { ETag: '"1"' } } } });
    await page.goto("/settings/work-schedule");
    await expect(page.getByLabel("Hours per working day")).toBeVisible();
    const result = await new AxeBuilder({ page }).analyze();
    expect(result.violations.map((v) => `${v.id}: ${v.nodes[0]?.html}`)).toEqual([]);
  });
}
