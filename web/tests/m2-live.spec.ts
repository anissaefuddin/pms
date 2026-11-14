import { expect, test, type Page } from "@playwright/test";

const e = process.env;
test.skip(!e.PMS_FIXTURE_M2_COOKIE, "Requires the local Go/PostgreSQL M2 fixture");
test.describe.configure({ mode: "serial" });

async function login(page: Page, baseURL: string | undefined) {
  await page.context().addCookies([{ name: "pms_session", value: e.PMS_FIXTURE_M2_COOKIE!.split("=")[1], url: baseURL! }]);
}
const item = async (page: Page, id: string) => (await (await page.request.get(`/api/v1/items/${id}`)).json());

test("flow 1: a holiday in the work schedule, then moving a predecessor pushes its successor past the holiday", async ({ page, baseURL }) => {
  await login(page, baseURL);
  await page.goto("/settings/work-schedule");
  await page.getByLabel("Holiday date").fill("2027-03-03");
  await page.getByLabel("Holiday name").fill("Fixture holiday");
  await page.getByRole("button", { name: "Add holiday" }).click();
  await page.getByRole("button", { name: "Save work schedule" }).click();
  await expect(page.getByText("Work schedule saved")).toBeVisible();

  await page.goto(`/spaces/${e.PMS_FIXTURE_M2_SPACE}?view=${e.PMS_FIXTURE_M2_GANTT}`);
  const predBar = page.locator(`.wx-bar[data-id=":${e.PMS_FIXTURE_M2_PRED}"]`);
  await predBar.scrollIntoViewIfNeeded();
  await expect(predBar).toBeVisible();
  await expect(page.locator(".wx-line").first()).toBeAttached(); // the link comes from the server
  await page.getByRole("application", { name: "Gantt chart" }).focus();
  const rows = ["Pred task", "Succ task"];
  // Select "Pred task" (rows are ordered by the server). One key press, then wait for the chart to announce the row: pressing again
  // while the announcement is still on its way would move the selection to the next row. Only a press that announced nothing is repeated
  // (a slow runner can press before the chart is ready to take keys).
  const announced = page.getByRole("status").filter({ hasText: new RegExp(rows.join("|")) });
  for (let attempt = 0; attempt < 4; attempt++) {
    await page.keyboard.press("ArrowDown");
    if (await announced.first().waitFor({ timeout: 4000 }).then(() => true, () => false)) break;
  }
  await expect(page.getByRole("status").filter({ hasText: "Pred task" })).toBeVisible();
  await page.keyboard.press("ArrowRight"); // 1 Mar -> 2 Mar
  await expect.poll(async () => (await item(page, e.PMS_FIXTURE_M2_PRED!)).start_date).toBe("2027-03-02");
  // Finish-to-start: the next working day after 2 Mar is 4 Mar because 3 Mar is a holiday.
  await expect.poll(async () => (await item(page, e.PMS_FIXTURE_M2_SUCC!)).start_date).toBe("2027-03-04");
  const bar = async (id: string) => (await page.locator(`.wx-bar[data-id=":${id}"]`).boundingBox())!.x;
  await expect.poll(async () => (await bar(e.PMS_FIXTURE_M2_SUCC!)) - (await bar(e.PMS_FIXTURE_M2_PRED!))).toBeGreaterThan(30);
});

test("flow 2: a custom field is created, filled, and shown in Table, Board, and Gantt", async ({ page, baseURL }) => {
  await login(page, baseURL);
  await page.goto(`/spaces/${e.PMS_FIXTURE_M2_SPACE}`);
  await page.getByRole("button", { name: "Space actions" }).click();
  await page.getByRole("menuitem", { name: "Custom fields" }).click();
  const dialog = page.getByRole("dialog", { name: "Custom fields" });
  await dialog.getByRole("button", { name: "Add custom field" }).click();
  await dialog.getByRole("textbox", { name: "Field name" }).fill("Effort score");
  await dialog.getByRole("combobox").first().selectOption("number");
  await dialog.getByRole("button", { name: "Save", exact: true }).click();
  await expect(dialog.getByRole("listitem").filter({ hasText: "Effort score" })).toBeVisible();
  await page.keyboard.press("Escape");

  await page.goto(`/tasks/${e.PMS_FIXTURE_M2_FIELDS_TASK}`);
  const section = page.getByRole("region", { name: "Custom fields" });
  await section.getByRole("button", { name: "Edit Effort score" }).click();
  await section.getByRole("form", { name: "Effort score" }).getByRole("textbox").fill("42");
  await section.getByRole("form", { name: "Effort score" }).getByRole("button", { name: "Save" }).click();
  await expect(section.getByText("42")).toBeVisible();

  for (const [view, button] of [[e.PMS_FIXTURE_M2_TABLE, "Columns"], [e.PMS_FIXTURE_M2_BOARD, "Card fields"], [e.PMS_FIXTURE_M2_GANTT, "Columns"]] as const) {
    await page.goto(`/spaces/${e.PMS_FIXTURE_M2_SPACE}?view=${view}`);
    await page.getByRole("button", { name: button }).click();
    await page.getByRole("dialog").getByLabel("Effort score").check();
    await page.getByRole("dialog").getByRole("button", { name: "Save columns" }).click();
  }
  await page.goto(`/spaces/${e.PMS_FIXTURE_M2_SPACE}?view=${e.PMS_FIXTURE_M2_TABLE}`);
  await expect(page.getByRole("table", { name: "Table" }).getByRole("button", { name: "Effort score: Fields task" })).toHaveText("42");
  await page.goto(`/spaces/${e.PMS_FIXTURE_M2_SPACE}?view=${e.PMS_FIXTURE_M2_BOARD}`);
  await expect(page.getByRole("listitem").filter({ hasText: "Fields task" })).toContainText(/Effort score:\s*42/);
  await page.goto(`/spaces/${e.PMS_FIXTURE_M2_SPACE}?view=${e.PMS_FIXTURE_M2_GANTT}`);
  await expect(page.getByRole("columnheader", { name: "Effort score" })).toBeVisible();
  await expect(page.getByRole("gridcell", { name: "42" })).toBeVisible();
});

test("flow 3: Calendar drag changes dates and a swimlane drag changes the assignee after confirmation", async ({ page, baseURL }) => {
  await login(page, baseURL);
  await page.setViewportSize({ width: 1400, height: 2000 });
  await page.goto(`/spaces/${e.PMS_FIXTURE_M2_SPACE}?view=${e.PMS_FIXTURE_M2_CALENDAR}`);
  const [y, m] = e.PMS_FIXTURE_M2_CAL_DAY!.split("-").map(Number);
  const cell = (d: number) => page.getByRole("gridcell").filter({ hasText: new Date(y, m - 1, d).toLocaleDateString("en-US", { dateStyle: "full" }) });
  await cell(10).getByRole("button", { name: /Calendar task/ }).dragTo(cell(17));
  const moved = `${y}-${String(m).padStart(2, "0")}-17`;
  await expect.poll(async () => (await item(page, e.PMS_FIXTURE_M2_CAL_TASK!)).due_date).toBe(moved);
  expect((await item(page, e.PMS_FIXTURE_M2_CAL_TASK!)).start_date).toBe(moved);

  await page.goto(`/spaces/${e.PMS_FIXTURE_M2_SPACE}?view=${e.PMS_FIXTURE_M2_BOARD}&group=assignee`);
  const owner = page.getByRole("region", { name: "Lane Owner", exact: true });
  const otherLane = page.getByRole("region", { name: "Lane Other", exact: true });
  await expect(owner.getByRole("button", { name: "Lane task", exact: true })).toBeVisible();
  await owner.getByRole("listitem").filter({ hasText: "Lane task" }).dragTo(otherLane.getByRole("region").first());
  await page.getByRole("dialog", { name: "Change assignee?" }).getByRole("button", { name: "Change assignee" }).click();
  await expect.poll(async () => (await item(page, e.PMS_FIXTURE_M2_LANE_A!)).assignees.map((a: { id: string }) => a.id)).toEqual([e.PMS_FIXTURE_M2_OTHER]);
});
