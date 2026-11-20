import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// The Project KPI strip shows the count when it loads, a dash when the count failed, and never sticks on "…".
const id = "c0000000-0000-4000-8000-000000000001";
const project = { id, seq: 1, type: "project", title: "Launch plan", version: 1, my_capabilities: ALL, assignees: [], description: null,
  locations: [{ space_id: marketing.id, space_name: "Marketing" }], children: [] };
const base = {
  [`GET /items/${id}`]: { body: project, headers: { ETag: '"1"' } }, [`GET /items/${id}/children`]: { body: { items: [] } },
  [`GET /items/${id}/comments`]: { body: { items: [] } }, [`GET /items/${id}/attachments`]: { body: { items: [] } },
  [`GET /items/${id}/approvals`]: { body: { items: [] } }, "GET /users": { body: { items: [] } },
  "GET /workflows": { body: { items: [{ id: "w1", name: "Default", statuses: [{ id: "s1", name: "Doing", group: "active" }] }] } },
};

test("the Work items count shows the real number", async ({ page }) => {
  await mockSpaces(page, { extra: { ...base, "POST /analytics/query": { body: { rows: [{ group: { key: "s1", label: "Doing" }, value: 7 }], total: 7 } } } });
  await page.goto(`/tasks/${id}`);
  await expect(page.getByText("Work items")).toBeVisible();
  await expect(page.getByText("Work items").locator("..")).toContainText("7");
});

test("a failed count shows a dash, not a loading placeholder", async ({ page }) => {
  await mockSpaces(page, { extra: { ...base, "POST /analytics/query": { status: 500, body: { code: "internal", detail: "boom", status: 500, title: "Internal", type: "about:blank" } } } });
  await page.goto(`/tasks/${id}`);
  await expect(page.getByText("Work items")).toBeVisible();
  await expect(page.getByText("Work items").locator("..")).toContainText("—");
  await expect(page.getByText("Work items").locator("..")).not.toContainText("…");
});
