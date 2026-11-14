import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { me } from "./mock-api";

async function signedOut(page: Page, otp = true) {
  const calls: { path: string; body: any }[] = [];
  await page.route("**/api/v1/me", (route) => route.fulfill({ status: 401, json: { code: "unauthenticated", detail: "no session" } }));
  await page.route("**/api/v1/auth/sso/providers", (route) => route.fulfill({ json: { items: [], otp_login: otp } }));
  return calls;
}

test("the sign-in page offers a code only when an administrator turned it on", async ({ page }) => {
  await signedOut(page, false);
  await page.goto("/login");
  await expect(page.getByLabel("Email")).toBeVisible();
  await expect(page.getByRole("button", { name: "Email me a sign-in code" })).toHaveCount(0);
  await page.unroute("**/api/v1/auth/sso/providers");
  await page.route("**/api/v1/auth/sso/providers", (route) => route.fulfill({ json: { items: [], otp_login: true } }));
  await page.reload();
  await expect(page.getByRole("button", { name: "Email me a sign-in code" })).toBeVisible();
});

test("ask for a code, type it, and land in the workspace; a wrong code is explained; axe is clean", async ({ page }) => {
  const calls = await signedOut(page);
  let signedIn = false;
  await page.route("**/api/v1/auth/otp/request", (route) => { calls.push({ path: "request", body: route.request().postDataJSON() }); return route.fulfill({ status: 202, body: "" }); });
  await page.route("**/api/v1/auth/otp/verify", (route) => {
    const body = route.request().postDataJSON(); calls.push({ path: "verify", body });
    if (body.code !== "123456") return route.fulfill({ status: 401, json: { code: "unauthenticated", detail: "the code is not valid or has expired" } });
    signedIn = true;
    return route.fulfill({ json: me });
  });
  await page.route("**/api/v1/me", (route) => signedIn ? route.fulfill({ json: me }) : route.fulfill({ status: 401, json: { code: "unauthenticated", detail: "no session" } }));
  await page.route("**/api/v1/spaces*", (route) => route.fulfill({ json: { items: [] } }));
  await page.goto("/login");
  await page.getByRole("button", { name: "Email me a sign-in code" }).click();
  await expect(page.getByRole("heading", { name: "Sign in with a code" })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByLabel("Email").fill("ada@example.test");
  await page.getByRole("button", { name: "Send code" }).click();
  await expect(page.getByText("If an account exists for ada@example.test, we sent a six-digit code.")).toBeVisible();
  const code = page.getByLabel("Sign-in code");
  await expect(code).toHaveAttribute("autocomplete", "one-time-code");
  await expect(code).toHaveAttribute("inputmode", "numeric");
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await code.fill("000000");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("The code is not valid or has expired.");
  await code.fill("123456");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect.poll(() => new URL(page.url()).pathname).not.toBe("/login");
  expect(calls.filter((c) => c.path === "request")[0].body).toEqual({ email: "ada@example.test" });
  expect(calls.filter((c) => c.path === "verify").map((c) => c.body)).toEqual([{ email: "ada@example.test", code: "000000" }, { email: "ada@example.test", code: "123456" }]);
});

test("a person with two-factor goes on to the second step; resend and the rate limit are explained", async ({ page }) => {
  await signedOut(page);
  let requests = 0;
  await page.route("**/api/v1/auth/otp/request", (route) => { requests++; return route.fulfill({ status: 202, body: "" }); });
  await page.route("**/api/v1/auth/otp/verify", (route) => route.request().postDataJSON().code === "111111"
    ? route.fulfill({ status: 202, json: { two_factor_required: true, challenge: "c-1" } })
    : route.fulfill({ status: 429, headers: { "Retry-After": "30" }, json: { code: "rate_limited", detail: "slow down" } }));
  await page.goto("/login");
  await page.getByRole("button", { name: "Email me a sign-in code" }).click();
  await page.getByLabel("Email").fill("ada@example.test");
  await page.getByRole("button", { name: "Send code" }).click();
  await page.getByRole("button", { name: "Send a new code" }).click();
  await expect(page.getByRole("status").filter({ hasText: "a new code is on its way" })).toBeVisible();
  expect(requests).toBe(2);
  await page.getByLabel("Sign-in code").fill("222222");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("Try again in 30 seconds");
  await page.getByLabel("Sign-in code").fill("111111");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Two-factor authentication" })).toBeVisible();
});
