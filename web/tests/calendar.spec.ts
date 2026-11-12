import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const calId = "77777777-7777-4777-8777-777777777771";
const path = `/spaces/${marketing.id}`;
const now = new Date();
const ym = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
const base = { version: 1, my_capabilities: ALL, assignees: [], locations: [{ space_id: marketing.id }], type: "task" };
const span = { ...base, id: "d0000000-0000-4000-8000-000000000001", seq: 1, title: "Launch prep", start_date: `${ym}-10`, due_date: `${ym}-12` };
const loose = { ...base, id: "d0000000-0000-4000-8000-000000000002", seq: 2, title: "Someday" };

function mock(page: import("@playwright/test").Page, space = ALL) {
  let items: any[] = [span, loose];
  return mockSpaces(page, { spaces: [{ ...marketing, my_capabilities: space }], extra: {
    [`GET ${path}/views`]: { body: { items: [{ id: calId, type: "calendar", name: "Calendar", visibility: "public", position: 0, is_default: true, config: {} }] } },
    [`GET ${path}/items`]: () => ({ body: { items } }),
    "GET /workflows": { body: { items: [] } },
    "GET /settings/work-schedule": { body: { working_days: [1, 2, 3, 4, 5], daily_hours: 8, version: 1, exceptions: [{ date: `${ym}-11`, name: "Founders Day" }] } },
    ...Object.fromEntries([span, loose].map((item) => [`PATCH /items/${item.id}`, (body: any) => {
      const next = { ...items.find((entry) => entry.id === item.id), ...body, version: 2 };
      items = items.map((entry) => entry.id === item.id ? next : entry);
      return { body: next };
    }])),
  } });
}
const cell = (page: import("@playwright/test").Page, day: number) => page.getByRole("gridcell").filter({ hasText: new Date(now.getFullYear(), now.getMonth(), day).toLocaleDateString("en-US", { dateStyle: "full" }) });

test("shows items on every day they span, holidays, and Unscheduled", async ({ page }) => {
  await mock(page);
  await page.goto(path);
  for (const day of [10, 11, 12]) await expect(cell(page, day).getByRole("button", { name: /Launch prep/ })).toBeVisible();
  await expect(cell(page, 13).getByRole("button")).toHaveCount(0);
  await expect(cell(page, 11)).toContainText("Founders Day");
  await expect(page.getByRole("region", { name: "Unscheduled" }).getByRole("button", { name: "Someday" })).toBeVisible();
  await page.getByRole("button", { name: "Next month" }).click();
  await expect(page.getByRole("button", { name: /Launch prep/ })).toHaveCount(0);
  await page.getByRole("button", { name: "Today" }).click();
  await expect(cell(page, 10).getByRole("button", { name: /Launch prep/ })).toBeVisible();
});

test("dragging moves the whole span, dropping an unscheduled item gives it one day, and keys move by day or week", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto(path);
  await page.setViewportSize({ width: 1400, height: 1200 });
  await cell(page, 11).getByRole("button", { name: /Launch prep/ }).dragTo(cell(page, 17));
  await expect.poll(() => calls.find((call) => call.method === "PATCH" && call.path === `/items/${span.id}`)?.body)
    .toEqual({ start_date: `${ym}-16`, due_date: `${ym}-18` });
  await page.getByRole("region", { name: "Unscheduled" }).getByRole("button", { name: "Someday" }).dragTo(cell(page, 20));
  await expect.poll(() => calls.find((call) => call.method === "PATCH" && call.path === `/items/${loose.id}`)?.body)
    .toEqual({ start_date: `${ym}-20`, due_date: `${ym}-20` });
  await cell(page, 16).getByRole("button", { name: /Launch prep/ }).focus();
  await page.keyboard.press("Alt+ArrowRight");
  await expect.poll(() => calls.filter((call) => call.method === "PATCH" && call.path === `/items/${span.id}`).at(-1)?.body)
    .toEqual({ start_date: `${ym}-17`, due_date: `${ym}-19` });
});

test("view-only users cannot move items; axe is clean", async ({ page }) => {
  const { calls } = await mock(page, ["view"]);
  await page.goto(path);
  await expect(cell(page, 10).getByRole("button", { name: /Launch prep/ })).toHaveAttribute("draggable", "false");
  await cell(page, 10).getByRole("button", { name: /Launch prep/ }).focus();
  await page.keyboard.press("Alt+ArrowRight");
  expect(calls.some((call) => call.method === "PATCH")).toBe(false);
  const found = await new AxeBuilder({ page }).include('[role="grid"]').analyze();
  expect(found.violations).toEqual([]);
});
