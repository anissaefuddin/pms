import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

const taskId = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const entry = (over: Record<string, unknown>) => ({
  id: "1", kind: "activity", action: "update", changes: { title: {} }, snippet: null, auto: false,
  created_at: "2026-11-10T09:00:00Z", actor: { id: "u1", name: "Grace Hopper" },
  item: { id: taskId, title: "Prepare brief", type: "task" }, ...over,
});

test("sidebar entries can be hidden per user and the choice is saved on the profile", async ({ page }) => {
  let hidden: string[] = [];
  const { calls } = await mockSpaces(page, { extra: {
    "GET /me": () => ({ body: { ...me, sidebar_hidden: hidden } }),
    "PATCH /me": (body) => { hidden = body.sidebar_hidden; return { body: { ...me, sidebar_hidden: hidden } }; },
  } });
  await page.goto("/settings/profile");
  const nav = page.getByRole("navigation", { name: "Main navigation" });
  await expect(nav.getByRole("button", { name: "Stream" })).toBeVisible();
  await expect(nav.getByRole("button", { name: "Request forms" })).toBeVisible();
  await page.getByRole("checkbox", { name: "Stream" }).uncheck();
  await expect(nav.getByRole("button", { name: "Stream" })).toHaveCount(0);
  await page.getByRole("checkbox", { name: "My to-do" }).uncheck();
  await nav.getByRole("button", { name: "More" }).click();
  await expect(page.getByRole("menuitem", { name: "My to-do" })).toHaveCount(0);
  await expect(page.getByRole("menuitem", { name: "Recycle bin" })).toBeVisible();
  expect(calls.filter((call) => call.method === "PATCH" && call.path === "/me").map((call) => call.body))
    .toEqual([{ sidebar_hidden: ["stream"] }, { sidebar_hidden: ["stream", "my_todo"] }]);
  await expect(nav.getByRole("button", { name: "Search" })).toBeVisible();
  await expect(nav.getByRole("button", { name: "Inbox" })).toBeVisible();
});

test("Stream filters, hides auto updates, pages with a cursor and has no axe violations", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: {
    "GET /stream": (_, req) => req.query.get("cursor")
      ? { body: { items: [entry({ id: "3", kind: "comment", action: "comment", changes: {}, snippet: "Looks good" })], next_cursor: null } }
      : { body: { items: [entry({ id: "2" }), entry({ id: "4", action: "delete", changes: {} })], next_cursor: "page-2" } },
  } });
  await page.goto("/");
  await page.getByRole("navigation", { name: "Main navigation" }).getByRole("button", { name: "Stream" }).click();
  await expect(page.getByRole("heading", { name: "Stream" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Prepare brief" }).first()).toHaveAttribute("href", `/tasks/${taskId}`);
  await expect(page.getByText("Grace Hopper updated").first()).toBeVisible();
  await expect(page.getByText("Changed: title")).toBeVisible();
  await page.getByRole("button", { name: "Load more" }).click();
  await expect(page.getByText("Looks good")).toBeVisible();
  expect(calls.some((call) => call.path === "/stream" && call.query.get("cursor") === "page-2")).toBe(true);

  await page.getByRole("button", { name: "Assigned to me" }).click();
  await page.getByRole("checkbox", { name: "Hide auto updates" }).check();
  await expect(page).toHaveURL(/filter=assigned&hideAuto=true/);
  await expect.poll(() => calls.some((call) => call.path === "/stream"
    && call.query.get("filter") === "assigned" && call.query.get("hide_auto") === "true")).toBe(true);
  const result = await new AxeBuilder({ page }).analyze();
  expect(result.violations).toEqual([]);
});

test("full search page applies presets, extra filters and sort, and has no axe violations", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: {
    "GET /search": { body: {
      items: [{ id: taskId, type: "task", title: "Roadmap checklist" }, { id: "p1", type: "project", title: "Roadmap plan" }],
      comments: [{ comment_id: "c1", item_id: taskId, item_title: "Checklist", snippet: "Review the roadmap" }],
      users: [],
    } },
  } });
  await page.goto("/search?q=roadmap");
  await expect(page.getByRole("link", { name: "Roadmap checklist" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Roadmap plan" })).toBeVisible();
  await page.getByRole("button", { name: "Projects & folders", exact: true }).click();
  await expect(page.getByRole("link", { name: "Roadmap plan" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Roadmap checklist" })).toHaveCount(0);
  await page.getByRole("button", { name: "All", exact: true }).click();
  await page.getByRole("checkbox", { name: "Assigned to me" }).check();
  await page.getByLabel("Status").selectOption("active");
  await page.getByLabel("Sort by").selectOption("date_modified");
  await expect.poll(() => calls.some((call) => call.path === "/search" && call.query.get("assigned_to_me") === "true"
    && call.query.get("status_group") === "active" && call.query.get("sort") === "date_modified")).toBe(true);
  const result = await new AxeBuilder({ page }).analyze();
  expect(result.violations).toEqual([]);
});
