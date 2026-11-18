import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";

const report = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";

test("the import summary opens its report through the typed reference, not the snippet", async ({ page }) => {
  const summary = {
    id: "n-summary", kind: "import_summary", created_at: new Date().toISOString(), read_at: null,
    actor: { id: "u1", name: "Ada" }, recipient: { id: "u1", name: "Ada" }, item_id: null, item_title: null,
    space_id: null, space_title: null, snippet: "Imported 2 items.", // no report id in the text on purpose
    reference: { kind: "excel_import_report", id: report },
  };
  const { calls } = await mockSpaces(page, { extra: {
    "GET /notifications": { body: { items: [summary], next_cursor: null, unread_count: 1 } },
    [`GET /excel-imports/${report}`]: { body: { created: 2, dependencies: 1, rescheduled: [{ row: 3, key: "S1", start: "2026-11-18" }] } },
  } });
  await page.goto("/inbox");
  await page.getByText("Imported 2 items.").click();
  await expect(page.getByText("Items created")).toBeVisible();
  await expect(page.getByText("Rows rescheduled")).toBeVisible();
  expect(calls.some((call) => call.path === `/excel-imports/${report}`)).toBe(true);
});

test("a summary without a reference shows no report and asks for nothing", async ({ page }) => {
  const legacy = {
    id: "n-old", kind: "import_summary", created_at: new Date().toISOString(), read_at: null,
    actor: { id: "u1", name: "Ada" }, recipient: { id: "u1", name: "Ada" }, item_id: null, item_title: null,
    space_id: null, space_title: null, snippet: "Imported 1 items. Report " + report, reference: null,
  };
  const { calls } = await mockSpaces(page, { extra: {
    "GET /notifications": { body: { items: [legacy], next_cursor: null, unread_count: 0 } },
  } });
  await page.goto("/inbox");
  await page.getByText("Imported 1 items.").click();
  await expect(page.getByText("Items created")).toHaveCount(0);
  expect(calls.some((call) => call.path.startsWith("/excel-imports/"))).toBe(false);
});
