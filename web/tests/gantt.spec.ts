import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { bar, design, ganttId, kids, milestone, mockGantt, path, project } from "./gantt-mock";

async function drag(page: import("@playwright/test").Page, id: string, dx: number, edge: "move" | "end" = "move") {
  const box = (await bar(page, id).boundingBox())!;
  const x = edge === "end" ? box.x + box.width - 2 : box.x + box.width / 2;
  const y = box.y + box.height / 2;
  await page.mouse.move(x, y);
  await page.mouse.down();
  await page.mouse.move(x + dx / 2, y, { steps: 4 });
  await page.mouse.move(x + dx, y, { steps: 4 });
  await page.mouse.up();
}

test("shows the hierarchy with progress, a milestone and the three scales", async ({ page }) => {
  await mockGantt(page);
  await page.goto(`${path}?view=${ganttId}`);
  await expect(bar(page, project.id)).toHaveClass(/wx-summary/);
  await expect(bar(page, project.id).locator(".wx-progress-marker")).toHaveText("40");
  await expect(bar(page, milestone.id)).toHaveClass(/wx-milestone/);
  await expect(bar(page, design)).toBeVisible();
  const width = async () => (await bar(page, design).boundingBox())!.width;
  expect(await width()).toBe(72); // 2 days (Nov 2 to 3, inclusive) x 36px
  await page.getByRole("button", { name: "Weeks" }).click();
  await expect.poll(width).toBeLessThan(40); // narrower than the 72px day-scale bar
  const weekWidth = await width();
  await page.getByRole("button", { name: "Months" }).click();
  await expect(page.getByRole("button", { name: "Months" })).toHaveAttribute("aria-pressed", "true");
  await expect.poll(width).toBeLessThan(weekWidth);
  await page.getByRole("button", { name: "Days" }).click();
  await expect(page.locator(".gantt-today").first()).toBeVisible();
});

test("drag moves the item and resize changes the finish date, with If-Match and the server's answer", async ({ page }) => {
  const { calls } = await mockGantt(page);
  await page.goto(`${path}?view=${ganttId}`);
  await drag(page, design, 72);
  await expect.poll(() => calls.find((call) => call.method === "PATCH")?.body).toEqual({ start_date: "2026-11-04", due_date: "2026-11-05" });
  expect(calls.find((call) => call.method === "PATCH")?.headers["if-match"]).toBe('"1"');
  await expect.poll(async () => (await bar(page, design).boundingBox())!.x - (await bar(page, project.id).boundingBox())!.x).toBe(72 + 0);
  const patches = () => calls.filter((call) => call.method === "PATCH");
  await drag(page, design, 36, "end");
  await expect.poll(() => patches().at(-1)?.body).toEqual({ start_date: "2026-11-04", due_date: "2026-11-06" });
  expect(patches().at(-1)?.headers["if-match"]).toBe('"2"');
});

test("a conflict keeps the bar where it was and says so", async ({ page }) => {
  const { calls } = await mockGantt(page, { conflict: true });
  await page.goto(`${path}?view=${ganttId}`);
  const before = (await bar(page, design).boundingBox())!.x;
  await drag(page, design, 72);
  await expect.poll(() => calls.some((call) => call.method === "PATCH")).toBe(true);
  await expect(page.getByText("Item was updated by someone else")).toBeVisible();
  expect((await bar(page, design).boundingBox())!.x).toBe(before);
});

test("view-only users cannot drag, and the chart says so", async ({ page }) => {
  const { calls } = await mockGantt(page, { space: ["view"] });
  await page.goto(`${path}?view=${ganttId}`);
  await expect(page.getByText("You can view this chart but not change dates.")).toBeVisible();
  const before = (await bar(page, design).boundingBox())!.x;
  await drag(page, design, 72);
  await page.waitForTimeout(300);
  expect(calls.some((call) => call.method === "PATCH")).toBe(false);
  expect((await bar(page, design).boundingBox())!.x).toBe(before);
});

test("keyboard selects a task and moves it one day; Shift changes the finish date", async ({ page }) => {
  const { calls } = await mockGantt(page);
  await page.goto(`${path}?view=${ganttId}`);
  const chart = page.getByRole("application", { name: "Gantt chart" });
  await chart.focus();
  await page.keyboard.press("ArrowDown"); // Launch
  await page.keyboard.press("ArrowDown"); // Design
  await expect(page.getByRole("status").filter({ hasText: "Design" })).toHaveText(/Design: .*2026.*–/);
  await page.keyboard.press("ArrowRight");
  await expect.poll(() => calls.find((call) => call.method === "PATCH")?.body).toEqual({ start_date: "2026-11-03", due_date: "2026-11-04" });
  await expect(page.getByRole("status").filter({ hasText: "Design" })).toContainText("2026-11-03");
  await page.waitForTimeout(300); // the next key is ignored while the previous PATCH is still settling
  await page.keyboard.press("Shift+ArrowRight");
  await expect.poll(() => calls.filter((call) => call.method === "PATCH").at(-1)?.body).toEqual({ start_date: "2026-11-03", due_date: "2026-11-05" });
});

for (const scheme of ["light", "dark"] as const) {
  test(`has no axe violations (${scheme})`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: scheme });
    await page.addInitScript((dark) => { new MutationObserver(() => document.documentElement.classList.toggle("dark", dark)).observe(document.documentElement, { attributes: true, attributeFilter: ["class"] }); document.documentElement.classList.toggle("dark", dark); }, scheme === "dark");
    await mockGantt(page, { deps: [{ id: "e0000000-0000-4000-8000-000000000001", predecessor_id: kids[0].id, successor_id: kids[1].id, type: "FS", lag: 0 }] });
    await page.goto(`${path}?view=${ganttId}`);
    await expect(bar(page, design)).toBeVisible();
    await expect(page.locator("html")).toHaveClass(scheme === "dark" ? /dark/ : /^(?!.*dark).*$/);
    await page.getByRole("application", { name: "Gantt chart" }).focus();
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("ArrowDown");
    const found = await new AxeBuilder({ page }).include('[role="application"]').include('[aria-label^="Predecessors"]').analyze();
    expect(found.violations.map((v) => `${v.id}: ${v.nodes[0]?.html.slice(0, 100)}`)).toEqual([]);
  });
}

const depId = "e0000000-0000-4000-8000-000000000001";
const existing = { id: depId, predecessor_id: kids[0].id, successor_id: kids[1].id, type: "FS", lag: 0 };

async function select(page: import("@playwright/test").Page, downs: number) {
  await page.getByRole("application", { name: "Gantt chart" }).focus();
  for (let i = 0; i < downs; i++) await page.keyboard.press("ArrowDown");
}

test("links come from the server; type and removal go through the API", async ({ page }) => {
  const { calls } = await mockGantt(page, { deps: [existing] });
  await page.goto(`${path}?view=${ganttId}`);
  await expect(page.locator(".wx-line").first()).toBeAttached();
  await select(page, 3); // Launch, Design, Build
  const panel = page.getByRole("region", { name: "Predecessors of Build" });
  await expect(panel.locator("li").getByText("Design")).toBeVisible();
  await panel.getByRole("combobox", { name: "Link type from Design" }).selectOption("SS");
  await expect.poll(() => calls.find((call) => call.method === "PATCH" && call.path === `/dependencies/${depId}`)?.body).toEqual({ type: "SS" });
  await panel.getByRole("button", { name: "Remove link from Design" }).click();
  await expect.poll(() => calls.some((call) => call.method === "DELETE" && call.path === `/dependencies/${depId}`)).toBe(true);
  await expect(panel.getByText("No predecessors.")).toBeVisible();
});

test("adding a predecessor shifts the successor from the server's data, and a cycle is refused", async ({ page }) => {
  const { calls } = await mockGantt(page);
  await page.goto(`${path}?view=${ganttId}`);
  await select(page, 4); // ... Ship
  const panel = page.getByRole("region", { name: "Predecessors of Ship" });
  const before = (await bar(page, kids[2].id).boundingBox())!.x;
  await panel.getByRole("combobox", { name: "Predecessor" }).selectOption({ label: "Design" });
  await panel.getByRole("button", { name: "Add predecessor" }).click();
  await expect.poll(() => calls.find((call) => call.method === "POST" && call.path === `/items/${kids[2].id}/dependencies`)?.body)
    .toEqual({ predecessor_id: kids[0].id, type: "FS", lag: 0 });
  await expect.poll(async () => (await bar(page, kids[2].id).boundingBox())!.x).toBeGreaterThan(before); // the server moved Ship from Nov 6 to Nov 9
  await panel.getByRole("combobox", { name: "Predecessor" }).selectOption({ label: "Go live" });
  await panel.getByRole("button", { name: "Add predecessor" }).click();
  await expect(page.getByText("This link would create a cycle")).toBeVisible();
});

test("clicking one bar's finish handle and then another bar's start handle creates a finish-to-start link", async ({ page }) => {
  const { calls } = await mockGantt(page);
  await page.goto(`${path}?view=${ganttId}`);
  await bar(page, kids[0].id).hover();
  await page.locator(`.wx-bar[data-id=":${kids[0].id}"] .wx-link.wx-right`).click();
  await bar(page, kids[2].id).hover();
  await page.locator(`.wx-bar[data-id=":${kids[2].id}"] .wx-link.wx-left`).click();
  await expect.poll(() => calls.find((call) => call.method === "POST" && call.path === `/items/${kids[2].id}/dependencies`)?.body)
    .toEqual({ predecessor_id: kids[0].id, type: "FS", lag: 0 });
});

test("selected custom fields become Gantt grid columns", async ({ page }) => {
  const cost = { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", space_id: "x", type: "number", name: "Cost", position: 0, options: [] };
  await mockGantt(page, { fields: [cost], config: { columns: [`field:${cost.id}`] }, children: kids.map((kid, i) => ({ ...kid, fields: { [cost.id]: 10 * (i + 1) } })) });
  await page.goto(`${path}?view=${ganttId}`);
  await expect(page.getByRole("columnheader", { name: "Cost" })).toBeVisible();
  await expect(page.getByRole("gridcell", { name: "20" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Columns" })).toBeVisible();
});
