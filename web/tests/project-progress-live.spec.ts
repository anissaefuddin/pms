import { expect, test } from "@playwright/test";

const { PMS_FIXTURE_PROGRESS_SPACE: space, PMS_FIXTURE_PROGRESS_PROJECT: project, PMS_FIXTURE_PROGRESS_TASK: task,
  PMS_FIXTURE_PROGRESS_ADMIN_COOKIE: admin, PMS_FIXTURE_PROGRESS_EDITOR_COOKIE: editor } = process.env;

test.skip(!space || !project || !task || !admin || !editor, "Requires the Go/PostgreSQL project progress fixture");

test("editing a nested task refreshes the Project panel and Space Table in other pages", async ({ page, browser, baseURL }) => {
  await page.context().addCookies([{ name: "pms_session", value: admin!.split("=")[1], url: baseURL! }]);
  const overview = await page.context().newPage();
  const editorContext = await browser.newContext({ baseURL });
  try {
    await editorContext.addCookies([{ name: "pms_session", value: editor!.split("=")[1], url: baseURL! }]);
    const taskPage = await editorContext.newPage();
    await Promise.all([page.goto(`/tasks/${project}`), overview.goto(`/spaces/${space}`), taskPage.goto(`/tasks/${task}`)]);
    const panel = page.getByRole("region", { name: "Project progress" });
    const table = overview.getByRole("table", { name: "Table" });
    const row = table.getByRole("row").filter({ has: overview.getByRole("button", { name: "Progress roadmap", exact: true }) });
    await expect(panel.getByText("0%")).toBeVisible();
    await expect(row.getByText("Green risk")).toBeVisible();
    await expect(page.getByText("Live", { exact: true })).toBeVisible();
    await expect(overview.getByText("Live", { exact: true })).toBeVisible();
    const method = panel.getByRole("combobox", { name: "Progress method" });
    const savedMethod = page.waitForResponse((res) => res.url().endsWith(`/api/v1/items/${project}`) && res.request().method() === "PATCH");
    await method.selectOption("average_percent");
    expect((await savedMethod).ok()).toBe(true);
    await expect(method).toBeEnabled();

    const taskPanel = taskPage.getByRole("region", { name: "Task progress" });
    await taskPanel.getByRole("spinbutton", { name: "Manual progress (%)" }).fill("60");
    await taskPanel.getByRole("button", { name: "Save progress" }).click();
    await expect(panel.getByText("60%")).toBeVisible({ timeout: 3000 });
    await expect(row.getByText("60%")).toBeVisible({ timeout: 3000 });
  } finally {
    await editorContext.close();
    await overview.close();
  }
});
