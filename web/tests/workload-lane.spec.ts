import { expect, test, type Page } from "@playwright/test";
import { mockSpaces, marketing } from "./spaces-mock";

// Batch 4 (W1, W2, W4): drag across a person's working days in Workload to create a Task. Mocked API.
const ana = { id: "a0000000-0000-4000-8000-00000000000a", name: "Ana Lane", capacity: [] as number[], planned: [] as number[], booked: [] as number[] };
const iso = (d: Date) => d.toISOString().slice(0, 10);
const from = new Date();
const dayAt = (i: number) => new Date(Date.UTC(from.getUTCFullYear(), from.getUTCMonth(), from.getUTCDate() + i));

function setup(page: Page, plannedFirstDay = 0) {
  const capacity = Array.from({ length: 14 }, (_, i) => ([1, 2, 3, 4, 5].includes(dayAt(i).getUTCDay()) ? 480 : 0));
  const person = { ...ana, capacity, planned: capacity.map((_, i) => (i === 0 ? plannedFirstDay : 0)), booked: capacity.map(() => 0) };
  const posted: any[] = [];
  return mockSpaces(page, { extra: {
    "GET /workload": { body: { people: [person], tasks: [], bookings: [], role_demand: [] } },
    "GET /utilization": { body: { items: [] } },
    "GET /resource-directory": { body: { roles: [], skills: [] } },
    "POST /items": (body: any) => { posted.push(body); return { status: 201, body: { id: "e0000000-0000-4000-8000-000000000002", ...body } }; },
  } }).then(() => posted);
}

test("Workload: dragging across working days creates a Task for that person with a 60 minute allocation", async ({ page }) => {
  const posted = await setup(page);
  await page.goto("/workload");
  const row = page.getByRole("table", { name: "Daily workload" }).getByRole("row", { name: /Ana Lane/ });
  // Day 0 (today) to day 3: the weekend between is skipped, so the Task spans the two working days 0 and 3 (or 0..3 if all work).
  const start = row.locator("td").nth(0);
  const end = row.locator("td").nth(3);
  const a = await start.boundingBox(); const b = await end.boundingBox();
  await page.mouse.move(a!.x + a!.width / 2, a!.y + a!.height / 2);
  await page.mouse.down();
  await page.mouse.move(b!.x + b!.width / 2, b!.y + b!.height / 2, { steps: 4 });
  await page.mouse.up();
  const form = page.getByRole("form", { name: /New task for Ana Lane/ });
  await form.getByLabel("Task name").fill("Lane task");
  await form.getByRole("button", { name: "Create task" }).click();
  await expect.poll(() => posted[0]).toMatchObject({
    type: "task", title: "Lane task", start_date: iso(dayAt(0)), due_date: iso(dayAt(3)),
    assignee_ids: [ana.id], assignee_allocations: [{ assignee_id: ana.id, allocation: { unit: "minutes_per_day", value: 60 } }],
  });
});

test("Workload: a day that would go over capacity warns but still creates the Task", async ({ page }) => {
  const posted = await setup(page, 430);
  await page.goto("/workload");
  const row = page.getByRole("table", { name: "Daily workload" }).getByRole("row", { name: /Ana Lane/ });
  const a = await row.locator("td").nth(0).boundingBox();
  await page.mouse.move(a!.x + a!.width / 2, a!.y + a!.height / 2);
  await page.mouse.down();
  await page.mouse.up();
  const form = page.getByRole("form", { name: /New task for Ana Lane/ });
  await expect(form.getByRole("status")).toContainText(`Over capacity on ${iso(dayAt(0))}`);
  await form.getByLabel("Task name").fill("Tight day");
  await form.getByRole("button", { name: "Create task" }).click();
  await expect.poll(() => posted.length).toBe(1);
});

test("Workload: the keyboard path creates the same Task (Enter, arrows, Enter)", async ({ page }) => {
  const posted = await setup(page);
  await page.goto("/workload");
  const row = page.getByRole("table", { name: "Daily workload" }).getByRole("row", { name: /Ana Lane/ });
  await row.locator("td").nth(0).focus();
  await page.keyboard.press("Enter");
  for (let i = 0; i < 3; i++) await page.keyboard.press("ArrowRight");
  await page.keyboard.press("Enter");
  const form = page.getByRole("form", { name: /New task for Ana Lane/ });
  await form.getByLabel("Task name").fill("Keyboard task");
  await form.getByRole("button", { name: "Create task" }).click();
  await expect.poll(() => posted[0]).toMatchObject({ title: "Keyboard task", start_date: iso(dayAt(0)), due_date: iso(dayAt(3)) });
});
