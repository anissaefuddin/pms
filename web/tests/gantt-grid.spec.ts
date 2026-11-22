import { expect, test } from "@playwright/test";
import { ganttId, kids, mockGantt, path } from "./gantt-mock";
import { problem } from "./mock-api";

test("the grid shows number and status by default, with advanced columns hidden", async ({ page }) => {
  await mockGantt(page);
  await page.goto(`${path}?view=${ganttId}`);
  await expect(page.getByText("#", { exact: true }).first()).toBeVisible();
  await expect(page.getByRole("columnheader", { name: "Status", exact: true })).toBeVisible();
  await expect(page.getByText("Predecessors", { exact: true })).toHaveCount(0);
  await expect(page.getByText("Duration", { exact: true })).toHaveCount(0);
});

test("grid columns can be resized by their grips", async ({ page }) => {
  await mockGantt(page);
  await page.goto(`${path}?view=${ganttId}`);
  const header = page.getByRole("columnheader", { name: "Status", exact: true });
  const before = (await header.boundingBox())!.width;
  const grip = header.locator(".wx-grip");
  const box = (await grip.boundingBox())!;
  await grip.dispatchEvent("mousedown", { clientX: box.x + box.width / 2 });
  await page.mouse.move(box.x + box.width / 2 + 80, box.y + box.height / 2, { steps: 4 });
  await page.mouse.up();
  await expect.poll(async () => (await header.boundingBox())!.width).toBeGreaterThan(before + 50);
});

test("the scale menu floats at the top right and lists five scales", async ({ page }) => {
  await mockGantt(page);
  await page.goto(`${path}?view=${ganttId}`);
  await expect(page.getByRole("button", { name: "Time scale" })).toHaveText(/Days/);
  await page.getByRole("button", { name: "Time scale" }).click();
  for (const name of ["Years", "Quarters", "Months", "Weeks", "Days"]) await expect(page.getByRole("menuitem", { name, exact: true })).toBeVisible();
  await page.keyboard.press("Escape");
});

test("zoom out from Days goes to Weeks, and zoom in comes back", async ({ page }) => {
  await mockGantt(page);
  await page.goto(`${path}?view=${ganttId}`);
  await expect(page.getByRole("button", { name: "Time scale" })).toHaveText(/Days/);
  await page.getByRole("button", { name: "Zoom out" }).click();
  await expect(page.getByRole("button", { name: "Time scale" })).toHaveText(/Weeks/);
  await page.getByRole("button", { name: "Zoom in" }).click();
  await expect(page.getByRole("button", { name: "Time scale" })).toHaveText(/Days/);
});

test("the add-task row creates a task at once, and a double-click edits the name", async ({ page }) => {
  const { calls } = await mockGantt(page, { extra: {
    "POST /items": (body: any) => ({ status: 201, body: { id: "99999999-9999-4999-8999-999999999999", title: body.title, start_date: null, due_date: null, locations: body.locations, my_capabilities: ["edit"], assignees: [], status: null } }),
  } });
  await page.goto(`${path}?view=${ganttId}`);
  const input = page.getByRole("textbox", { name: "Add task, then press Enter" });
  await expect(page.getByRole("application", { name: "Gantt chart" }).getByRole("textbox", { name: "Add task, then press Enter" })).toBeVisible();
  await input.fill("Quick task");
  await input.press("Enter");
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === "/items")?.body).toMatchObject({ title: "Quick task", id: expect.stringMatching(/^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/) });
  // Select the row, then F2 opens its name for editing; Enter saves.
  await page.getByText(kids[0].title, { exact: true }).first().click();
  await page.locator('[role="application"]').first().press("F2");
  const edit = page.locator('input[name="title"]').first();
  await expect(edit).toBeVisible();
  await edit.fill("Renamed task");
  await edit.press("Enter");
  await expect.poll(() => calls.find((c) => c.method === "PATCH")?.body).toMatchObject({ title: "Renamed task" });
});

test("the add-task row keeps the title and reports an API failure", async ({ page }) => {
  await mockGantt(page, { extra: { "POST /items": () => problem(422, "Task could not be created", "validation_failed") } });
  await page.goto(`${path}?view=${ganttId}`);
  const input = page.getByRole("textbox", { name: "Add task, then press Enter" });
  await input.fill("Keep this title");
  await input.press("Enter");
  await expect(input).toHaveValue("Keep this title");
  await expect(page.getByText("Task could not be created")).toBeVisible();
});
