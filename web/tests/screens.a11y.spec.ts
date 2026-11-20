import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { me, mockApi } from "./mock-api";
import { extra, marketing, mockSpaces } from "./spaces-mock";

async function expectNoViolations(page: Page) {
  // Popups fade in; axe would otherwise measure contrast mid-animation.
  await page.evaluate(() => Promise.all(document.getAnimations().map((a) => a.finished.catch(() => undefined))));
  const { violations } = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
    .analyze();
  expect(violations).toEqual([]);
}

const invitation = { email: "grace@example.test", expires_at: "2030-01-01T00:00:00Z" };

for (const colorScheme of ["light", "dark"] as const) {
  test.describe(colorScheme, () => {
    test.use({ colorScheme });

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
      await expect(page.getByRole("heading", { name: "Sign-in & security", level: 1 })).toBeVisible();
      await expectNoViolations(page);

      await page.getByRole("button", { name: "Change password" }).click();
      await expect(page.getByRole("dialog")).toBeVisible();
      await expectNoViolations(page);
      await page.keyboard.press("Escape");

      await page.getByRole("button", { name: "Account menu" }).click();
      await expect(page.getByRole("menuitem", { name: "Log out" })).toBeVisible();
      await expectNoViolations(page);
    });

    test("space page has no violations, also with the actions menu and the dialogs open", async ({ page }) => {
      await mockSpaces(page, { extra: extra() });
      await page.goto(`/spaces/${marketing.id}`);
      await expect(page.getByRole("heading", { name: "Marketing", level: 1 })).toBeVisible();
      await expect(page.getByRole("navigation", { name: "Spaces" }).getByRole("link", { name: "Personal" })).toBeVisible();
      await expectNoViolations(page);

      await page.getByRole("button", { name: "Space actions" }).click();
      await expect(page.getByRole("menuitem", { name: "Rename" })).toBeVisible();
      await expectNoViolations(page);
      await page.getByRole("menuitem", { name: "Rename" }).click();
      await expect(page.getByRole("dialog", { name: "Rename Space" })).toBeVisible();
      await expectNoViolations(page);
      await page.keyboard.press("Escape");

      await page.getByRole("button", { name: "New Space" }).click();
      await expect(page.getByRole("dialog", { name: "New Space" })).toBeVisible();
      await expectNoViolations(page);
    });

    test("not-found Space has no violations", async ({ page }) => {
      await mockSpaces(page);
      await page.goto("/spaces/99999999-9999-4999-8999-999999999999");
      await expect(page.getByRole("heading", { name: "Space not found" })).toBeVisible();
      await expectNoViolations(page);
    });

    test("share dialog has no violations, with results, an unknown email and the self warning", async ({ page }) => {
      await mockSpaces(page, { extra: extra() });
      await page.goto(`/spaces/${marketing.id}`);
      await page.getByRole("button", { name: "Share" }).click();
      const dialog = page.getByRole("dialog", { name: "Share “Marketing”" });
      await expect(dialog.getByText("Linus Pauling")).toBeVisible();
      await expectNoViolations(page);

      await dialog.getByLabel("Add people or groups").fill("gra");
      await expect(dialog.getByText("grace@example.test")).toBeVisible();
      await expectNoViolations(page);

      await dialog.getByLabel("Add people or groups").fill("nobody@example.test");
      await expect(dialog.getByRole("button", { name: "Invite as external user" })).toBeEnabled();
      await expectNoViolations(page);

      await dialog.getByLabel("Role for Ada Lovelace").selectOption("read");
      await dialog.getByRole("button", { name: "Save" }).click();
      await expect(dialog.getByText("lowers or removes your own access")).toBeVisible();
      await expectNoViolations(page);
    });
  });
}
