import { expect, test } from "@playwright/test";
import { kids, mockGantt, path, project, ganttId } from "./gantt-mock";

// Batch 3B: the Gantt row menu. Add subitem, Add task above, Add task below. A new Task takes the row's day (one day), sits
// under the same parent, and is placed with the reorder endpoint. Mocked API; the server stores each call as usual.
const newId = "f0000000-0000-4000-8000-000000000001";
const reorderPath = `/items/${newId}/reorder`;

async function setup(page: import("@playwright/test").Page) {
  const reorders: any[] = [];
  const { calls } = await mockGantt(page, { extra: {
    "POST /items": (body: any) => ({ status: 201, body: { ...kids[0], id: newId, title: body.title, start_date: body.start_date, due_date: body.due_date, locations: body.locations } }),
    [`POST ${reorderPath}`]: (body: any) => { reorders.push(body); return { status: 204 }; },
  } });
  return { calls, reorders };
}

async function openRowMenu(page: import("@playwright/test").Page, title: string) {
  await page.goto(`${path}?view=${ganttId}`);
  await page.getByRole("button", { name: `Actions for ${title}` }).click();
}

test("Gantt: Add task below creates a one-day Task on the row's day and places it under the row", async ({ page }) => {
  const { calls, reorders } = await setup(page);
  await openRowMenu(page, kids[0].title);
  await page.getByRole("menuitem", { name: "Add task below" }).click();
  await page.locator('input[name="title"]').fill("Below design");
  await page.getByRole("dialog").getByRole("button", { name: "Create item" }).click();
  const day = kids[0].start_date;
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === "/items")?.body).toMatchObject({
    title: "Below design", start_date: day, due_date: day, locations: [{ space_id: expect.any(String), parent_item_id: project.id }],
  });
  await expect.poll(() => reorders.length).toBe(1);
  expect(reorders[0]).toMatchObject({ parent_item_id: project.id, after_item_id: kids[0].id });
});

test("Gantt: Add subitem on a row creates a child of that row", async ({ page }) => {
  const { calls } = await setup(page);
  await openRowMenu(page, kids[0].title);
  await page.getByRole("menuitem", { name: "Add subtask" }).click();
  await page.locator('input[name="title"]').fill("Child of design");
  await page.getByRole("dialog").getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === "/items")?.body).toMatchObject({
    title: "Child of design", locations: [{ space_id: expect.any(String), parent_item_id: kids[0].id }],
  });
});

test("Gantt: Add task above places the new Task before the row, after the sibling above it", async ({ page }) => {
  const { reorders } = await setup(page);
  await openRowMenu(page, kids[1].title);
  await page.getByRole("menuitem", { name: "Add task above" }).click();
  await page.locator('input[name="title"]').fill("Above build");
  await page.getByRole("dialog").getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => reorders.length).toBe(1);
  // Above "Build" means after "Design" (the sibling before Build), so it lands between them.
  expect(reorders[0]).toMatchObject({ parent_item_id: project.id, after_item_id: kids[0].id });
});
