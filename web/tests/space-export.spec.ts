import { expect, test } from "@playwright/test";
import { marketing, mockSpaces } from "./spaces-mock";

test("Export Space to Excel posts to the Space scope and polls until the download is ready", async ({ page }) => {
  const job = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
  let polls = 0;
  const { calls } = await mockSpaces(page, { extra: {
    [`POST /spaces/${marketing.id}/excel-exports`]: { status: 202, body: { id: job, status: "pending", scope: "space" } },
    [`GET /excel-exports/${job}`]: () => ({ body: { id: job, status: ++polls > 1 ? "ready" : "pending", scope: "space", total_items: 12, processed_items: polls > 1 ? 12 : 5 } }),
  } });
  await page.goto(`/spaces/${marketing.id}`);
  await page.getByRole("button", { name: "Space actions" }).click();
  await page.getByRole("menuitem", { name: "Export Space to Excel" }).click();
  const dialog = page.getByRole("dialog", { name: "Export Space to Excel" });
  await dialog.getByRole("button", { name: "Start export" }).click();
  await expect(dialog.getByRole("link", { name: "Download workbook" })).toHaveAttribute("href", `/api/v1/excel-exports/${job}/file`);
  const post = calls.find((call) => call.method === "POST" && call.path.endsWith("/excel-exports"));
  expect(post?.path).toBe(`/spaces/${marketing.id}/excel-exports`);
});

test("the export dialog shows real item progress while the workbook is built", async ({ page }) => {
  const job = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
  let polls = 0;
  await mockSpaces(page, { extra: {
    [`POST /spaces/${marketing.id}/excel-exports`]: { status: 202, body: { id: job, status: "pending", scope: "space" } },
    [`GET /excel-exports/${job}`]: () => ({ body: { id: job, status: ++polls > 2 ? "ready" : "pending", scope: "space", total_items: 40, processed_items: polls > 2 ? 40 : 10 } }),
  } });
  await page.goto(`/spaces/${marketing.id}`);
  await page.getByRole("button", { name: "Space actions" }).click();
  await page.getByRole("menuitem", { name: "Export Space to Excel" }).click();
  const dialog = page.getByRole("dialog", { name: "Export Space to Excel" });
  await dialog.getByRole("button", { name: "Start export" }).click();
  await expect(dialog.getByText("10 of 40 items written")).toBeVisible();
  await expect(dialog.getByRole("link", { name: "Download workbook" })).toBeVisible({ timeout: 10_000 });
});
