import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";
test("an item the user cannot open shows an alert, not an endless loading state", async ({ page }) => {
  await mockSpaces(page, { extra: { "GET /items/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa": { status: 404, body: { title: "Not found", status: 404 } } } });
  await page.goto("/tasks/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa");
  await expect(page.getByRole("alert")).toBeVisible();
});
