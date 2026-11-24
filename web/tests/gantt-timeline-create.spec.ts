import { expect, test, type Page } from "@playwright/test";
import { ganttId, mockGantt, path } from "./gantt-mock";

// Batch 3C: timeline create on the Gantt (day scale). Double-click an empty day, or drag across empty days. The range is
// inclusive and includes weekends (G1). No create on a plain click, a bar, or when the user cannot create (G2, G3, G4).
// The timeline origin is three days before the earliest task (Oct 30, 2026 in this mock).
const newId = "f0000000-0000-4000-8000-000000000002";
const origin = new Date(2026, 9, 30);
// The timeline is scrolled so days 17 to 25 are on screen; a day sits at its cell centre minus the scroll.
const scrollDays = 17;
const dayX = (chart: { x: number }, index: number) => chart.x + index * 36 + 18 - scrollDays * 36;
const dayText = (index: number) => { const d = new Date(origin); d.setDate(d.getDate() + index); return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`; };

async function openChart(page: Page, opts: Parameters<typeof mockGantt>[1] = {}) {
  const { calls } = await mockGantt(page, { ...opts, extra: {
    "POST /items": (body: any) => ({ status: 201, body: { id: newId, title: body.title, start_date: body.start_date, due_date: body.due_date, locations: body.locations, my_capabilities: [] } }),
  } });
  await page.goto(`${path}?view=${ganttId}`);
  const chart = page.locator(".wx-chart").first();
  await chart.waitFor({ state: "visible" });
  await chart.evaluate((el, days) => { (el as HTMLElement).scrollLeft = days * 36; }, scrollDays);
  const box = (await chart.boundingBox())!;
  return { calls, chart: box, y: box.y + box.height - 24 };
}

const creates = (calls: any[]) => calls.filter((c) => c.method === "POST" && c.path === "/items");

test("G2: double-click on an empty day opens Quick Create for that day (start = finish)", async ({ page }) => {
  const { calls, chart, y } = await openChart(page);
  await page.mouse.dblclick(dayX(chart, 20), y);
  const dialog = page.getByRole("dialog", { name: "Create item" });
  await expect(dialog).toBeVisible();
  await dialog.locator('input[name="title"]').fill("Double clicked");
  await dialog.getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => creates(calls)[0]?.body).toMatchObject({ title: "Double clicked", start_date: dayText(20), due_date: dayText(20) });
});

test("G1: drag Monday to Friday creates a range Monday to Friday", async ({ page }) => {
  const { calls, chart, y } = await openChart(page);
  await page.mouse.move(dayX(chart, 17), y);
  await page.mouse.down();
  await page.mouse.move(dayX(chart, 19), y, { steps: 4 });
  await page.mouse.move(dayX(chart, 21), y, { steps: 4 });
  await page.mouse.up();
  const dialog = page.getByRole("dialog", { name: "Create item" });
  await expect(dialog).toBeVisible();
  await dialog.locator('input[name="title"]').fill("Mon to Fri");
  await dialog.getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => creates(calls)[0]?.body).toMatchObject({ start_date: dayText(17), due_date: dayText(21) });
});

test("G1: drag Friday to Monday keeps the weekend inside the range, and creates no allocation", async ({ page }) => {
  const { calls, chart, y } = await openChart(page);
  await page.mouse.move(dayX(chart, 24), y);
  await page.mouse.down();
  await page.mouse.move(dayX(chart, 22), y, { steps: 4 });
  await page.mouse.move(dayX(chart, 21), y, { steps: 4 });
  await page.mouse.up();
  const dialog = page.getByRole("dialog", { name: "Create item" });
  await dialog.locator('input[name="title"]').fill("Fri to Mon");
  await dialog.getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => creates(calls)[0]?.body).toMatchObject({ start_date: dayText(21), due_date: dayText(24) });
  expect(creates(calls)[0].body.assignee_allocations ?? []).toEqual([]);
});

test("G4: a click without a move creates nothing; a drag that leaves the timeline is cancelled", async ({ page }) => {
  const { calls, chart, y } = await openChart(page);
  await page.mouse.click(dayX(chart, 18), y);
  await expect(page.getByRole("dialog", { name: "Create item" })).toHaveCount(0);
  await page.mouse.move(dayX(chart, 17), y);
  await page.mouse.down();
  await page.mouse.move(dayX(chart, 20), y, { steps: 4 });
  await page.mouse.move(dayX(chart, 20), chart.y - 80, { steps: 4 });
  await page.mouse.up();
  await expect(page.getByRole("dialog", { name: "Create item" })).toHaveCount(0);
  expect(creates(calls)).toHaveLength(0);
});

test("G2: double-click on an existing bar creates nothing", async ({ page }) => {
  const { calls, chart } = await openChart(page);
  const bar = page.locator('.wx-bar[data-id=":b0000000-0000-4000-8000-000000000001"]');
  const box = (await bar.boundingBox())!;
  await page.mouse.dblclick(box.x + box.width / 2, box.y + box.height / 2);
  await expect(page.getByRole("dialog", { name: "Create item" })).toHaveCount(0);
  expect(creates(calls)).toHaveLength(0);
  expect(chart.width).toBeGreaterThan(0);
});

test("Week and Month: timeline create is off, the chart says why, and the row menu still works", async ({ page }) => {
  const { calls, chart, y } = await openChart(page);
  await page.getByRole("button", { name: "Weeks" }).click();
  await expect(page.locator(".gantt-chart")).toHaveAttribute("title", "Switch to Day view to create a task on the timeline.");
  await page.mouse.dblclick(dayX(chart, 20), y);
  await expect(page.getByText("Switch to Day view to create a task on the timeline.")).toBeVisible();
  await expect(page.getByRole("dialog", { name: "Create item" })).toHaveCount(0);
  expect(creates(calls)).toHaveLength(0);
  // The scale does not change on its own, and the row menu is still there.
  await expect(page.getByRole("button", { name: "Weeks" })).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByRole("button", { name: "Actions for Design" })).toBeVisible();
});

test("Permission: without create, the timeline offers no create", async ({ page }) => {
  const { calls, chart, y } = await openChart(page, { space: ["view", "comment"] });
  await page.mouse.dblclick(dayX(chart, 20), y);
  await expect(page.getByRole("dialog", { name: "Create item" })).toHaveCount(0);
  expect(creates(calls)).toHaveLength(0);
});
