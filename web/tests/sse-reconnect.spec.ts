import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";

// S7.1.2: after the event stream drops, the client reconnects and reloads what it shows within the 3 second budget.
test("a dropped event stream reconnects and refetches the visible data within 3 seconds", async ({ page }) => {
  await page.addInitScript(() => {
    const w = window as any;
    w.streams = [];
    w.EventSource = class {
      static CLOSED = 2;
      readyState = 0; onopen: any; onerror: any;
      constructor() { w.streams.push(this); setTimeout(() => { this.readyState = 1; this.onopen?.(); }, 20); }
      addEventListener() {}
      close() { this.readyState = 2; }
    };
  });
  let lists = 0;
  await mockSpaces(page, { extra: { "GET /reports": () => { lists++; return { body: { items: [] } }; } } });
  await page.goto("/reports");
  await expect.poll(() => lists).toBeGreaterThan(0);
  await expect.poll(() => page.evaluate(() => (window as any).streams.length)).toBe(1);
  const before = lists;
  const started = Date.now();
  await page.evaluate(() => { const s = (window as any).streams[0]; s.readyState = 2; s.onerror?.(); });
  await expect.poll(() => page.evaluate(() => (window as any).streams.length), { timeout: 3000 }).toBe(2);
  await expect.poll(() => lists, { timeout: 3000 }).toBeGreaterThan(before);
  expect(Date.now() - started).toBeLessThan(3000);
});
