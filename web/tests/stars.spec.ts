import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { ALL, extra, marketing, mockSpaces } from "./spaces-mock";
import { problem } from "./mock-api";

const itemId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const first = { id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", name: "New", color: "slate", group: "active" };
const item = { id: itemId, type: "task", title: "Write the plan", seq: 1, version: 1, locations: [{ space_id: marketing.id, parent_item_id: null }], assignees: [], status: first, my_capabilities: ALL };

test("the star on an item toggles with PUT and DELETE and reflects the star list", async ({ page }) => {
  let starred = false;
  const { calls } = await mockSpaces(page, { extra: extra({
    [`GET /spaces/${marketing.id}/items`]: { body: { items: [item] } },
    [`GET /items/${itemId}`]: { headers: { ETag: '"1"' }, body: item },
    [`GET /items/${itemId}/children`]: { body: { items: [] } },
    "GET /stars": () => ({ body: { items: starred ? [{ id: itemId, title: item.title, type: "task" }] : [] } }),
    [`PUT /items/${itemId}/star`]: () => { starred = true; return { status: 204 }; },
    [`DELETE /items/${itemId}/star`]: () => { starred = false; return { status: 204 }; },
  }) });
  await page.goto(`/tasks/${itemId}`);
  const star = page.getByRole("button", { name: "Star this item" });
  await expect(star).toHaveAttribute("aria-pressed", "false");
  await star.click();
  const remove = page.getByRole("button", { name: "Remove the star" });
  await expect(remove).toHaveAttribute("aria-pressed", "true");
  expect(calls.some((c) => c.method === "PUT" && c.path === `/items/${itemId}/star`)).toBe(true);
  await remove.click();
  await expect(page.getByRole("button", { name: "Star this item" })).toHaveAttribute("aria-pressed", "false");
  expect(calls.some((c) => c.method === "DELETE" && c.path === `/items/${itemId}/star`)).toBe(true);
});

test("the Starred page lists items with their type, shows the empty state, and has no axe violations", async ({ page }) => {
  let items: unknown[] = [{ id: itemId, title: "Write the plan", type: "task" }, { id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", title: "Website launch", type: "project" }];
  await mockSpaces(page, { extra: extra({ "GET /stars": () => ({ body: { items } }) }) });
  await page.goto("/starred");
  await expect(page.getByRole("heading", { name: "Starred", level: 1 })).toBeVisible();
  await expect(page.getByRole("link", { name: "Write the plan" })).toHaveAttribute("href", `/items/${itemId}`);
  await expect(page.getByRole("listitem").filter({ hasText: "Website launch" })).toContainText("Project");
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  items = [];
  await page.reload();
  await expect(page.getByText("You have not starred anything yet.")).toBeVisible();
});

test("a failed star shows an error toast instead of flipping the button", async ({ page }) => {
  await mockSpaces(page, { extra: extra({
    [`GET /spaces/${marketing.id}/items`]: { body: { items: [item] } },
    [`GET /items/${itemId}`]: { headers: { ETag: '"1"' }, body: item },
    [`GET /items/${itemId}/children`]: { body: { items: [] } },
    "GET /stars": { body: { items: [] } },
    [`PUT /items/${itemId}/star`]: problem(404, "not found", "not_found"),
  }) });
  await page.goto(`/tasks/${itemId}`);
  await page.getByRole("button", { name: "Star this item" }).click();
  await expect(page.getByText("The star could not be changed.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Star this item" })).toHaveAttribute("aria-pressed", "false");
});
