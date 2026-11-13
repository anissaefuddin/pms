import { expect, test } from "@playwright/test";

const { PMS_M4_PROJECT: project, PMS_M4_ADMIN_COOKIE: admin, PMS_M4_MEMBER_COOKIE: member } = process.env;
test.skip(!project || !admin || !member, "Requires Go/PostgreSQL M4 fixture");

test("timesheet hours appear in project Timelog and alongside its budget", async ({ page, browser, baseURL }) => {
  const worker = await browser.newContext({ baseURL });
  try {
    await worker.addCookies([{ name: "pms_session", value: member!.split("=")[1], url: baseURL! }]);
    const sheet = await worker.newPage();
    await sheet.goto("/timesheets");
    await sheet.getByRole("searchbox", { name: "Find a task to pin" }).fill("M4 delivery task");
    await sheet.getByRole("button", { name: "Pin M4 delivery task", exact: true }).click();
    await sheet.getByRole("textbox", { name: /Duration for M4 delivery task/ }).first().fill("2:15");
    await sheet.getByRole("textbox", { name: /Comment for M4 delivery task/ }).first().fill("Delivered");
    await sheet.getByRole("button", { name: "Save", exact: true }).click();
    await expect(sheet.getByRole("cell", { name: "2:15", exact: true }).last()).toBeVisible();

    await page.context().addCookies([{ name: "pms_session", value: admin!.split("=")[1], url: baseURL! }]);
    await page.goto(`/tasks/${project}`);
    const fields = page.getByRole("region", { name: "Custom fields" });
    await expect(fields.getByText("Actual hours: 2:15")).toBeVisible();
    await fields.getByRole("button", { name: "Edit Budget" }).click();
    const budget = fields.getByRole("form", { name: "Budget" });
    await budget.getByRole("textbox", { name: "Budget" }).fill("1250.75");
    await budget.getByRole("button", { name: "Save", exact: true }).click();
    await expect(fields.getByText("$1,250.75")).toBeVisible();

    await page.getByRole("button", { name: "Add view" }).click();
    await page.getByRole("menuitem", { name: "Timelog" }).click();
    await page.getByRole("dialog", { name: "Add view" }).getByRole("button", { name: "Save" }).click();
    const log = page.getByRole("table", { name: "Timelog" });
    await expect(log.getByText("Delivered")).toBeVisible();
    await expect(log.getByRole("cell", { name: "2:15", exact: true })).toBeVisible();
    await page.reload();
    await expect(log.getByText("Delivered")).toBeVisible();
    await expect(fields.getByText("$1,250.75")).toBeVisible();
  } finally { await worker.close(); }
});
