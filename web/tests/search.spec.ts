import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const projectId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const taskId = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";

test("search palette scopes, highlights, supports keyboard and opens the full search page", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /items/${projectId}`]: { body: {
      id: projectId, type: "project", title: "Product launch", locations: [{ space_id: marketing.id }],
      assignees: [], my_capabilities: ALL, description: null,
    }, headers: { ETag: '"1"' } },
    [`GET /items/${projectId}/children`]: { body: { items: [], next_cursor: null } },
    "GET /search": { body: {
      items: [
        { id: taskId, type: "task", title: "Roadmap checklist" },
        { id: projectId, type: "project", title: "Roadmap plan" },
      ],
      comments: [{ comment_id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", item_id: taskId, item_title: "Checklist", snippet: "Review the roadmap" }],
      users: [{ id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", name: "Roadmap Owner" }],
    } },
  } });
  await page.goto(`/items/${projectId}`);
  await expect(page.getByRole("heading", { name: "Product launch" })).toBeVisible();
  await page.keyboard.press("Control+k");
  const dialog = page.getByRole("dialog", { name: "Search" });
  const input = dialog.getByRole("searchbox", { name: "Search query" });
  await expect(input).toBeFocused();
  await expect(dialog.getByText("Enter at least two characters.")).toBeVisible();
  await dialog.getByRole("button", { name: "This project" }).click();
  await input.fill("r");
  await expect(dialog.getByText("Enter at least two characters.")).toBeVisible();
  expect(calls.some((call) => call.path === "/search")).toBe(false);
  await input.fill("roadm");
  await expect(dialog.getByRole("heading", { name: "Tasks" })).toBeVisible();
  await expect(dialog.getByRole("heading", { name: "Projects & folders" })).toBeVisible();
  await expect(dialog.getByRole("heading", { name: "Comments" })).toBeVisible();
  await expect(dialog.getByRole("heading", { name: "Users" })).toBeVisible();
  await expect(dialog.getByRole("link", { name: "Roadmap checklist" }).locator("mark")).toHaveText("Roadmap");
  expect(calls.some((call) => call.path === "/search" && call.query.get("container_id") === projectId
    && call.query.get("space_id") === marketing.id && call.query.get("limit") === "5")).toBe(true);
  await input.press("ArrowUp");
  const showAll = dialog.getByRole("link", { name: "Show all results" });
  await expect(showAll).toBeFocused();
  await showAll.press("Enter");
  await expect(page).toHaveURL(/\/search\?.*q=roadm/);
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("main").getByRole("link", { name: "Roadmap plan" })).toBeVisible();
  await expect.poll(() => calls.some((call) => call.path === "/search" && call.query.get("limit") === "50")).toBe(true);
  await page.getByRole("button", { name: "This space" }).click();
  await expect.poll(() => calls.some((call) => call.path === "/search" && call.query.get("space_id") === marketing.id
    && !call.query.has("container_id"))).toBe(true);
});
