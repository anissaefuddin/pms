import { test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";

// S-UIP.4.1: evidence screenshots at 1440 and 390 px, written to docs/ui-parity-evidence. Off by default so CI never depends on pixels:
//   UIP_SHOTS=1 npx playwright test tests/ui-parity-shots.spec.ts
const routes: Record<string, string> = { home: "/", "my-todo": "/my-todo", inbox: "/inbox", dashboards: "/dashboards", profile: "/settings/profile" };
test.skip(!process.env.UIP_SHOTS, "evidence run only");
for (const width of [1440, 390]) {
  for (const [name, route] of Object.entries(routes)) {
    test(`${name} at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: width === 1440 ? 900 : 844 });
      await mockSpaces(page);
      await page.goto(route);
      await page.waitForLoadState("networkidle");
      await page.screenshot({ path: `../docs/ui-parity-evidence/${name}-${width}.png` });
    });
  }
}
