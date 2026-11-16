import { expect, test } from "@playwright/test";
import { me, mockApi, problem } from "./mock-api";

test.describe("login", () => {
  test("success lands on the app", async ({ page }) => {
    let signedIn = false;
    const calls = await mockApi(page, {
      "POST /auth/login": () => { signedIn = true; return { body: me }; },
      "GET /me": () => (signedIn ? { body: me } : problem(401, "no session", "unauthenticated")),
    });
    await page.goto("/login");
    await page.getByRole("textbox", { name: "Email" }).fill("ada@example.test");
    await page.getByLabel("Password").fill("correct horse");
    await page.getByRole("button", { name: "Log in" }).click();
    await expect(page.getByRole("heading", { name: "Your workspace is ready" })).toBeVisible();
    expect(calls.find((c) => c.path === "/auth/login")?.body).toEqual({ email: "ada@example.test", password: "correct horse" });
  });

  test("wrong credentials show one generic message", async ({ page }) => {
    await mockApi(page, { "POST /auth/login": problem(401, "invalid credentials", "unauthenticated") });
    await page.goto("/login");
    await page.getByRole("textbox", { name: "Email" }).fill("ada@example.test");
    await page.getByLabel("Password").fill("wrong-password");
    await page.getByRole("button", { name: "Log in" }).click();
    await expect(page.getByRole("alert")).toHaveText("Incorrect email or password");
    await expect(page).toHaveURL(/\/login$/);
  });

  test("password can be revealed", async ({ page }) => {
    await page.goto("/login");
    const password = page.getByLabel("Password");
    await expect(password).toHaveAttribute("type", "password");
    await page.getByRole("button", { name: "Show" }).click();
    await expect(password).toHaveAttribute("type", "text");
  });

  for (const [next, expected] of [["/settings/security", "/settings/security"], ["//evil.test", "/"]] as const) {
    test(`next=${next} ${expected === "/" ? "is ignored" : "is honored"}`, async ({ page, baseURL }) => {
      let signedIn = false;
      await mockApi(page, {
        "POST /auth/login": () => { signedIn = true; return { body: me }; },
        "GET /me": () => (signedIn ? { body: me } : problem(401, "no session", "unauthenticated")),
      });
      await page.goto(`/login?next=${encodeURIComponent(next)}`);
      await page.getByRole("textbox", { name: "Email" }).fill("ada@example.test");
      await page.getByLabel("Password").fill("correct horse");
      await page.getByRole("button", { name: "Log in" }).click();
      await expect(page.getByRole("button", { name: "Account menu" })).toBeVisible();
      const url = new URL(page.url());
      expect(url.origin).toBe(new URL(baseURL!).origin);
      expect(url.pathname).toBe(expected);
    });
  }

  test("logged-out visit to /settings redirects to login with next", async ({ page }) => {
    await mockApi(page, { "GET /me": problem(401, "no session", "unauthenticated") });
    await page.goto("/settings");
    await expect(page).toHaveURL(/\/login\?next=%2Fsettings$/);
    await expect(page.getByRole("heading", { name: "Sign in to your workspace" })).toBeVisible();
  });
});

test.describe("forgot password", () => {
  test("shows the same confirmation and posts the email", async ({ page }) => {
    const calls = await mockApi(page, { "POST /auth/password/forgot": { status: 202 } });
    await page.goto("/forgot-password");
    await page.getByRole("textbox", { name: "Email" }).fill("nobody@example.test");
    await page.getByRole("button", { name: "Send reset link" }).click();
    await expect(page.getByText("If an account exists, we sent a link.")).toBeVisible();
    expect(calls.find((c) => c.path === "/auth/password/forgot")?.body).toEqual({ email: "nobody@example.test" });
    await expect(page.getByRole("link", { name: "Back to sign in" })).toBeVisible();
  });
});

test.describe("reset password", () => {
  test("rejects mismatched and short passwords without calling the API", async ({ page }) => {
    const calls = await mockApi(page, {});
    await page.goto("/reset-password?token=abc");
    await page.getByLabel("New password").fill("longenough1");
    await page.getByLabel("Confirm password").fill("different123");
    await page.getByRole("button", { name: "Save password" }).click();
    await expect(page.getByRole("alert")).toHaveText("Passwords do not match");

    await page.getByLabel("New password").fill("short");
    await page.getByLabel("Confirm password").fill("short");
    await page.getByRole("button", { name: "Save password" }).click();
    await expect(page.getByRole("alert")).toHaveText("Password must be at least 8 characters");
    expect(calls).toEqual([]);
  });

  test("valid reset sends the token and returns to login", async ({ page }) => {
    const calls = await mockApi(page, { "POST /auth/password/reset": { status: 204 } });
    await page.goto("/reset-password?token=abc");
    await page.getByLabel("New password").fill("longenough1");
    await page.getByLabel("Confirm password").fill("longenough1");
    await page.getByRole("button", { name: "Save password" }).click();
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByText("Password updated")).toBeVisible();
    expect(calls[0].body).toEqual({ token: "abc", password: "longenough1" });
  });

  test("expired token offers a new link", async ({ page }) => {
    await mockApi(page, { "POST /auth/password/reset": problem(410, "token expired", "token_invalid") });
    await page.goto("/reset-password?token=old");
    await page.getByLabel("New password").fill("longenough1");
    await page.getByLabel("Confirm password").fill("longenough1");
    await page.getByRole("button", { name: "Save password" }).click();
    await expect(page.getByText("This reset link is invalid or has expired.")).toBeVisible();
    await page.getByRole("link", { name: "Request a new link" }).click();
    await expect(page).toHaveURL(/\/forgot-password$/);
  });
});

test.describe("accept invitation", () => {
  test("invalid token shows the expired message", async ({ page }) => {
    await mockApi(page, { "GET /auth/invitations/bad": problem(410, "gone", "token_invalid") });
    await page.goto("/invitations/bad");
    await expect(page.getByRole("alert")).toHaveText("This invitation has expired. Ask your admin to send a new one.");
  });

  test("valid token submits the profile and password", async ({ page }) => {
    const calls = await mockApi(page, {
      "GET /auth/invitations/good": { body: { email: "grace@example.test", expires_at: "2030-01-01T00:00:00Z" } },
      "POST /auth/invitations/accept": { body: me },
      "GET /me": { body: me },
    });
    await page.goto("/invitations/good");
    await expect(page.getByText("Invited as grace@example.test")).toBeVisible();
    await page.getByLabel("First name").fill("Grace");
    await page.getByLabel("Last name").fill("Hopper");
    await page.getByLabel("Password", { exact: true }).fill("longenough1");
    await page.getByLabel("Confirm password").fill("longenough1");

    await page.getByRole("button", { name: "Get started" }).click();
    await expect(page.getByRole("alert")).toHaveText("You must accept the terms to continue");

    await page.getByRole("checkbox", { name: /Terms of Service/ }).check();
    await page.getByRole("button", { name: "Get started" }).click();
    await expect(page.getByRole("heading", { name: "Your workspace is ready" })).toBeVisible();
    expect(calls.find((c) => c.path === "/auth/invitations/accept")?.body).toEqual({
      token: "good", first_name: "Grace", last_name: "Hopper", password: "longenough1", accept_terms: true,
    });
  });
});

test.describe("app shell", () => {
  test("security page changes the password in a dialog", async ({ page }) => {
    const calls = await mockApi(page, { "GET /me": { body: me }, "POST /auth/password/change": { status: 204 } });
    await page.goto("/settings/security");
    await expect(page.getByRole("heading", { name: "Sign-in & security", level: 1 })).toBeVisible();
    await page.getByRole("button", { name: "Change password" }).click();
    const dialog = page.getByRole("dialog", { name: "Change password" });
    await expect(dialog).toBeVisible();

    await dialog.getByLabel("Current password").fill("old-password");
    await dialog.getByLabel("New password").fill("longenough1");
    await dialog.getByLabel("Confirm password").fill("nope");
    await dialog.getByRole("button", { name: "Change password" }).click();
    await expect(dialog.getByRole("alert")).toHaveText("Passwords do not match");

    await dialog.getByLabel("Confirm password").fill("longenough1");
    await dialog.getByRole("button", { name: "Change password" }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByText("Password updated. Other sessions were signed out.")).toBeVisible();
    expect(calls.find((c) => c.path === "/auth/password/change")?.body).toEqual({ current_password: "old-password", new_password: "longenough1" });
  });

  test("wrong current password keeps the dialog open with the server message", async ({ page }) => {
    await mockApi(page, { "GET /me": { body: me }, "POST /auth/password/change": problem(401, "current password is incorrect", "unauthenticated") });
    await page.goto("/settings/security");
    await page.getByRole("button", { name: "Change password" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Current password").fill("bad");
    await dialog.getByLabel("New password").fill("longenough1");
    await dialog.getByLabel("Confirm password").fill("longenough1");
    await dialog.getByRole("button", { name: "Change password" }).click();
    await expect(dialog.getByRole("alert")).toHaveText("current password is incorrect");
  });

  test("/settings opens the security page when signed in", async ({ page }) => {
    await mockApi(page, { "GET /me": { body: me } });
    await page.goto("/settings");
    await expect(page).toHaveURL(/\/settings\/security$/);
  });

  test("profile menu logs out", async ({ page }) => {
    const calls = await mockApi(page, { "GET /me": { body: me }, "POST /auth/logout": { status: 204 } });
    await page.goto("/");
    await page.getByRole("button", { name: "Account menu" }).click();
    await page.getByRole("menuitem", { name: "Log out" }).click();
    await expect(page).toHaveURL(/\/login$/);
    expect(calls.some((c) => c.method === "POST" && c.path === "/auth/logout")).toBe(true);
  });
});

test("expired guard preserves the destination and login shows Retry-After", async ({ page }) => {
  await mockApi(page, {
    "GET /me": problem(401, "session expired", "unauthenticated"),
    "POST /auth/login": { ...problem(429, "try later", "rate_limited"), headers: { "Retry-After": "30" } },
  });
  await page.goto("/settings/security");
  await expect(page).toHaveURL(/\/login\?next=%2Fsettings%2Fsecurity&reason=expired$/);
  await expect(page.getByText("Your session expired. Log in to continue.")).toBeVisible();
  await page.getByRole("textbox", { name: "Email" }).fill("ada@example.test");
  await page.getByLabel("Password").fill("correct horse");
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page.getByRole("alert")).toHaveText("Too many attempts. Try again in 30 seconds.");
});
