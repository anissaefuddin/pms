import { expect, test } from "@playwright/test";
import { mockGantt, project, ganttId, path, bar, base, loc } from "./gantt-mock";

test.skip(!process.env.PMS_PERF, "Timing test: run alone with PMS_PERF=1 (parallel workers distort it)");

test("500 tasks render within 1.5 s at p95 (S2.4.2 measurement)", async ({ page }) => {
  const many = Array.from({ length: 500 }, (_, i) => ({ ...base, id: `d${String(i).padStart(7, "0")}-0000-4000-8000-000000000000`, seq: i + 10, type: "task", title: `Task ${i + 1}`,
    start_date: `2026-${i % 60 < 30 ? "11" : "12"}-${String((i % 28) + 1).padStart(2, "0")}`, due_date: `2027-01-${String((i % 28) + 1).padStart(2, "0")}`,
    progress_percent: i % 100, child_count: 0, locations: loc(project.id) }));
  await mockGantt(page, { children: many });
  const took: number[] = [];
  for (let run = 0; run < 10; run++) {
    const start = Date.now();
    await page.goto(`${path}?view=${ganttId}`);
    await expect(bar(page, many[0].id)).toBeVisible();
    took.push(Date.now() - start);
  }
  took.sort((a, b) => a - b);
  console.log(`gantt 500 tasks: p50=${took[5]}ms p95=${took[9]}ms max=${took.at(-1)}ms`);
  const rows = await page.locator(".wx-bar").count();
  console.log(`bars in DOM: ${rows}`); // virtualised: far fewer than 501
  expect(took[9]).toBeLessThan(1500);
});
