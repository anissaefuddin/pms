import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { grace, mockSpaces } from "./spaces-mock";

test("an admin adds an hourly rate for a person; the page passes axe", async ({ page }) => {
  let rates = [{ id: "r1", name: "Design", user_id: null, role_id: "r0", rate_minor: 4000, effective_from: "2026-01-01" }];
  const { calls } = await mockSpaces(page, { me: { ...me, user_type: "admin" }, extra: {
    "GET /rates": () => ({ body: { items: rates } }),
    "GET /resource-directory": { body: { roles: [{ id: "r0", name: "Design" }], skills: [], users: [{ id: grace.id, name: "Grace Hopper", roles: [], skills: [] }] } },
    "POST /rates": (body: any) => { rates = [...rates, { id: "r2", name: "Grace Hopper", user_id: body.user_id, role_id: null, rate_minor: body.rate_minor, effective_from: body.effective_from }]; return { status: 201, body: { id: "r2" } }; },
  } });
  await page.goto("/settings/rates");
  await expect(page.getByRole("heading", { name: "Hourly rates" })).toBeVisible();
  await expect(page.getByRole("row", { name: /Design 40\.00 2026-01-01/ })).toBeVisible();
  await page.getByLabel("Person or job role").selectOption({ label: "Grace Hopper" });
  await page.getByLabel("Rate per hour").fill("55.5");
  await page.getByLabel("Effective from").fill("2026-03-01");
  await page.getByRole("button", { name: "Add rate" }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === "/rates")?.body).toEqual({ user_id: grace.id, rate_minor: 5550, effective_from: "2026-03-01" });
  await expect(page.getByRole("row", { name: /Grace Hopper 55\.50 2026-03-01/ })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});
