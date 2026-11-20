import { expect, test, type Page } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// Batch 3A: Calendar create. A "+" on a working day of the project Calendar View opens the dialog with that day as start and
// finish. Weekends and holidays have no control. Mocked API; the server stores the POST as usual.
const calId = "77777777-7777-4777-8777-777777777771";
const path = `/spaces/${marketing.id}`;
const now = new Date();
const y = now.getFullYear();
const m = now.getMonth();
const ym = `${y}-${String(m + 1).padStart(2, "0")}`;
const base = { version: 1, my_capabilities: ALL, assignees: [], locations: [{ space_id: marketing.id }], type: "task" };
const holiday = 11; // "Founders Day" in the work-schedule mock

function setup(page: Page) {
  let items: any[] = [];
  return mockSpaces(page, { extra: {
    [`GET ${path}/views`]: { body: { items: [{ id: calId, type: "calendar", name: "Calendar", visibility: "public", position: 0, is_default: true, config: {} }] } },
    [`GET ${path}/items`]: () => ({ body: { items } }),
    "GET /workflows": { body: { items: [] } },
    "GET /settings/work-schedule": { body: { working_days: [1, 2, 3, 4, 5], daily_hours: 8, version: 1, exceptions: [{ date: `${ym}-${holiday}`, name: "Founders Day" }] } },
    "POST /items": (body: any) => {
      const created = { ...base, id: "e0000000-0000-4000-8000-000000000001", seq: 1, title: body.title, start_date: body.start_date, due_date: body.due_date, locations: body.locations };
      items = [...items, created];
      return { status: 201, body: created };
    },
  } });
}

const days = () => Array.from({ length: new Date(y, m + 1, 0).getDate() }, (_, i) => i + 1);
const weekday = () => days().find((d) => d > 14 && d !== holiday && [1, 2, 3, 4, 5].includes(new Date(y, m, d).getDay()))!;
const weekend = () => days().find((d) => [0, 6].includes(new Date(y, m, d).getDay()))!;
const cell = (page: Page, day: number) => page.getByRole("gridcell").filter({ hasText: new Date(y, m, day).toLocaleDateString("en-US", { dateStyle: "full" }) });

test("Calendar: the + on a working day opens the dialog with that day, and the Task is created on it", async ({ page }) => {
  const { calls } = await setup(page);
  await page.goto(path);
  const day = weekday();
  await cell(page, day).getByRole("button", { name: /^New task on/ }).click();
  const dialog = page.getByRole("dialog", { name: "Create item" });
  await dialog.getByLabel("Name").fill("Calendar task");
  await dialog.getByRole("button", { name: "Create item" }).click();
  const expected = `${ym}-${String(day).padStart(2, "0")}`;
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === "/items")?.body).toMatchObject({
    title: "Calendar task", start_date: expected, due_date: expected,
  });
});

test("Calendar: weekends and holidays have no create control", async ({ page }) => {
  await setup(page);
  await page.goto(path);
  await expect(cell(page, weekend()).getByRole("button", { name: /^New task on/ })).toHaveCount(0);
  await expect(cell(page, holiday).getByRole("button", { name: /^New task on/ })).toHaveCount(0);
});
