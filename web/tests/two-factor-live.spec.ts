import { createHmac } from "node:crypto";
import { expect, test, type Page } from "@playwright/test";

const email = process.env.PMS_FIXTURE_2FA_EMAIL!;
const password = process.env.PMS_FIXTURE_2FA_PASSWORD!;
test.skip(!email, "Requires the local Go/PostgreSQL 2FA fixture");

/** RFC 6238 TOTP (HMAC-SHA1, 30 s, 6 digits) for the current step plus `offset`. */
function totp(secret: string, offset = 0) {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  let bits = "";
  for (const ch of secret.replace(/=+$/, "").toUpperCase()) bits += alphabet.indexOf(ch).toString(2).padStart(5, "0");
  const key = Buffer.from(bits.match(/.{8}/g)!.map((b) => parseInt(b, 2)));
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000) + offset));
  const mac = createHmac("sha1", key).update(counter).digest();
  const at = mac[19] & 0xf;
  return String((mac.readUInt32BE(at) & 0x7fffffff) % 1_000_000).padStart(6, "0");
}

async function passwordStep(page: Page) {
  await page.goto("/login");
  await page.getByRole("textbox", { name: "Email" }).fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Log in" }).click();
}
async function logout(page: Page) {
  await page.getByRole("button", { name: "Account menu" }).click();
  await page.getByRole("menuitem", { name: "Log out" }).click();
  await expect(page.getByRole("heading", { name: "Sign in to your workspace" })).toBeVisible();
}
const signedIn = (page: Page) => expect(page.getByRole("button", { name: "Account menu" })).toBeVisible();

test("2FA: set up in settings, sign in with a code, reject a wrong one, turn it off", async ({ page }) => {
  await passwordStep(page);
  await signedIn(page);

  await page.goto("/settings/security");
  await page.getByRole("button", { name: "Set up two-factor authentication" }).click();
  await expect(page.getByRole("img", { name: "QR code for your authenticator app" })).toBeVisible();
  const secret = (await page.getByTestId("two-factor-secret").textContent())!.trim();
  expect(secret).toMatch(/^[A-Z2-7]+=*$/);
  await page.getByLabel("Code from your app").fill(totp(secret));
  await page.getByRole("button", { name: "Turn on" }).click();
  const list = page.getByRole("list", { name: "Backup codes" });
  await expect(list.getByRole("listitem")).toHaveCount(10);
  const backup = await list.getByRole("listitem").allTextContents();
  await page.getByRole("button", { name: "I saved these codes" }).click();
  await expect(page.getByRole("region", { name: "Two-factor authentication" })).toContainText("10 backup codes left.");
  expect((await (await page.request.get("/api/v1/me/2fa")).json()).enabled).toBe(true);

  await logout(page);
  await passwordStep(page);
  await expect(page.getByRole("heading", { name: "Two-factor authentication" })).toBeVisible();
  const code = page.getByRole("textbox", { name: "Authentication code" });
  await code.fill("000000");
  await page.getByRole("button", { name: "Verify" }).click();
  await expect(page.getByRole("alert")).toHaveText("That code is not valid. Try again.");
  // The enable step's code is spent; the next step is inside the server's one-step drift window.
  await code.fill(totp(secret, 1));
  await page.getByRole("button", { name: "Verify" }).click();
  await signedIn(page);

  await page.goto("/settings/security");
  await page.getByRole("button", { name: "Turn off two-factor authentication" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Current password").fill(password);
  await dialog.getByRole("textbox", { name: "Authentication code" }).fill(backup[0].toLowerCase());
  await dialog.getByRole("button", { name: "Turn off" }).click();
  await expect(page.getByRole("button", { name: "Set up two-factor authentication" })).toBeVisible();
  expect((await (await page.request.get("/api/v1/me/2fa")).json()).enabled).toBe(false);

  await logout(page);
  await passwordStep(page);
  await signedIn(page);
  await expect(page.getByRole("heading", { name: "Two-factor authentication" })).toHaveCount(0);
});
