import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const tableId = "55555555-5555-4555-8555-555555555555";
const calId = "77777777-7777-4777-8777-777777777771";
const path = `/spaces/${marketing.id}`;
const task = { id: "b0000000-0000-4000-8000-000000000001", seq: 1, type: "task", title: "Plan", version: 1, importance: "normal",
  my_capabilities: ALL, locations: [{ space_id: marketing.id }], assignees: [] };

function mock(page: import("@playwright/test").Page) {
  return mockSpaces(page, { extra: {
    [`GET ${path}/views`]: { body: { items: [
      { id: tableId, type: "table", name: "Table", visibility: "public", position: 0, is_default: true, config: {} },
      { id: calId, type: "calendar", name: "Calendar", visibility: "public", position: 1, is_default: false, config: {} },
    ] } },
    [`GET ${path}/items`]: { body: { items: [task] } },
    "GET /workflows": { body: { items: [] } },
    "GET /settings/work-schedule": { body: { working_days: [1, 2, 3, 4, 5], daily_hours: 8, version: 1, exceptions: [] } },
    [`GET /items/${task.id}`]: { body: task, headers: { ETag: '"1"' } },
    [`GET /items/${task.id}/children`]: { body: { items: [] } },
    [`GET /items/${task.id}/comments`]: { body: { items: [] } },
    [`GET /items/${task.id}/attachments`]: { body: { items: [] } },
    [`PATCH /items/${task.id}`]: (body) => ({ body: { ...task, ...body, version: 2 }, headers: { ETag: '"2"' } }),
  } });
}
const listCalls = (calls: Awaited<ReturnType<typeof mock>>["calls"]) => calls.filter((call) => call.method === "GET" && call.path === `${path}/items`);

test("presets and sorts go to the server and the URL, in Table and Calendar", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto(path);
  const sort = page.getByRole("combobox", { name: "Sort" });
  await expect(sort.locator("option")).toHaveText(["Manual order", "Priority", "Finish date", "Status", "Name", "Predecessors", "Start date"]);
  await page.getByRole("combobox", { name: "Preset" }).selectOption("mine");
  await expect.poll(() => listCalls(calls).some((call) => call.query.get("status_group") === "active" && call.query.get("assignee") === "me")).toBe(true);
  await expect(page).toHaveURL(/preset=mine/);
  await page.getByRole("combobox", { name: "Preset" }).selectOption("active");
  await expect.poll(() => listCalls(calls).some((call) => call.query.get("status_group") === "active" && !call.query.get("assignee"))).toBe(true);
  await sort.selectOption("priority");
  await expect.poll(() => listCalls(calls).some((call) => call.query.get("sort") === "priority" && call.query.get("status_group") === "active")).toBe(true);
  await sort.selectOption("predecessors");
  await expect(page).toHaveURL(/sort=predecessors/);
  await page.getByRole("combobox", { name: "Preset" }).selectOption("all");
  await expect.poll(() => listCalls(calls).at(-1)?.query.get("status_group")).toBeNull();
  await page.goto(`${path}?view=${calId}&preset=mine`);
  await expect(page.getByRole("combobox", { name: "Preset" })).toHaveValue("mine");
  await expect.poll(() => listCalls(calls).filter((call) => call.query.get("assignee") === "me").length).toBeGreaterThan(1); // Table above, Calendar here
});

test("the panel edits importance with If-Match", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto(`/tasks/${task.id}`);
  await page.getByLabel("Importance").selectOption("high");
  await expect.poll(() => calls.find((call) => call.method === "PATCH" && call.path === `/items/${task.id}`)?.body).toEqual({ importance: "high" });
});

test("My to-do offers Priority and Importance sorts", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: { "GET /me/todo": { body: { items: [], next_cursor: null } } } });
  await page.goto("/my-todo");
  await expect(page.getByRole("combobox", { name: "Sort tasks" }).locator("option")).toContainText(["Date", "Priority", "Importance"]);
  await page.getByRole("combobox", { name: "Sort tasks" }).selectOption("priority");
  await expect.poll(() => calls.some((call) => call.path === "/me/todo" && call.query.get("sort") === "priority")).toBe(true);
});
