import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { mockSpaces, personal } from "./spaces-mock";

const day = (offset: number) => new Date(Date.now() + offset * 864e5).toLocaleDateString("sv");
const task = (id: string, title: string, due: string | null, status = "Active") => ({
  id, type: "task", title, version: 1, due_date: due, locations: [{ space_id: personal.id }], assignees: [{ id: "u1", name: "Ada" }],
  my_capabilities: ["view"], status: { id: status, name: status, group: "active", color: "blue" },
});

test("My to-do groups by date with counts, quick-adds into the personal Space and has no axe violations", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: {
    "GET /me/todo": { body: { items: [task("a1", "Late one", day(-2)), task("a2", "Later one", day(30)), task("a3", "No date", null)], next_cursor: null } },
    "GET /workflows": { body: { items: [] } },
    "POST /items": { status: 201, body: task("a4", "Buy milk", null) },
  } });
  await page.goto("/my-todo");
  await expect(page.getByRole("heading", { name: "Overdue (1)" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Later (1)" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "No date (1)" })).toBeVisible();
  await page.getByLabel("New to-do title").fill("Buy milk");
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === "/items")?.body)
    .toMatchObject({ type: "task", title: "Buy milk", locations: [{ space_id: personal.id }] });
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("Inbox tabs request their box, Sent shows the recipient, and axe is clean", async ({ page }) => {
  const entry = (id: string) => ({ id, kind: "shared", created_at: new Date().toISOString(), read_at: null, actor: { id: "u1", name: "Ada" }, recipient: { id: "u2", name: "Grace" }, item_id: null, item_title: "Plan", space_id: null, space_title: null, snippet: null, archived_at: null });
  const { calls } = await mockSpaces(page, { extra: {
    "GET /notifications": (_, req) => ({ body: { items: req.query.get("box") === "sent" ? [entry("n1")] : [], next_cursor: null, unread_count: 0 } }),
  } });
  await page.goto("/inbox");
  await expect(page.getByText("You're all caught up")).toBeVisible();
  await page.getByRole("tab", { name: "Sent" }).click();
  await expect.poll(() => calls.some((c) => c.path === "/notifications" && c.query.get("box") === "sent")).toBe(true);
  await page.getByRole("button", { name: /shared Plan/ }).click();
  await expect(page.getByText("To Grace")).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByRole("tab", { name: "Archive" }).click();
  await expect(page.getByText("Nothing archived")).toBeVisible();
});

test("Settings has no Gantt spike link and 2FA text is neutral for non-admins", async ({ page }) => {
  await mockSpaces(page, { me: { ...me, user_type: "regular" }, extra: { "GET /me/2fa": { body: { configured: false, enabled: false } } } });
  await page.goto("/settings/security");
  await expect(page.getByRole("link", { name: /gantt spike/i })).toHaveCount(0);
  await expect(page.getByText("PMS_2FA_KEY")).toHaveCount(0);
});

test("Dashboard task list widget groups by due date with status chips and an empty state", async ({ page }) => {
  const d = "d0000000-0000-4000-8000-000000000001", w = "e0000000-0000-4000-8000-000000000001";
  let empty = false;
  await mockSpaces(page, { extra: {
    "GET /dashboards": { body: { items: [{ id: d, owner_id: me.id, space_id: null, name: "Board", version: 1 }] } },
    [`GET /dashboards/${d}`]: { body: { id: d, owner_id: me.id, space_id: null, name: "Board", layout: "columns", version: 1,
      widgets: [{ id: w, name: "This week", type: "task_list", position: 1, config: { list: "todo_this_week" } }] } },
    [`GET /dashboards/${d}/widgets/${w}/data`]: () => ({ body: { kind: "tasks", items: empty ? [] : [
      { id: "f0000000-0000-4000-8000-000000000001", title: "Write the plan", status: "Open", status_group: "active", due_date: day(-1), importance: "high" },
      { id: "f0000000-0000-4000-8000-000000000002", title: "Ship it", status: "Done", status_group: "completed", due_date: "", importance: "low" }] } }),
  } });
  await page.goto(`/dashboards/${d}`);
  await expect(page.getByRole("heading", { name: "Overdue (1)" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "No date (1)" })).toBeVisible();
  await expect(page.getByText("Open", { exact: true })).toBeVisible();
  empty = true;
  await page.reload();
  await expect(page.getByText("Nothing to show.")).toBeVisible();
});
