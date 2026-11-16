import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";

const longName = "Cross-team delivery and operational readiness coordination for international teams";
for (const [width, height] of [[1440, 900], [1280, 800], [768, 1024], [390, 844]]) {
  for (const route of ["workload", "portfolio"]) {
    test(`${route} fits at ${width}px and supports keyboard scrolling`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.clock.setFixedTime(new Date("2026-11-14T00:00:00Z"));
      await mockSpaces(page, { extra: {
        "GET /workload": { body: {
          people: [{ id: "person", name: longName, capacity: Array(14).fill(480), planned: Array(14).fill(600), booked: Array(14).fill(120) }],
          tasks: [], bookings: [], role_demand: [],
        } },
        "GET /resource-directory": { body: { roles: [], skills: [] } },
        "GET /utilization": { body: { items: [] } },
        "POST /portfolio/timeline": { body: {
          projects: [{ id: "p1", title: "Alpha" }],
          items: [{ project_id: "p1", item: { id: "i1", title: longName, start_date: "2026-11-14", due_date: "2026-11-18" } }],
          dependencies: [],
        } },
      } });
      await page.goto(route === "workload" ? "/workload" : "/portfolio?projects=p1");
      const scroll = page.getByRole("region", { name: route === "workload" ? "Daily workload columns" : "Portfolio timeline" });
      await expect(scroll).toBeVisible();
      if (route === "workload") {
        const bounds = await page.getByRole("combobox", { name: "Resource", exact: true }).boundingBox();
        expect(bounds).not.toBeNull();
        expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(width);
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(width);
      await page.getByRole(route === "workload" ? "checkbox" : "button", { name: route === "workload" ? "Group by week" : "Show timeline" }).focus();
      await page.keyboard.press("Tab");
      await expect(scroll).toBeFocused();
      if (await scroll.evaluate(node => node.scrollWidth > node.clientWidth)) {
        await page.keyboard.press("ArrowRight");
        await expect.poll(() => scroll.evaluate(node => node.scrollLeft)).toBeGreaterThan(0);
        await scroll.evaluate(node => { node.scrollLeft = 0; });
      }
      expect((await new AxeBuilder({ page }).include("main").analyze()).violations).toEqual([]);
      if (process.env.PMS_AUDIT_RETEST) {
        await page.screenshot({ path: `../docs/ui-audit-2-retest-evidence/${route}-${width}.png`, fullPage: true, animations: "disabled" });
      }
    });
  }
}
