import { AxeBuilder } from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

const admin = { ...me, onboarding_pending: true, account_name: "Pilot Co" };

test("first-run wizard creates the workspace once, then stays away", async ({ page }) => {
  let pending = true;
  const { calls } = await mockSpaces(page, { extra: {
    "GET /me": () => ({ body: { ...admin, onboarding_pending: pending } }),
    "PATCH /account": { body: { name: "Acme" } },
    "POST /spaces": { status: 201, body: { ...marketing, id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", name: "Acme" } },
    "POST /items": (body: Record<string, unknown>) => ({ status: 201, body: { id: body.type === "project" ? "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" : "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ...body } }),
    "POST /users/invitations": { status: 201, body: me },
    "POST /onboarding/complete": () => { pending = false; return { status: 204 }; },
  } });
  await page.goto("/");
  const dialog = page.getByRole("dialog", { name: "Set up your workspace" });
  await expect(dialog.getByText("Step 1 of 4")).toBeVisible();
  await expect(dialog).toHaveCSS("opacity", "1"); // axe must not measure contrast mid fade-in
  expect((await new AxeBuilder({ page }).include('[role="dialog"]').analyze()).violations).toEqual([]);
  await dialog.getByLabel("Organization name").fill("Acme");
  await dialog.getByRole("button", { name: "Next" }).click();
  await expect(dialog.getByRole("button", { name: "Next" })).toBeDisabled();
  await dialog.getByLabel("Project name").fill("Launch");
  await dialog.getByLabel("Task 1").fill("Write brief");
  await dialog.getByLabel("Task 2").fill("Book venue");
  await dialog.getByRole("button", { name: "Next" }).click();
  await dialog.getByLabel("Board: columns by status").check();
  await dialog.getByRole("button", { name: "Create project" }).click();
  await dialog.getByLabel("Email addresses").fill("a@example.test\nb@example.test");
  await dialog.getByRole("button", { name: "Finish" }).click();
  await expect(dialog).toBeHidden();
  const made = calls.filter((call) => call.method === "POST" && call.path === "/items");
  expect(made.map((call) => (call.body as { title: string }).title)).toEqual(["Launch", "Write brief", "Book venue"]);
  expect(made[0].body).toMatchObject({ type: "project", default_view_type: "board" });
  expect(calls.filter((call) => call.path === "/users/invitations")).toHaveLength(2);
  expect(calls.filter((call) => call.path === "/onboarding/complete")).toHaveLength(1);
  await page.reload();
  await expect(page.getByRole("dialog", { name: "Set up your workspace" })).toHaveCount(0);
});

test("skipping saves the choice without creating anything", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: {
    "GET /me": { body: admin },
    "POST /onboarding/complete": { status: 204 },
  } });
  await page.goto("/");
  await page.getByRole("button", { name: "Skip setup" }).click();
  await expect.poll(() => calls.some((call) => call.path === "/onboarding/complete")).toBe(true);
  expect(calls.some((call) => call.method === "POST" && (call.path === "/spaces" || call.path === "/items"))).toBe(false);
});

test("DR-04: one Skip closes the wizard even when /me still reports it pending", async ({ page }) => {
  await mockSpaces(page, { extra: {
    "GET /me": { body: admin }, // never flips: a stale or lagging read must not bring the wizard back
    "POST /onboarding/complete": { status: 204 },
  } });
  await page.goto("/");
  await page.getByRole("button", { name: "Skip setup" }).click();
  await expect(page.getByRole("dialog", { name: "Set up your workspace" })).toBeHidden();
  await page.getByRole("link", { name: "Settings" }).first().click();
  await expect(page.getByRole("dialog", { name: "Set up your workspace" })).toHaveCount(0);
});
