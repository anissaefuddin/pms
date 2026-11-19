import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const project = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const itemPath = `/items/${project}`;
const loc = { space_id: marketing.id, space_name: "Marketing" };
const projectItem = { id: project, type: "project", title: "Roadmap", seq: 1, version: 1, locations: [loc], assignees: [], my_capabilities: ALL,
  status: { id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", name: "New", color: "slate", group: "active" }, created_by: { id: me.id, name: "Ada" } };
const common = {
  [`GET /spaces/${marketing.id}/items`]: { body: { items: [projectItem] } },
  [`GET ${itemPath}`]: { headers: { ETag: '"1"' }, body: projectItem },
  [`GET ${itemPath}/children`]: { body: { items: [] } },
  "GET /me/todo": { body: { items: [projectItem], next_cursor: null } },
};

test("Space node shows overview, Tools and Projects with type icons, walked by arrow keys", async ({ page }) => {
  await mockSpaces(page, { extra: common as never });
  await page.goto(`/spaces/${marketing.id}`);
  const tree = page.getByRole("navigation", { name: "Spaces" });
  const toggle = tree.getByRole("button", { name: "Expand Marketing" });
  await toggle.focus();
  await toggle.press("ArrowRight");
  await expect(tree.getByRole("link", { name: "Space overview" })).toHaveCount(0);
  await expect(tree.getByText("Tools")).toBeVisible();
  await expect(tree.getByRole("button", { name: "Create in Marketing" })).toBeVisible();
  await expect(tree.getByRole("link", { name: "Roadmap" }).locator("svg")).toHaveCount(1);
  await tree.getByRole("button", { name: "Collapse Marketing" }).press("ArrowDown"); // next focusable: the Marketing link
  await expect(tree.getByRole("link", { name: "Marketing" })).toBeFocused();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("a project page is header plus view; details live in the Info panel and /tasks/{id} still works", async ({ page }) => {
  await mockSpaces(page, { extra: { ...common, "GET /spaces/x/views": { body: { items: [] } } } as never });
  await page.goto(`/tasks/${project}`);
  await expect(page.getByRole("heading", { name: "Roadmap", level: 1 })).toBeVisible();
  await expect(page.getByRole("textbox", { name: "Name", exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "Info" }).click();
  await expect(page.getByRole("dialog").getByRole("textbox", { name: "Name", exact: true })).toHaveValue("Roadmap");
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("the account menu is the one place for Settings, Recycle bin, Apps, Help and Log out; Search shows its shortcut", async ({ page }) => {
  await mockSpaces(page);
  await page.goto("/");
  await expect(page.getByRole("button", { name: "Search", exact: false }).first()).toContainText(/⌘K|Ctrl K/);
  await expect(page.getByRole("link", { name: "Settings" })).toHaveCount(0);
  await page.getByRole("button", { name: "Account menu" }).click();
  for (const name of ["Settings", "Recycle bin", "Apps & Integrations", "Help", "Log out"]) await expect(page.getByRole("menuitem", { name })).toBeVisible();
  await page.getByRole("menuitem", { name: "Apps & Integrations" }).click();
  await expect(page).toHaveURL(/\/settings\/apps$/);
});

test("More lists Calendars and Created by me; Calendars and Created by me pages open without axe violations", async ({ page }) => {
  await mockSpaces(page, { extra: common as never });
  await page.goto("/");
  await page.getByRole("button", { name: "More", exact: true }).click();
  await expect(page.getByRole("menuitem", { name: "Customize sidebar" })).toBeVisible();
  await page.getByRole("menuitem", { name: "Calendars" }).click();
  await expect(page.getByRole("heading", { name: "Calendars", level: 1 })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByRole("button", { name: "More", exact: true }).click();
  await page.getByRole("menuitem", { name: "Created by me" }).click();
  await expect(page.getByRole("link", { name: "Roadmap" })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("the realtime status shows only while disconnected", async ({ page }) => {
  await mockSpaces(page);
  await page.goto("/");
  await expect(page.getByText("Live", { exact: true })).toHaveCount(0);
});

test("on a project page a task opens in the slide-right panel, not a new page", async ({ page }) => {
  const taskId = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
  const task = { ...projectItem, id: taskId, type: "task", title: "Draft brief", seq: 2 };
  await mockSpaces(page, { extra: { ...common, "GET /spaces/x/views": { body: { items: [] } },
    [`GET ${itemPath}/children`]: { body: { items: [task] } }, [`GET /items/${taskId}`]: { headers: { ETag: '"1"' }, body: task },
    [`GET /items/${taskId}/children`]: { body: { items: [] } } } as never });
  await page.goto(`/tasks/${project}`);
  await page.getByRole("button", { name: "Draft brief", exact: true }).first().click();
  await expect(page).toHaveURL(new RegExp(`/tasks/${project}\\?.*task=${taskId}`));
  await expect(page.getByRole("dialog", { name: "Item detail" })).toBeVisible();
});
