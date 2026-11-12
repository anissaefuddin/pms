import { expect, test } from "@playwright/test";
import { mockSpaces, marketing } from "./spaces-mock";

test("My to-do filters assigned tasks, paginates, and changes status with the current version", async ({ page }) => {
  const firstId = "81111111-1111-4111-8111-111111111111";
  const nextId = "82222222-2222-4222-8222-222222222222";
  const task = {
    id: firstId, type: "task", title: "Prepare brief", version: 2, due_date: "2026-11-10",
    locations: [{ space_id: marketing.id }], my_capabilities: ["view", "change_status"],
    status: { id: "active", name: "In Progress", group: "active", color: "blue" },
  };
  const done = { id: "completed", name: "Completed", group: "completed", color: "green" };
  let completed = false;
  const { calls } = await mockSpaces(page, {
    extra: {
      "GET /me/todo": (_, req) => {
        if (req.query.get("cursor")) {
          return { body: { items: [{ ...task, id: nextId, title: "Send review", my_capabilities: ["view"] }], next_cursor: null } };
        }
        return { body: completed ? { items: [], next_cursor: null } : { items: [task], next_cursor: "page-2" } };
      },
      "GET /workflows": { body: { items: [{ id: "wf", is_default: true, statuses: [task.status, done] }] } },
      [`PATCH /items/${firstId}`]: () => { completed = true; return { body: { ...task, status: done, version: 3 } }; },
    },
  });

  await page.goto(`/spaces/${marketing.id}`);
  await page.getByRole("button", { name: "More", exact: true }).click();
  await page.getByRole("menuitem", { name: "My to-do" }).click();
  await expect(page.getByRole("heading", { name: "My to-do" })).toBeVisible();
  await expect(page.getByLabel("Filter tasks")).toHaveValue("active");
  await expect(page.getByRole("link", { name: "Prepare brief" })).toHaveAttribute("href", `/tasks/${firstId}`);
  await expect(page.getByText("2026-11-10")).toBeVisible();
  await expect(page.getByRole("main").getByText("Marketing")).toBeVisible();
  await expect(page.getByLabel("Sort tasks").getByRole("option")).toHaveCount(5);

  await page.getByLabel("Filter tasks").selectOption("overdue");
  await page.getByLabel("Sort tasks").selectOption("date_modified");
  await expect(page).toHaveURL(/filter=overdue&sort=date_modified/);
  await expect.poll(() => calls.some((call) => call.path === "/me/todo"
    && call.query.get("filter") === "overdue" && call.query.get("sort") === "date_modified")).toBe(true);
  await page.getByRole("button", { name: "Load more" }).click();
  await expect(page.getByRole("link", { name: "Send review" })).toBeVisible();

  await page.getByLabel("Status for Prepare brief").selectOption("completed");
  await expect(page.getByRole("link", { name: "Prepare brief" })).toHaveCount(0);
  const patch = calls.find((call) => call.method === "PATCH" && call.path === `/items/${firstId}`);
  expect(patch?.headers["if-match"]).toBe('"2"');
  expect(patch?.body).toEqual({ status_id: "completed" });
});
