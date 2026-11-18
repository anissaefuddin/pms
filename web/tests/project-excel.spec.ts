import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces, ops } from "./spaces-mock";

test("exports the selected visible Project location and polls until ready", async ({ page }) => {
  const project = "88888888-8888-4888-8888-888888888888";
  const job = "99999999-9999-4999-8999-999999999999";
  let polls = 0;
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /items/${project}`]: { body: { id: project, type: "project", title: "Delivery",
      locations: [{ space_id: marketing.id }, { space_id: ops.id }], my_capabilities: ALL, assignees: [] }, headers: { ETag: '"1"' } },
    [`GET /items/${project}/children`]: { body: { items: [] } },
    [`POST /items/${project}/excel-exports`]: { status: 202, body: { id: job, status: "pending" } },
    [`GET /excel-exports/${job}`]: () => ({ body: { id: job, status: ++polls > 1 ? "ready" : "pending" } }),
  } });
  await page.goto(`/tasks/${project}`);
  await page.getByRole("button", { name: "Info" }).click(); // S-UIP.1.8: project detail lives in the Info panel
  await page.getByRole("button", { name: "Item actions" }).click();
  await page.getByRole("menuitem", { name: "Export to Excel" }).click();
  const dialog = page.getByRole("dialog", { name: "Export to Excel" });
  await dialog.getByRole("combobox", { name: "Space" }).selectOption(ops.id);
  await dialog.getByRole("button", { name: "Start export" }).click();
  await expect(dialog.getByRole("link", { name: "Download workbook" })).toHaveAttribute("href", `/api/v1/excel-exports/${job}/file`);
  expect(calls.find((call) => call.method === "POST" && call.path.endsWith("/excel-exports"))?.body).toEqual({ space_id: ops.id });
});

test("maps an unknown column and imports only the last validated preview", async ({ page }) => {
  await mockSpaces(page);
  const requests: string[] = [];
  await page.route(`**/api/v1/spaces/${marketing.id}/excel-imports`, async (route) => {
    const body = route.request().postData() ?? "";
    requests.push(body);
    const mapped = body.includes('{"Name":"title"}');
    const commit = body.includes('name="commit"');
    await route.fulfill({ json: commit ? { item_ids: ["imported-item"] } : {
      columns: ["Name"], rows: 1, sample: [["Delivery"]],
      errors: mapped ? [] : [{ row: 1, column: "Name", message: "Unknown column" }], digest: mapped ? "checked" : "",
    } });
  });
  await page.goto(`/spaces/${marketing.id}`);
  await page.getByRole("button", { name: "Space actions" }).click();
  await page.getByRole("menuitem", { name: "Import from Excel" }).click();
  const dialog = page.getByRole("dialog", { name: "Import from Excel" });
  await dialog.getByLabel("Excel workbook").setInputFiles({
    name: "items.xlsx", mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer: Buffer.from("fixture"),
  });
  // Step 1 reads the columns from the server; the unknown column is shown in the validation step, not before.
  await dialog.getByRole("button", { name: "Next" }).click();
  await expect(dialog.getByText(/^Step 2 of 4/)).toBeVisible();
  await expect(dialog.getByText(/Unknown column/)).toHaveCount(0);
  await dialog.getByRole("combobox", { name: "Map column Name" }).selectOption("title");
  await dialog.getByRole("button", { name: "Next" }).click();
  await expect(dialog.getByText(/^Step 3 of 4/)).toBeVisible();
  await expect(dialog.getByRole("table", { name: "Workbook preview" }).getByText("Delivery")).toBeVisible();
  await dialog.getByRole("button", { name: "Next" }).click();
  await expect(dialog.getByText(/^Step 4 of 4/)).toBeVisible();
  await dialog.getByRole("button", { name: "Import items" }).click();
  await expect(dialog).toBeHidden();
  expect(requests).toHaveLength(3);
  expect(requests[2]).toContain('name="digest"');
  expect(requests[2]).toContain("checked");
  expect(requests[2]).toContain('{"Name":"title"}');
});
