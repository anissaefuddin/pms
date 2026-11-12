import { expect, test } from "@playwright/test";

const { PMS_FIXTURE_PERF_PROJECT: project, PMS_FIXTURE_PERF_TABLE: table, PMS_FIXTURE_PERF_BOARD: board,
  PMS_FIXTURE_FIRST_COOKIE: cookie } = process.env;

test.skip(!project || !table || !board || !cookie, "Requires the 10,000-Item PostgreSQL fixture");

for (const mode of ["Table", "Board"] as const) {
  test(`${mode} p95 with a 500-Item project`, async ({ page, baseURL }) => {
    test.setTimeout(60_000);
    await page.context().addCookies([{ name: "pms_session", value: cookie!.split("=")[1], url: baseURL! }]);
    const url = `/items/${project}?view=${mode === "Table" ? table : board}`;
    const scope = mode === "Table" ? page.getByRole("table", { name: "Table" }) : page.getByRole("region", { name: "New", exact: true });
    const first = scope.getByRole("button", { name: "Pilot benchmark 00001", exact: true });
    const samples: number[] = [];
    for (let i = -2; i < 20; i++) {
      const start = performance.now();
      await page.goto(url, { waitUntil: "domcontentloaded" });
      await expect(first).toBeVisible();
      if (i >= 0) samples.push(performance.now() - start);
    }
    samples.sort((a, b) => a - b);
    const p95 = samples[Math.ceil(samples.length * .95) - 1];
    console.log(`${mode}: 500 tasks in project, 10,000 tasks in database, 20 samples after 2 warmups, p95=${p95.toFixed(1)}ms, max=${samples.at(-1)!.toFixed(1)}ms`);
    expect(p95).toBeLessThan(1500);
    if (mode === "Board") await expect(scope.getByRole("listitem")).toHaveCount(500);
    else expect(await scope.getByRole("row").count()).toBeLessThan(60);
  });
}
