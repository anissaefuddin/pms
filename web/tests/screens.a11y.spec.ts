import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { me, mockApi } from "./mock-api";

test.use({ colorScheme: "light" });

async function expectNoViolations(page: Page) {
  // Popups fade in; axe would otherwise measure contrast mid-animation.
  await page.evaluate(() => Promise.all(document.getAnimations().map((a) => a.finished.catch(() => undefined))));
  const { violations } = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
    .analyze();
  expect(violations).toEqual([]);
}

const invitation = { email: "grace@example.test", expires_at: "2030-01-01T00:00:00Z" };

test("forgot password has no violations", async ({ page }) => {
  await page.goto("/forgot-password");
  await expect(page.getByRole("heading", { name: "Reset your password" })).toBeVisible();
  await expectNoViolations(page);
});

test("reset password has no violations", async ({ page }) => {
  await page.goto("/reset-password?token=abc");
  await expect(page.getByRole("heading", { name: "Choose a new password" })).toBeVisible();
  await page.getByLabel("New password").fill("longenough1");
  await expect(page.getByText("Strength: fair")).toBeVisible();
  await expectNoViolations(page);
});

test("accept invitation has no violations", async ({ page }) => {
  await mockApi(page, { "GET /auth/invitations/good": { body: invitation } });
  await page.goto("/invitations/good");
  await expect(page.getByText("Invited as grace@example.test")).toBeVisible();
  await expectNoViolations(page);
});

test("expired invitation has no violations", async ({ page }) => {
  await mockApi(page, { "GET /auth/invitations/bad": { status: 410, body: { type: "about:blank", title: "gone", status: 410, code: "token_invalid", request_id: "r" } } });
  await page.goto("/invitations/bad");
  await expect(page.getByRole("alert")).toBeVisible();
  await expectNoViolations(page);
});

test("workspace shell has no violations", async ({ page }) => {
  await mockApi(page, { "GET /me": { body: me } });
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Your workspace is ready" })).toBeVisible();
  await expectNoViolations(page);
});

test("security page has no violations, also with the dialog and profile menu open", async ({ page }) => {
  await mockApi(page, { "GET /me": { body: me } });
  await page.goto("/settings/security");
  await expect(page.getByRole("heading", { name: "Sign-in & security", level: 2 })).toBeVisible();
  await expectNoViolations(page);

  await page.getByRole("button", { name: "Change password" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await expectNoViolations(page);
  await page.keyboard.press("Escape");

  await page.getByRole("button", { name: "Account menu" }).click();
  await expect(page.getByRole("menuitem", { name: "Log out" })).toBeVisible();
  await expectNoViolations(page);
});
