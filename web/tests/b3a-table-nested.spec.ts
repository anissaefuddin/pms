import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// RT-37 / QA-D-lane3-010: second-level rows expand, and the filter searches the server so unexpanded rows are found.
const path = `/spaces/${marketing.id}`;
const status = { id: "88888888-8888-4888-8888-888888888888", name: "New", group: "active", color: "slate" };
const base = { seq: 1, version: 1, status, my_capabilities: ALL, assignees: [] };
const ita = { ...base, id: "77777777-7777-4777-8777-777777777777", type: "project", title: "ITA", child_count: 1, locations: [{ space_id: marketing.id }] };
const kurau = { ...base, id: "66666666-6666-4666-8666-666666666666", type: "folder", title: "Kurau Plant", child_count: 1, locations: [{ space_id: marketing.id, parent_item_id: ita.id }] };
const hazop = { ...base, id: "44444444-4444-4444-8444-444444444444", type: "task", title: "HAZOP study", child_count: 0, locations: [{ space_id: marketing.id, parent_item_id: kurau.id }] };
const table = { id: "55555555-5555-4555-8555-555555555555", type: "table", name: "Table", visibility: "public", position: 0, is_default: true };
const extra = {
  [`GET ${path}/views`]: { body: { items: [table] } },
  [`GET ${path}/items`]: { body: { items: [ita] } },
  [`GET /items/${ita.id}/children`]: { body: { items: [kurau] } },
  [`GET /items/${kurau.id}/children`]: { body: { items: [hazop] } },
  "GET /workflows": { body: { items: [{ is_default: true, statuses: [status] }] } },
};

test("a second-level row expands and loads its children", async ({ page }) => {
  await mockSpaces(page, { extra });
  await page.goto(path);
  await page.getByRole("button", { name: "Expand ITA", exact: true }).click();
  await page.getByRole("button", { name: "Expand Kurau Plant", exact: true }).click();
  await expect(page.getByRole("button", { name: "HAZOP study", exact: true })).toBeVisible();
});

test("table density and hierarchy controls are persistent and expand nested rows", async ({ page }) => {
  await mockSpaces(page, { extra });
  await page.goto(path);
  await expect(page.getByRole("button", { name: "Compact rows" })).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("button", { name: "Compact rows" }).click();
  await expect(page.getByRole("button", { name: "Compact rows" })).toHaveAttribute("aria-pressed", "false");
  await page.getByRole("button", { name: "Compact rows" }).click();
  await page.getByRole("button", { name: "Expand all" }).click();
  await expect(page.getByRole("button", { name: "HAZOP study", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Collapse all" }).click();
  await expect(page.getByRole("button", { name: "Kurau Plant", exact: true })).toHaveCount(0);
  await page.reload();
  await expect(page.getByRole("button", { name: "Compact rows" })).toHaveAttribute("aria-pressed", "true");
});

test("the filter finds an unexpanded nested row through server search and shows its path", async ({ page }) => {
  await mockSpaces(page, { extra: { ...extra,
    "GET /search": { body: { items: [{ ...hazop, path: [{ id: ita.id, title: "ITA", type: "project" }, { id: kurau.id, title: "Kurau Plant", type: "folder" }] }], comments: [], users: [] } } } });
  await page.goto(path);
  await page.getByRole("button", { name: "Filter", exact: true }).click();
  await page.getByRole("searchbox").or(page.getByPlaceholder(/filter|search/i)).first().fill("HAZOP");
  await expect(page.getByRole("button", { name: "HAZOP study", exact: true })).toBeVisible();
  await expect(page.getByText("ITA › Kurau Plant")).toBeVisible();
  await expect(page.getByText("No matching items")).toHaveCount(0);
});

test("the table keeps its primary and action columns pinned", async ({ page }) => {
  await mockSpaces(page, { extra });
  await page.goto(path);
  const table = page.getByRole("table");
  await expect(table.getByRole("columnheader", { name: "Name" })).toHaveClass(/sticky/);
  await expect(table.getByRole("columnheader", { name: "Actions" })).toHaveClass(/sticky/);
  await expect(table.getByRole("row").nth(1).getByRole("cell").nth(1)).toHaveClass(/sticky/);
});

test("the table supports standard keyboard row navigation", async ({ page }) => {
  await mockSpaces(page, { extra });
  await page.goto(path);
  const grid = page.getByRole("table");
  await grid.focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("button", { name: "Kurau Plant", exact: true })).toBeVisible();
  await page.keyboard.press("ArrowDown");
  await expect(grid.getByRole("row").nth(2)).toHaveAttribute("aria-current", "true");
  await page.keyboard.press("Home");
  await expect(grid.getByRole("row").nth(1)).toHaveAttribute("aria-current", "true");
});
