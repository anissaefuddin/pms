import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

const e = process.env;
test.skip(!e.PMS_FIXTURE_M5_ADMIN, "Requires the local Go/PostgreSQL M5 fixture");
test.describe.configure({ mode: "serial" });

let page: Page;
let reportId = "";

test.beforeAll(async ({ browser }) => {
  const baseURL = test.info().project.use.baseURL!;
  const context = await browser.newContext({ baseURL, colorScheme: "light" });
  await context.addCookies([{ name: "pms_session", value: e.PMS_FIXTURE_M5_ADMIN!.split("=")[1], url: baseURL }]);
  page = await context.newPage();
});
test.afterAll(async () => { await page.context().close(); });

async function axe() {
  await expect(page.locator("[data-starting-style], [data-ending-style]")).toHaveCount(0); // animations settled
  const result = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
  expect(result.violations.map((v) => `${v.id}: ${v.nodes[0]?.html}`)).toEqual([]);
}

async function addWidget(name: string, type: string, choose: (dialog: ReturnType<Page["getByRole"]>) => Promise<void> = async () => {}) {
  await page.getByRole("button", { name: "Add widget" }).click();
  await page.getByRole("dialog", { name: "Choose widget" }).getByRole("button", { name: /^Task list/ }).click();
  const dialog = page.getByRole("dialog", { name: "Add widget" });
  await dialog.getByLabel("Widget name").fill(name);
  await dialog.getByLabel(/^Widget type/).selectOption(type);
  await choose(dialog);
  await dialog.getByRole("button", { name: "Save widget" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("region", { name })).toBeVisible();
}

test("flow 1: a project manager builds a dashboard from the UI and the numbers match the data", async () => {
  await page.goto("/dashboards");
  await expect(page.getByText("No dashboards here yet")).toBeVisible();
  await page.getByLabel("Name", { exact: true }).fill("Delivery board");
  await page.getByRole("button", { name: "Create dashboard" }).click();
  await expect(page.getByRole("heading", { name: "Delivery board", level: 1 })).toBeVisible();
  await expect(page.getByText("This dashboard has no widgets yet.")).toBeVisible();

  // a list from a template: the two overdue tasks, and nothing else
  await addWidget("Overdue", "task_list", async (dialog) => { await dialog.getByLabel(/^List or template/).selectOption("template:overdue"); });
  const overdue = page.getByRole("region", { name: "Overdue" });
  await expect(overdue.getByRole("link")).toHaveText(["Overdue one", "Overdue two"]);

  // a pie of all tasks by status group: 4 active, 1 completed
  await addWidget("By status", "pie", async (dialog) => {
    await dialog.getByRole("tab", { name: "Design" }).click(); // S-UIP.2.6: style lives on the Design tab
    await dialog.getByLabel(/^Pie style/).selectOption("donut");
    await expect(dialog.locator('[data-chart="pie"] svg')).toBeVisible(); // live preview
  });
  const status = page.getByRole("region", { name: "By status" });
  await expect(status.locator('[data-chart="pie"] svg')).toBeVisible();
  await status.getByText("Show data as a table").click();
  await expect(status.getByRole("row", { name: /Active/ })).toContainText("4");
  await expect(status.getByRole("row", { name: /Completed/ })).toContainText("1");
  await expect(status.getByText("Total: 5")).toBeVisible();

  // an indicator without grouping
  await addWidget("All tasks", "indicator", async (dialog) => { await dialog.getByLabel(/^Group by/).selectOption(""); });
  await expect(page.getByRole("region", { name: "All tasks" })).toContainText("5");

  // a star on a task shows up in the Starred widget
  await page.goto(`/tasks/${e.PMS_FIXTURE_M5_FRESH}`);
  await page.getByRole("button", { name: "Star this item" }).click();
  await expect(page.getByRole("button", { name: "Remove the star" })).toHaveAttribute("aria-pressed", "true");
  await page.goto("/starred");
  await expect(page.getByRole("main").getByRole("link", { name: "Fresh work" })).toBeVisible();
  await page.goto("/dashboards");
  await page.getByRole("link", { name: "Delivery board" }).click();
  await addWidget("Starred", "task_list", async (dialog) => { await dialog.getByLabel(/^List or template/).selectOption("template:starred"); });
  await expect(page.getByRole("region", { name: "Starred" }).getByRole("link")).toHaveText(["Fresh work"]);
  await axe();

  // everything survives a reload, and a widget can be moved and removed
  await page.reload();
  const cards = page.locator("main").getByRole("region");
  await expect(cards).toHaveCount(4);
  await page.getByRole("button", { name: "Actions for Starred" }).click();
  await page.getByRole("menuitem", { name: "Move earlier" }).click();
  await expect(cards.nth(2)).toHaveAccessibleName("Starred");
  await page.getByRole("button", { name: "Actions for All tasks" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await page.getByRole("button", { name: "Delete widget" }).click();
  // While a modal is open the page behind it is hidden from the accessibility tree, so wait for the dialog first.
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByRole("region", { name: "All tasks" })).toHaveCount(0);
  await expect(cards).toHaveCount(3);

  // rename, duplicate and delete from the list, with confirmation
  await page.goto("/dashboards");
  await page.getByRole("button", { name: "Actions for Delivery board" }).click();
  await page.getByRole("menuitem", { name: "Rename" }).click();
  await page.getByLabel("Name", { exact: true }).last().fill("Delivery board v2");
  await page.getByRole("button", { name: "Rename", exact: true }).click();
  await expect(page.getByRole("link", { name: "Delivery board v2" })).toBeVisible();
  await page.getByRole("button", { name: "Actions for Delivery board v2" }).click();
  await page.getByRole("menuitem", { name: "Duplicate" }).click();
  await page.getByRole("button", { name: "Duplicate", exact: true }).click();
  await expect(page.getByRole("link", { name: "Delivery board v2 (copy)" })).toBeVisible();
  await page.getByRole("button", { name: "Actions for Delivery board v2 (copy)" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await expect(page.getByRole("dialog")).toContainText("Tasks and other data are not affected.");
  await page.getByRole("button", { name: "Delete dashboard" }).click();
  await expect(page.getByRole("link", { name: "Delivery board v2 (copy)" })).toHaveCount(0);
  await expect(page.getByRole("dialog")).toHaveCount(0); // the dialog and its backdrop have faded out before axe measures contrast
  await axe();
  // deleting a dashboard never deletes data
  expect((await (await page.request.get(`/api/v1/items/${e.PMS_FIXTURE_M5_FRESH}`)).json()).title).toBe("Fresh work");
});

test("flow 2: a report is built from a template, edited, saved, exported and subscribed", async () => {
  await page.goto("/reports");
  await page.getByRole("button", { name: "New report", exact: true }).click();
  await page.getByLabel("Start from").selectOption("active_tasks_by_assignee");
  await expect(page.getByLabel("Name", { exact: true })).toHaveValue("Active tasks by assignee");
  await page.getByRole("button", { name: "Create report" }).click();
  await expect(page).toHaveURL(/\/reports\/[0-9a-f-]{36}$/);
  reportId = page.url().split("/").pop()!;
  const rows = page.getByRole("region", { name: "Result" });
  await expect(rows.getByRole("row", { name: /Bob Builder/ })).toContainText("2");
  await expect(rows.getByRole("row", { name: /Ada Admin/ })).toContainText("1");
  await expect(rows.getByRole("row", { name: /Unassigned/ })).toContainText("1");
  await expect(rows.getByText("Total: 4")).toBeVisible();
  await axe();

  // regroup by importance and save
  await page.getByLabel(/^Group by/).selectOption("importance");
  await page.getByRole("button", { name: "Run report" }).click();
  await expect(rows.getByRole("row", { name: /High/ })).toContainText("1");
  await expect(rows.getByRole("row", { name: /Normal/ })).toContainText("3");
  await page.getByLabel("Title").fill("Active tasks by importance");
  await page.getByRole("button", { name: "Save report" }).click();
  await expect(page.getByText("Report saved").first()).toBeVisible();
  await page.reload();
  await expect(page.getByLabel(/^Group by/)).toHaveValue("importance");
  await expect(page.getByRole("heading", { name: "Active tasks by importance", level: 1 })).toBeVisible();

  // CSV export has the same numbers as the screen
  const csv = await (await page.request.get(`/api/v1/reports/${reportId}/export`)).text();
  expect(csv).toBe("importance,count\nhigh,1\nnormal,3\n");

  // subscribe, then see it in the list
  await page.getByLabel(/^Day/).selectOption("1");
  await page.getByLabel(/^Hour/).selectOption("9");
  await page.getByRole("button", { name: "Subscribe", exact: true }).click();
  await expect(page.getByRole("button", { name: "Unsubscribe" })).toBeVisible();
  await page.goto("/reports");
  await expect(page.getByRole("row", { name: /Active tasks by importance/ })).toContainText("Monday at 09:00");

  // utilization: minutes over the month's working time, per person
  await page.getByRole("button", { name: "New report", exact: true }).click();
  await page.getByLabel("Start from").selectOption("utilization");
  await page.getByRole("button", { name: "Create report" }).click();
  const util = page.getByRole("region", { name: "Result" });
  await expect(util.getByRole("row", { name: /Bob Builder/ })).toContainText(e.PMS_FIXTURE_M5_UTIL_BOB!);
  await expect(util.getByRole("row", { name: /Ada Admin/ })).toContainText(e.PMS_FIXTURE_M5_UTIL_ADA!);
  await axe();

  // deleting reports asks first and never touches the tasks
  await page.goto("/reports");
  await page.getByRole("button", { name: "Actions for Utilization this month" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await page.getByRole("button", { name: "Delete report" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByRole("link", { name: "Utilization this month" })).toHaveCount(0);
  expect((await (await page.request.get(`/api/v1/items/${e.PMS_FIXTURE_M5_FRESH}`)).json()).title).toBe("Fresh work");
});
