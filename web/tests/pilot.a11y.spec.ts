import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { me } from "./mock-api";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const item = {
  id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", title: "Pilot task", type: "task", seq: 1, version: 1,
  my_capabilities: ALL, locations: [{ space_id: marketing.id }], assignees: [],
};
const table = { id: "55555555-5555-4555-8555-555555555555", type: "table", name: "Table", visibility: "public", position: 0, is_default: true };
const board = { id: "66666666-6666-4666-8666-666666666666", type: "board", name: "Board", visibility: "public", position: 1, is_default: false };
const path = `/spaces/${marketing.id}`;

async function check(page: Page) {
  await page.evaluate(() => Promise.all(document.getAnimations().map((animation) => animation.finished.catch(() => undefined))));
  const { violations } = await new AxeBuilder({ page }).analyze();
  expect(violations.filter((violation) => violation.impact === "serious" || violation.impact === "critical")).toEqual([]);
}

for (const colorScheme of ["light", "dark"] as const) {
  test.describe(colorScheme, () => {
    test.use({ colorScheme });
    test("Table, Board, item detail, search, My to-do and Inbox have no serious axe violations", async ({ page }) => {
      await mockSpaces(page, { extra: {
        [`GET ${path}/views`]: { body: { items: [table, board] } },
        [`GET ${path}/items`]: { body: { items: [item] } },
        [`GET /items/${item.id}`]: { body: item, headers: { ETag: '"1"' } },
        [`GET /items/${item.id}/children`]: { body: { items: [] } },
        [`GET /items/${item.id}/comments`]: { body: { items: [] } },
        [`GET /items/${item.id}/attachments`]: { body: { items: [], max_upload_bytes: 1048576 } },
        "GET /workflows": { body: { items: [] } },
        "GET /notifications": { body: { items: [{
          id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", kind: "mention", item_id: item.id,
          item_title: item.title, actor: { id: me.id, name: "Ada Lovelace" }, created_at: "2026-11-12T12:00:00Z",
        }], unread_count: 1 } },
        "GET /me/todo": { body: { items: [item] } },
        "GET /search": { body: { items: [item], comments: [], users: [] } },
        "GET /recycle-bin": { body: { items: [{ ...item, restore_blocked_reason: "space_deleted" }] } },
        "GET /seats": { body: { used: 1, pending: 0, max: 3, available: 2 } },
        "GET /users": { body: { items: [me] } },
        "GET /groups": { body: { items: [] } },
      } });
      await page.goto(path);
      await expect(page.locator("html")).toHaveCSS("color-scheme", colorScheme);
      await expect(page.getByRole("table", { name: "Table" })).toBeVisible();
      await check(page);
      await page.getByRole("button", { name: "Board", exact: true }).click();
      await expect(page.getByRole("region", { name: "No status" })).toBeVisible();
      await check(page);
      await page.getByRole("region", { name: "No status" }).getByRole("button", { name: item.title }).click();
      const panel = page.getByRole("dialog", { name: "Item details" });
      await expect(panel.getByRole("heading", { name: item.title })).toBeVisible();
      await expect(panel.getByRole("region", { name: "Comments" })).toBeVisible();
      await check(page);

      await page.goto("/inbox");
      await expect(page.getByRole("button", { name: /mentioned you in Pilot task/ })).toBeVisible();
      await check(page);
      await page.goto("/my-todo");
      await expect(page.getByRole("link", { name: item.title })).toBeVisible();
      await check(page);
      await page.goto("/search?q=Pilot");
      await expect(page.getByRole("link", { name: item.title })).toBeVisible();
      await check(page);
      await page.goto("/settings/profile");
      await expect(page.getByRole("button", { name: "Save profile" })).toBeVisible();
      await check(page);
      await page.goto("/settings/users");
      await expect(page.getByRole("button", { name: "Invite user" })).toBeVisible();
      await check(page);
      await page.goto("/recycle-bin");
      await expect(page.getByText("The original Space was deleted.")).toBeVisible();
      await check(page);
    });
  });
}
