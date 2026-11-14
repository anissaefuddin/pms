import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { me, mockApi, problem, type Handler } from "./mock-api";

const challenge = { two_factor_required: true, challenge: "chal-1" };
const codes = ["K7QD2-M4XPA", "B2CD3-E4FGH", "C3DE4-F5GHJ", "D4EF5-G6HJK", "E5FG6-H7JKM", "F6GH7-J8KMN", "G7HJ8-K9MNP", "H8JK9-M2NPQ", "J9KM2-N3PQR", "K2MN3-P4QRS"];
const setup = { secret: "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP", otpauth_uri: "otpauth://totp/PMS:ada%40example.test?secret=JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP&issuer=PMS" };
const off = { configured: true, enabled: false, enabled_at: null, backup_codes_remaining: 0 };
const on = { configured: true, enabled: true, enabled_at: "2026-11-01T03:00:00Z", backup_codes_remaining: 10 };

async function expectNoViolations(page: Page) {
  await page.evaluate(() => Promise.all(document.getAnimations().map((a) => a.finished.catch(() => undefined))));
  const { violations } = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
  expect(violations).toEqual([]);
}

async function toSecondStep(page: Page, second: Handler, next = "") {
  let signedIn = false;
  const calls = await mockApi(page, {
    "POST /auth/login": { status: 202, body: challenge },
    "POST /auth/login/2fa": (body, req) => {
      const reply = typeof second === "function" ? second(body, req) : second;
      if ((reply.status ?? 200) === 200) signedIn = true;
      return reply;
    },
    "GET /me": () => (signedIn ? { body: me } : problem(401, "no session", "unauthenticated")),
  });
  await page.goto(`/login${next}`);
  await page.getByRole("textbox", { name: "Email" }).fill("ada@example.test");
  await page.getByLabel("Password").fill("correct horse");
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page.getByRole("heading", { name: "Two-factor authentication" })).toBeVisible();
  return calls;
}
const codeBox = (page: Page) => page.getByRole("textbox", { name: "Authentication code" });

test.describe("login second step", () => {
  test("202 challenge then a TOTP code signs in and honours next", async ({ page }) => {
    const calls = await toSecondStep(page, { body: me }, "?next=%2Fsettings%2Fsecurity");
    await expect(page.getByLabel("Password")).toHaveCount(0); // password form is gone
    await expect(codeBox(page)).toHaveAttribute("autocomplete", "one-time-code");
    await expect(codeBox(page)).toHaveAttribute("type", "text");
    await expect(codeBox(page)).not.toHaveAttribute("inputmode", "numeric");
    await expect(page.getByText("6-digit code from your authenticator app, or a backup code")).toBeVisible();
    await codeBox(page).fill("123 456");
    await page.getByRole("button", { name: "Verify" }).click();
    await expect(page).toHaveURL(/\/settings\/security$/);
    expect(calls.find((c) => c.path === "/auth/login/2fa")?.body).toEqual({ challenge: "chal-1", code: "123 456" });
  });

  test("a backup code is accepted", async ({ page }) => {
    const calls = await toSecondStep(page, (body) => (body.code === "k7qd2-m4xpa" ? { body: me } : problem(401, "invalid code", "unauthenticated")));
    await codeBox(page).fill("k7qd2-m4xpa");
    await page.getByRole("button", { name: "Verify" }).click();
    await expect(page.getByRole("button", { name: "Account menu" })).toBeVisible();
    expect(calls.filter((c) => c.path === "/auth/login/2fa")).toHaveLength(1);
  });

  test("a wrong code stays on the step with an error", async ({ page }) => {
    await toSecondStep(page, problem(401, "invalid code", "unauthenticated"));
    await codeBox(page).fill("000000");
    await page.getByRole("button", { name: "Verify" }).click();
    await expect(page.getByRole("alert")).toHaveText("That code is not valid. Try again.");
    await expect(page.getByRole("heading", { name: "Two-factor authentication" })).toBeVisible();
    await expect(page).toHaveURL(/\/login$/);
  });

  test("an expired challenge returns to the password step with the message", async ({ page }) => {
    await toSecondStep(page, problem(401, "the sign-in expired; enter your password again", "challenge_expired"));
    await codeBox(page).fill("123456");
    await page.getByRole("button", { name: "Verify" }).click();
    await expect(page.getByRole("heading", { name: "Sign in to your workspace" })).toBeVisible();
    await expect(page.getByRole("status")).toHaveText("The sign-in took too long. Enter your password again.");
    await expect(page.getByLabel("Password")).toHaveValue("");
  });

  test("rate limiting shows the retry message", async ({ page }) => {
    await toSecondStep(page, { ...problem(429, "too many attempts", "rate_limited"), headers: { "Retry-After": "120" } });
    await codeBox(page).fill("123456");
    await page.getByRole("button", { name: "Verify" }).click();
    await expect(page.getByRole("alert")).toHaveText("Too many attempts. Try again in 120 seconds.");
  });

  test("use a different account discards the challenge", async ({ page }) => {
    const calls = await toSecondStep(page, { body: me });
    await page.getByRole("button", { name: "Use a different account" }).click();
    await expect(page.getByRole("heading", { name: "Sign in to your workspace" })).toBeVisible();
    await expect(page.getByRole("textbox", { name: "Email" })).toHaveValue("");
    expect(calls.some((c) => c.path === "/auth/login/2fa")).toBe(false);
  });
});

/** Security page with a stateful 2FA backend. */
async function security(page: Page, start: typeof off | typeof on, extra: Record<string, Handler> = {}) {
  let status: { configured: boolean; enabled: boolean; enabled_at: string | null; backup_codes_remaining: number } = { ...start };
  const calls = await mockApi(page, {
    "GET /me": { body: me },
    "GET /me/2fa": () => ({ body: status }),
    "POST /me/2fa/setup": { body: setup },
    "POST /me/2fa/enable": (body) => {
      if (body.code !== "123456") return problem(422, "the code is not valid; check the time on your phone and try again", "validation_failed");
      status = { ...on };
      return { body: { enabled_at: on.enabled_at, backup_codes: codes } };
    },
    "POST /me/2fa/backup-codes": (body) => {
      if (body.password !== "correct horse" || body.code !== "654321") return problem(401, "password and a current code are required", "unauthenticated");
      status = { ...status, backup_codes_remaining: 10 };
      return { body: { backup_codes: codes.map((c) => c.replace("K", "Z")) } };
    },
    "POST /me/2fa/disable": (body) => {
      if (body.password !== "correct horse" || body.code !== "K7QD2-M4XPA") return problem(401, "password and a current code are required", "unauthenticated");
      status = { ...off };
      return { status: 204 };
    },
    ...extra,
  });
  await page.goto("/settings/security");
  await expect(page.getByRole("heading", { name: "Two-factor authentication" })).toBeVisible();
  return calls;
}
const section = (page: Page) => page.getByRole("region", { name: "Two-factor authentication" });

test.describe("security settings", () => {
  test("setup shows the QR code and secret, rejects a wrong code, then shows backup codes once", async ({ page }) => {
    await security(page, off);
    await page.getByRole("button", { name: "Set up two-factor authentication" }).click();
    const qr = page.getByRole("img", { name: "QR code for your authenticator app" });
    await expect(qr).toBeVisible();
    await expect(qr.locator("path")).toHaveAttribute("d", /^M/);
    await expect(page.getByText(setup.secret, { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Copy key" })).toBeVisible();

    await page.getByLabel("Code from your app").fill("000000");
    await page.getByRole("button", { name: "Turn on" }).click();
    await expect(section(page).getByRole("alert")).toHaveText("That code is not valid. Check the time on your phone and try again.");

    await page.getByLabel("Code from your app").fill("123456");
    await page.getByRole("button", { name: "Turn on" }).click();
    const list = page.getByRole("list", { name: "Backup codes" });
    await expect(list.getByRole("listitem")).toHaveCount(10);
    await expect(list).toContainText("K7QD2-M4XPA");
    await expect(page.getByText("Your other sessions were signed out; this one stays signed in.", { exact: false })).toBeVisible();
    await expect(page.getByRole("button", { name: "Copy codes" })).toBeVisible();
    const download = page.waitForEvent("download");
    await page.getByRole("link", { name: "Download .txt" }).click();
    expect((await download).suggestedFilename()).toBe("pms-backup-codes.txt");

    await page.getByRole("button", { name: "I saved these codes" }).click();
    await expect(page.getByRole("list", { name: "Backup codes" })).toHaveCount(0);
    await expect(section(page)).toContainText("On since 2026-11-01.");
    await expect(section(page)).toContainText("10 backup codes left.");
    await page.reload();
    await expect(section(page)).toContainText("10 backup codes left.");
    await expect(page.getByText("K7QD2-M4XPA")).toHaveCount(0);
  });

  test("cancel leaves setup", async ({ page }) => {
    await security(page, off);
    await page.getByRole("button", { name: "Set up two-factor authentication" }).click();
    await page.getByRole("button", { name: "Cancel" }).click();
    await expect(page.getByRole("button", { name: "Set up two-factor authentication" })).toBeVisible();
  });

  test("low backup codes warn", async ({ page }) => {
    await security(page, { ...on, backup_codes_remaining: 2 });
    await expect(section(page)).toContainText("2 backup codes left. Generate new backup codes soon.");
  });

  test("renew dialog shows the 401 inline then the new codes", async ({ page }) => {
    const calls = await security(page, { ...on, backup_codes_remaining: 3 });
    await page.getByRole("button", { name: "Generate new backup codes" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Current password").fill("correct horse");
    await dialog.getByRole("textbox", { name: "Authentication code" }).fill("000000");
    await dialog.getByRole("button", { name: "Generate codes" }).click();
    await expect(dialog.getByRole("alert")).toHaveText("The password or code is not correct.");
    await dialog.getByRole("textbox", { name: "Authentication code" }).fill("654321");
    await dialog.getByRole("button", { name: "Generate codes" }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(page.getByRole("list", { name: "Backup codes" })).toContainText("Z7QD2-M4XPA");
    expect(calls.filter((c) => c.path === "/me/2fa/backup-codes").at(-1)?.body).toEqual({ password: "correct horse", code: "654321" });
    await page.getByRole("button", { name: "I saved these codes" }).click();
    await expect(section(page)).toContainText("10 backup codes left.");
  });

  test("disable dialog turns 2FA off", async ({ page }) => {
    await security(page, on);
    await page.getByRole("button", { name: "Turn off two-factor authentication" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Current password").fill("wrong");
    await dialog.getByRole("textbox", { name: "Authentication code" }).fill("K7QD2-M4XPA");
    await dialog.getByRole("button", { name: "Turn off" }).click();
    await expect(dialog.getByRole("alert")).toHaveText("The password or code is not correct.");
    await dialog.getByLabel("Current password").fill("correct horse");
    await dialog.getByRole("button", { name: "Turn off" }).click();
    await expect(page.getByRole("button", { name: "Set up two-factor authentication" })).toBeVisible();
  });

  test("not configured explains and has no setup button", async ({ page }) => {
    await security(page, { ...off, configured: false });
    await expect(section(page)).toContainText("Set PMS_2FA_KEY on the server");
    await expect(page.getByRole("button", { name: "Set up two-factor authentication" })).toHaveCount(0);
  });
});

for (const colorScheme of ["light", "dark"] as const) {
  test.describe(`a11y ${colorScheme}`, () => {
    test.use({ colorScheme });

    test("login second step has no violations", async ({ page }) => {
      await toSecondStep(page, problem(401, "invalid code", "unauthenticated"));
      await codeBox(page).fill("000000");
      await page.getByRole("button", { name: "Verify" }).click();
      await expect(page.getByRole("alert")).toBeVisible();
      await expectNoViolations(page);
    });

    test("security 2FA states have no violations", async ({ page }) => {
      await security(page, off);
      await expect(page.getByRole("button", { name: "Set up two-factor authentication" })).toBeVisible();
      await expectNoViolations(page);
      await page.getByRole("button", { name: "Set up two-factor authentication" }).click();
      await expect(page.getByRole("img", { name: "QR code for your authenticator app" })).toBeVisible();
      await expectNoViolations(page);
      await page.getByLabel("Code from your app").fill("123456");
      await page.getByRole("button", { name: "Turn on" }).click();
      await expect(page.getByRole("list", { name: "Backup codes" })).toBeVisible();
      await expectNoViolations(page);
      await page.getByRole("button", { name: "I saved these codes" }).click();
      await expect(section(page)).toContainText("backup codes left");
      await expectNoViolations(page);
      await page.getByRole("button", { name: "Turn off two-factor authentication" }).click();
      await expect(page.getByRole("dialog")).toBeVisible();
      await expectNoViolations(page);
    });
  });
}
