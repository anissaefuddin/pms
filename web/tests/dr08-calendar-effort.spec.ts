import { expect, test } from "@playwright/test";
import { ALL, extra, grace, marketing, mockSpaces } from "./spaces-mock";

// DR-08: the Calendar view and the Effort block shared the ["work-schedule"] query key with different data shapes; the page crashed.
const id = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaab";
const calId = "77777777-7777-4777-8777-777777777772";
const status = { id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", name: "New", color: "slate", group: "active" };

test("opening an assigned item from the Calendar view shows the Effort block without crashing", async ({ page }) => {
  const project = {
    id, type: "project", title: "Launch", seq: 1, version: 1, locations: [{ space_id: marketing.id, parent_item_id: null }],
    my_capabilities: ALL, status, effort_minutes: null, assignees: [],
  };
  const taskId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaac";
  const task = { ...project, id: taskId, type: "task", title: "Assigned task", seq: 2, assignees: [{ id: grace.id, name: "Grace Hopper" }] };
  await mockSpaces(page, { extra: extra({
    [`GET /items/${id}`]: { headers: { ETag: '"1"' }, body: project },
    [`GET /items/${id}/children`]: { body: { items: [task] } },
    [`GET /items/${id}/items`]: { body: { items: [task] } },
    [`GET /items/${taskId}`]: { headers: { ETag: '"1"' }, body: task },
    [`GET /items/${taskId}/children`]: { body: { items: [] } },
    [`GET /items/${id}/views`]: { body: { items: [{ id: calId, type: "calendar", name: "Calendar", visibility: "public", position: 0, is_default: true, config: {} }] } },
    "GET /workflows": { body: { items: [] } },
    "GET /settings/work-schedule": { body: { working_days: [1, 2, 3, 4, 5], daily_hours: 6, version: 1, exceptions: [] } },
  }) });
  await page.goto(`/tasks/${id}`);
  await expect(page.getByRole("button", { name: "Next month" })).toBeVisible();
  await page.getByRole("button", { name: "Unscheduled 1" }).click();
  await page.getByLabel("Assigned task").click();
  await page.getByText("More fields", { exact: true }).click();
  await expect(page.getByRole("region", { name: "Effort and allocation" })).toBeVisible();
  await expect(page.getByText("Something went wrong")).toHaveCount(0);
});
