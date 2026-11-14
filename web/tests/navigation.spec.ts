import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me, problem } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

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

for (const width of [390, 768]) {
  test(`mobile navigation is keyboard-accessible at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await mockSpaces(page, { spaces: [marketing] });
    await page.goto("/");

    const trigger = page.getByRole("button", { name: "Open navigation" });
    const drawer = page.getByRole("dialog", { name: "Main navigation" });
    await expect(page.getByRole("navigation", { name: "Main navigation" })).toHaveCount(0);
    await expect(trigger).toBeVisible();
    await page.keyboard.press("Tab");
    await expect(trigger).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(drawer).toBeVisible();
    await expect(drawer.getByRole("link", { name: "Marketing" })).toBeVisible();
    await expect.poll(async () => Math.abs((await drawer.boundingBox())?.x ?? width)).toBeLessThan(8);
    expect((await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa"]).analyze()).violations).toEqual([]);
    await page.keyboard.press("Escape");
    await expect(drawer).toBeHidden();
    await expect(trigger).toBeFocused();

    await trigger.press("Enter");
    const space = drawer.getByRole("link", { name: "Marketing" });
    await expect.poll(async () => {
      await space.focus();
      return space.evaluate((link) => link === document.activeElement);
    }).toBe(true);
    await page.keyboard.press("Enter");
    await expect(page.getByRole("heading", { name: "Marketing" })).toBeVisible();
    await expect(drawer).toBeHidden();
    await expect(trigger).toBeFocused();
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(width);

    await trigger.press("Enter");
    await drawer.getByRole("button", { name: "Search", exact: true }).press("Enter");
    await expect(drawer).toBeHidden();
    const search = page.getByRole("dialog", { name: "Search" });
    await expect(search.getByRole("searchbox")).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(search).toBeHidden();

    await trigger.press("Enter");
    await expect(drawer).toBeVisible();
    await page.setViewportSize({ width: 1280, height: 800 });
    await expect(drawer).toBeHidden();
    await expect(trigger).toBeHidden();
    await expect(page.getByRole("button", { name: "Collapse sidebar" })).toBeVisible();
  });
}

test("unknown URLs provide a keyboard path back to the workspace", async ({ page }) => {
  await mockSpaces(page);
  await page.goto("/no-such-page");
  await expect(page.getByRole("heading", { name: "Page not found" })).toBeVisible();
  const back = page.getByRole("link", { name: "Return to workspace" });
  await page.keyboard.press("Tab");
  await expect(back).toBeFocused();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.keyboard.press("Enter");
  // The workspace home forwards to the person's own space when there is one (the old "ready" panel only shows with no spaces).
  await expect(page).toHaveURL(/\/spaces\//);
});

test("workspace headings follow compact and comfortable density", async ({ page }) => {
  await mockSpaces(page, { extra: { "GET /forms": { body: { items: [] } }, "GET /me": { body: { ...me, density: "compact" } } } });
  await page.goto("/forms");
  const heading = page.getByRole("heading", { name: "Manage forms", level: 1 });
  await expect(heading).toHaveCSS("font-size", "20px");
  await page.evaluate(() => { document.documentElement.dataset.density = "comfortable"; });
  await expect(heading).toHaveCSS("font-size", "22px");
});

test("the 404 recovery link sends a signed-out visitor to login", async ({ page }) => {
  await mockSpaces(page, { extra: { "GET /me": problem(401, "No session", "unauthenticated") } });
  await page.goto("/no-such-page");
  await page.getByRole("link", { name: "Return to workspace" }).press("Enter");
  await expect(page.getByRole("heading", { name: "Sign in to your workspace" })).toBeVisible();
});
