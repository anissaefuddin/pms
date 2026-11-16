import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// Phase 3.2 sidebar refinement: SB-01 (expanded toggles keep nav colours), SB-02 (thin scrollbar),
// SB-05 (Tools is a collapsible section, open only on a tool page).
const project = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const loc = { space_id: marketing.id, space_name: "Marketing" };
const projectItem = { id: project, type: "project", title: "Roadmap", seq: 1, version: 1, locations: [loc], assignees: [], my_capabilities: ALL,
  status: { id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", name: "New", color: "slate", group: "active" }, created_by: { id: me.id, name: "Ada" } };
const common = {
  [`GET /spaces/${marketing.id}/items`]: { body: { items: [projectItem] } },
  [`GET /items/${project}`]: { headers: { ETag: '"1"' }, body: projectItem },
  [`GET /items/${project}/children`]: { body: { items: [] } },
  "GET /me/todo": { body: { items: [projectItem], next_cursor: null } },
};

test("SB-T09 / SB-05: Tools starts collapsed on a Space page and expands on demand", async ({ page }) => {
  await mockSpaces(page, { extra: common as never });
  await page.goto(`/spaces/${marketing.id}`);
  const tree = page.getByRole("navigation", { name: "Spaces" });
  await tree.getByRole("button", { name: "Expand Marketing" }).click();
  const tools = tree.getByRole("button", { name: "Tools" });
  await expect(tools).toHaveAttribute("aria-expanded", "false");
  await expect(tree.getByRole("link", { name: "Dashboards" })).toHaveCount(0);
  await tools.click();
  await expect(tools).toHaveAttribute("aria-expanded", "true");
  await expect(tree.getByRole("link", { name: "Dashboards" })).toBeVisible();
});

test("SB-T07 / SB-01: an expanded Space toggle keeps the sidebar colours instead of a light box", async ({ page }) => {
  await mockSpaces(page, { extra: common as never });
  await page.goto(`/spaces/${marketing.id}`);
  const toggle = page.getByRole("navigation", { name: "Spaces" }).getByRole("button", { name: "Expand Marketing" });
  await toggle.click();
  const expanded = page.getByRole("navigation", { name: "Spaces" }).getByRole("button", { name: "Collapse Marketing" });
  await expect(expanded).toHaveAttribute("aria-expanded", "true");
  const bg = await expanded.evaluate((el) => getComputedStyle(el).backgroundColor);
  expect(bg === "rgba(0, 0, 0, 0)" || bg === "transparent").toBe(true);
});

test("SB-T14 / SB-02: the Spaces list uses a thin scrollbar", async ({ page }) => {
  await mockSpaces(page, { extra: common as never });
  await page.goto(`/spaces/${marketing.id}`);
  const width = await page.getByRole("navigation", { name: "Spaces" }).evaluate((el) => getComputedStyle(el).scrollbarWidth);
  expect(width).toBe("thin");
});
