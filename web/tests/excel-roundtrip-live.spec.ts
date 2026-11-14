import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";

const { PMS_EXCEL_PROJECT: project, PMS_EXCEL_SPACE: space, PMS_EXCEL_FIELD: field, PMS_EXCEL_COOKIE: cookie } = process.env;
test.skip(!project || !space || !field || !cookie, "Requires Go/PostgreSQL Excel fixture");

test("exports and re-imports a Project workbook with dates and multi-parent hierarchy", async ({ page, baseURL }) => {
  await page.context().addCookies([{ name: "pms_session", value: cookie!.split("=")[1], url: baseURL! }]);
  await page.goto(`/tasks/${project}`);
  await page.getByRole("button", { name: "Info" }).click(); // S-UIP.1.8
  await page.getByRole("button", { name: "Item actions" }).click();
  await page.getByRole("menuitem", { name: "Export to Excel" }).click();
  const exportDialog = page.getByRole("dialog", { name: "Export to Excel" });
  await exportDialog.getByRole("button", { name: "Start export" }).click();
  const download = page.waitForEvent("download");
  await exportDialog.getByRole("link", { name: "Download workbook" }).click({ timeout: 12_000 });
  const workbook = await download;
  const buffer = await readFile((await workbook.path())!);

  await page.goto(`/spaces/${space}`);
  await page.getByRole("button", { name: "Space actions" }).click();
  await page.getByRole("menuitem", { name: "Import from Excel" }).click();
  const dialog = page.getByRole("dialog", { name: "Import from Excel" });
  await dialog.getByLabel("Excel workbook").setInputFiles({ name: workbook.suggestedFilename(), mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer });
  await dialog.getByRole("button", { name: "Preview import" }).click();
  const result = page.waitForResponse((response) => response.url().endsWith("/excel-imports") && response.status() === 201);
  await dialog.getByRole("button", { name: "Import items" }).click();
  const { item_ids: ids } = await (await result).json();
  expect(ids).toHaveLength(3);
  const item = await (await page.request.get(`/api/v1/items/${ids[2]}`)).json();
  expect(item).toMatchObject({ title: "=2+2", start_date: "2026-11-13", due_date: "2026-11-16", fields: { [field!]: 123.45 } });
  expect(item.locations.map((location: { parent_item_id: string }) => location.parent_item_id).sort()).toEqual(ids.slice(0, 2).sort());
});
