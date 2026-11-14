import { expect, test } from "@playwright/test";
import { ALL, extra, marketing, mockSpaces } from "./spaces-mock";

// DR-03: after "Create item" the dialog must stay closed (no empty form), and the new item must be shown.
const id = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaa03";
const status = { id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", name: "New", color: "slate", group: "active" };
const root = `/spaces/${marketing.id}`;

for (const where of ["space page button", "global top bar"]) {
  test(`creating a task from the ${where} closes the dialog and opens the new task`, async ({ page }) => {
    let item: Record<string, unknown> | undefined;
    await mockSpaces(page, { extra: extra({
      [`GET ${root}/items`]: () => ({ body: { items: item ? [item] : [] } }),
      [`GET /items/${id}`]: () => ({ headers: { ETag: '"1"' }, body: item }),
      [`GET /items/${id}/children`]: { body: { items: [] } },
      "POST /items": (body: Record<string, unknown>) => {
        item = { id, seq: 1, version: 1, assignees: [], status, my_capabilities: ALL, ...body };
        return { status: 201, body: item };
      },
    }) });
    await page.goto(root);
    const button = where === "global top bar" ? page.locator("header").first().getByRole("button", { name: "New item" }) : page.getByRole("main").getByRole("button", { name: "New item" });
    await button.click();
    const dialog = page.getByRole("dialog", { name: "Create item" });
    await dialog.getByRole("radio", { name: "Task" }).check();
    await dialog.getByLabel("Name").fill("Fresh task");
    await dialog.getByRole("button", { name: "Create item" }).click();
    await expect(page.getByRole("heading", { name: "Fresh task" })).toBeVisible();
    await expect(page.getByRole("dialog", { name: "Create item" })).toHaveCount(0);
  });
}
