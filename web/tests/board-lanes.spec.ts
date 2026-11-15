import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const boardId = "66666666-6666-4666-8666-666666666666";
const path = `/spaces/${marketing.id}`;
const ann = { id: "a1000000-0000-4000-8000-000000000001", name: "Ann Lee" };
const bob = { id: "a1000000-0000-4000-8000-000000000002", name: "Bob Ray" };
const nw = { id: "88888888-8888-4888-8888-888888888888", name: "New", group: "active", color: "slate" };
const done = { id: "99999999-9999-4999-8999-999999999999", name: "Done", group: "completed", color: "green" };
const day = (n: number) => new Date(Date.now() - n * 86_400_000).toISOString();
const mk = (n: number, title: string, assignees: object[], extra: object = {}) => ({
  id: `b0000000-0000-4000-8000-00000000000${n}`, seq: n, type: "task", title, version: 1, status: nw, my_capabilities: ALL, assignees,
  locations: [{ space_id: marketing.id, space_name: "Marketing" }], created_by: { id: ann.id, name: ann.name }, status_changed_at: day(4), ...extra,
});
const shared = mk(1, "Shared work", [ann, bob], {
  cover_attachment_id: "c1000000-0000-4000-8000-000000000001", importance: "high", start_date: "2026-11-10", due_date: "2026-11-20",
});
const solo = mk(2, "Solo work", [ann]);
const orphan = mk(3, "Nobody's work", []);

function mock(page: import("@playwright/test").Page, config: object = {}) {
  let views = [{ id: boardId, type: "board", name: "Board", visibility: "public", position: 0, is_default: true, config }];
  let items: any[] = [shared, solo, orphan];
  return mockSpaces(page, { extra: {
    [`GET ${path}/views`]: () => ({ body: { items: views } }),
    [`GET ${path}/items`]: () => ({ body: { items } }),
    "GET /workflows": { body: { items: [{ is_default: true, statuses: [nw, done] }] } },
    [`PATCH /views/${boardId}`]: (body) => { views = [{ ...views[0], ...body }]; return { body: views[0] }; },
    ...Object.fromEntries(items.map((item) => [`PATCH /items/${item.id}`, (body: any) => {
      const assignees = body.assignee_ids ? body.assignee_ids.map((id: string) => [ann, bob].find((person) => person.id === id)) : item.assignees;
      const next = { ...item, ...body, assignees, version: item.version + 1, status: body.status_id === done.id ? done : item.status };
      items = items.map((entry) => entry.id === item.id ? next : entry);
      return { body: next };
    }])),
  } });
}

test("group by Assignee makes swimlanes; a multi-assignee item shows in each lane", async ({ page }) => {
  await mock(page);
  await page.goto(`${path}?group=assignee`);
  for (const lane of ["Ann Lee", "Bob Ray", "Unassigned"]) await expect(page.getByRole("region", { name: lane, exact: true })).toBeVisible();
  const lane = (name: string) => page.getByRole("region", { name, exact: true });
  await expect(lane("Ann Lee").getByRole("button", { name: "Shared work", exact: true })).toBeVisible();
  await expect(lane("Bob Ray").getByRole("button", { name: "Shared work", exact: true })).toBeVisible();
  await expect(lane("Ann Lee").getByRole("button", { name: "Solo work", exact: true })).toBeVisible();
  await expect(lane("Unassigned").getByRole("button", { name: "Nobody's work", exact: true })).toBeVisible();
  await expect(lane("Bob Ray").getByRole("button", { name: "Solo work", exact: true })).toHaveCount(0);
});

test("dragging to another lane asks first, then replaces only that assignee", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto(`${path}?group=assignee`);
  await page.setViewportSize({ width: 1280, height: 1800 }); // lanes are stacked; HTML5 drag does not survive scrolling
  const card = page.getByRole("region", { name: "Ann Lee: New" }).getByRole("listitem").filter({ hasText: "Solo work" });
  await card.dragTo(page.getByRole("region", { name: "Bob Ray: New" }));
  const dialog = page.getByRole("dialog", { name: "Change assignee?" });
  await expect(dialog).toContainText("Solo work");
  await dialog.getByRole("button", { name: "Cancel" }).click();
  expect(calls.some((call) => call.method === "PATCH")).toBe(false);
  await card.dragTo(page.getByRole("region", { name: "Bob Ray: New" }));
  await page.getByRole("dialog", { name: "Change assignee?" }).getByRole("button", { name: "Change assignee" }).click();
  await expect.poll(() => calls.find((call) => call.method === "PATCH" && call.path === `/items/${solo.id}`)?.body).toEqual({ assignee_ids: [bob.id] });
  // The shared item keeps its other assignee when dragged out of Ann's lane.
  await page.getByRole("region", { name: "Ann Lee: New" }).getByRole("listitem").filter({ hasText: "Shared work" })
    .dragTo(page.getByRole("region", { name: "Unassigned: New" }));
  await page.getByRole("dialog", { name: "Change assignee?" }).getByRole("button", { name: "Change assignee" }).click();
  await expect.poll(() => calls.find((call) => call.method === "PATCH" && call.path === `/items/${shared.id}`)?.body).toEqual({ assignee_ids: [bob.id] });
});

test("without lanes a status drop is still immediate, and the card picker saves the chosen fields", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto(path);
  // The first drag can land before React has rendered the drop targets on a slow runner, so retry the whole gesture.
  await expect(async () => {
    await page.getByRole("listitem").filter({ hasText: "Solo work" }).dragTo(page.getByRole("region", { name: "Done" }));
    expect(calls.find((call) => call.method === "PATCH" && call.path === `/items/${solo.id}`)?.body).toEqual({ status_id: done.id });
  }).toPass({ timeout: 20000 });
  await expect(page.getByRole("dialog")).toHaveCount(0);
  const card = page.getByRole("listitem").filter({ hasText: "Shared work" });
  await expect(card).not.toContainText("days in status");
  await page.getByRole("button", { name: "Card fields" }).click();
  const dialog = page.getByRole("dialog", { name: "Choose card fields" });
  for (const name of ["Cover image", "ID", "Type", "Priority", "Days in status", "Author", "Locations"]) await dialog.getByLabel(name, { exact: true }).check();
  await dialog.getByLabel("Assignees", { exact: true }).uncheck();
  await dialog.getByLabel("Start date", { exact: true }).uncheck();
  await dialog.getByRole("button", { name: "Save columns" }).click();
  await expect.poll(() => calls.find((call) => call.method === "PATCH" && call.path === `/views/${boardId}`)?.body.config.columns)
    .toEqual(["status", "due_date", "cover", "id", "type", "importance", "days_in_status", "author", "locations"]);
  await expect(card).toContainText("#1");
  await expect(card).toContainText("4 days in status");
  await expect(card).toContainText("By Ann Lee");
  await expect(card).toContainText("Marketing");
  await expect(card).toContainText("High");
  await expect(card).not.toContainText("Ann Lee, Bob Ray");
  await expect(card).not.toContainText("2026-11-10");
  await expect(card).toContainText("2026-11-20");
  await expect(card.locator("img")).toHaveAttribute("src", "/api/v1/attachments/c1000000-0000-4000-8000-000000000001/download");
});
