import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me, problem } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

const d1 = "d0000000-0000-4000-8000-000000000001";
const w1 = "e0000000-0000-4000-8000-000000000001";
const w2 = "e0000000-0000-4000-8000-000000000002";
const query = { reporting_on: "tasks", metric: "count", filters: {} };

test("a visitor enters the password, sees the board, and money widgets are hidden", async ({ page }) => {
  const calls = (await mockSpaces(page, { extra: {
    "POST /public/dashboards/tok": (body: any) => body?.password === "open sesame" ? { body: { name: "Client view", layout: "columns", widgets: [
      { id: w1, name: "Open tasks", type: "indicator", config: { description: "Not done yet", query }, data: { kind: "series", result: { total: 12, rows: [{ group: { key: "", label: "" }, break: null, value: 12 }] } } },
      { id: w2, name: "Budget", type: "indicator", config: { query: { ...query, metric: "sum_field" } }, masked: true },
    ] } } : problem(401, "a password is required", "unauthorized"),
  } })).calls;
  await page.goto("/d/tok");
  await expect(page.getByRole("heading", { name: "This dashboard needs a password" })).toBeVisible();
  await page.getByLabel("Password (optional)").fill("nope");
  await page.getByRole("button", { name: "Open dashboard" }).click();
  await expect(page.getByRole("alert")).toContainText("That password is not right.");
  await page.getByLabel("Password (optional)").fill("open sesame");
  await page.getByRole("button", { name: "Open dashboard" }).click();
  await expect(page.getByRole("heading", { name: "Client view", level: 1 })).toBeVisible();
  await expect(page.getByRole("region", { name: "Open tasks" })).toContainText("Not done yet");
  await expect(page.getByRole("region", { name: "Open tasks" })).toContainText("12");
  await expect(page.getByRole("region", { name: "Budget" })).toContainText("Hidden: this widget shows financial data.");
  expect(calls.filter((c) => c.path === "/public/dashboards/tok").length).toBeGreaterThan(2);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("an unknown or expired link says it is unavailable", async ({ page }) => {
  await mockSpaces(page, { extra: { "POST /public/dashboards/gone": problem(404, "not found", "not_found") } });
  await page.goto("/d/gone");
  await expect(page.getByRole("heading", { name: "This dashboard is unavailable" })).toBeVisible();
});

test("an editor turns the public link on, sees it once, and turns it off", async ({ page }) => {
  let on = false;
  const board = { id: d1, owner_id: me.id, space_id: null, name: "Team", layout: "columns", version: 1, widgets: [] };
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /dashboards/${d1}`]: { body: board },
    [`GET /dashboards/${d1}/public-link`]: () => ({ body: { enabled: on, expires_at: on ? "2026-12-31T00:00:00Z" : null, has_password: on } }),
    [`PUT /dashboards/${d1}/public-link`]: () => { on = true; return { body: { enabled: true, expires_at: "2026-12-31T00:00:00Z", has_password: true, token: "T".repeat(43) } }; },
    [`DELETE /dashboards/${d1}/public-link`]: () => { on = false; return { status: 204 }; },
  } });
  await page.goto(`/dashboards/${d1}`);
  await page.getByRole("button", { name: "Share" }).click(); // S-UIP.2.4: Share opens the Public link dialog
  const dialog = page.getByRole("dialog", { name: "Public link" });
  await expect(dialog.getByRole("status").first()).toContainText("The link is off.");
  await dialog.getByLabel("Password (optional)").fill("correct horse battery");
  await dialog.getByRole("button", { name: "Turn on" }).click();
  await expect(dialog.getByLabel("Public link address")).toHaveValue(/\/d\/T{43}$/);
  expect(calls.find((c) => c.method === "PUT")?.body).toMatchObject({ password: "correct horse battery" });
  await expect(dialog).toHaveCSS("opacity", "1");
  expect((await new AxeBuilder({ page }).include('[role="dialog"]').analyze()).violations).toEqual([]);
  await dialog.getByRole("button", { name: "Turn off" }).click();
  await expect(dialog.getByRole("status").first()).toContainText("The link is off.");
});
