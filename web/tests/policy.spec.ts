import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me, problem } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

const policy = (over: Record<string, unknown> = {}) => ({ ip_allowlist_enabled: false, ip_allowlist: [], otp_login_enabled: false, version: 1, your_ip: "203.0.113.7", override_active: false, ...over });

test("admin edits the IP allowlist with If-Match, can add their own address, and axe is clean", async ({ page }) => {
  let current = policy();
  const { calls } = await mockSpaces(page, { extra: {
    "GET /access-policy": () => ({ headers: { ETag: `"${current.version}"` }, body: current }),
    "PUT /access-policy": (body: any) => { current = policy({ ...body, version: current.version + 1 }); return { headers: { ETag: `"${current.version}"` }, body: current }; },
  } });
  await page.goto("/settings/policy");
  await expect(page.getByRole("heading", { name: "Access policy", level: 1 })).toBeVisible();
  await expect(page.getByText("Your address as the server sees it: 203.0.113.7")).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  await page.getByRole("button", { name: "Add my address" }).click();
  await expect(page.getByLabel("Allowed addresses and networks")).toHaveValue("203.0.113.7");
  await expect(page.getByRole("button", { name: "Add my address" })).toBeDisabled();
  await page.getByLabel("Allowed addresses and networks").fill("203.0.113.7\n198.51.100.0/24, 2001:db8::/32");
  await page.getByLabel("Only allow the addresses below").check();
  await page.getByLabel("Allow signing in with an emailed code").check();
  await page.getByRole("button", { name: "Save policy" }).click();
  await expect.poll(() => calls.some((c) => c.method === "PUT")).toBe(true);
  const put = calls.find((c) => c.method === "PUT")!;
  expect(put.headers["if-match"]).toBe('"1"');
  expect(put.body).toEqual({ ip_allowlist_enabled: true, ip_allowlist: ["203.0.113.7", "198.51.100.0/24", "2001:db8::/32"], otp_login_enabled: true });
  await expect(page.getByLabel("Only allow the addresses below")).toBeChecked();
});

test("a lockout is explained, a stale policy is reported, and the override is shown", async ({ page }) => {
  await mockSpaces(page, { extra: {
    "GET /access-policy": () => ({ headers: { ETag: '"1"' }, body: policy({ override_active: true }) }),
    "PUT /access-policy": problem(422, "your own address 203.0.113.7 is not in the list; add it first so you are not locked out", "self_lockout"),
  } });
  await page.goto("/settings/policy");
  await expect(page.getByRole("status").filter({ hasText: "allowlist override" })).toBeVisible();
  await page.getByRole("button", { name: "Save policy" }).click();
  await expect(page.getByRole("alert")).toContainText("not locked out");
  await page.unrouteAll();
  await mockSpaces(page, { extra: {
    "GET /access-policy": () => ({ headers: { ETag: '"1"' }, body: policy() }),
    "PUT /access-policy": problem(409, "changed", "version_conflict"),
  } });
  await page.goto("/settings/policy");
  await page.getByRole("button", { name: "Save policy" }).click();
  await expect(page.getByRole("alert")).toContainText("Someone changed the policy.");
});

test("a regular user is told the policy is admin only and gets no menu entry", async ({ page }) => {
  await mockSpaces(page, { me: { ...me, user_type: "regular", capabilities: { manage_users: false, create_space: true } } as typeof me });
  await page.goto("/settings/policy");
  await expect(page.getByRole("alert")).toContainText("Only administrators can manage the access policy.");
  await expect(page.getByRole("link", { name: "Access policy" })).toHaveCount(0);
});

test("an administrator locks and unlocks a space from its menu; others see the lock but no menu entry", async ({ page }) => {
  const { calls } = await mockSpaces(page);
  await page.goto(`/spaces/${marketing.id}`);
  await expect(page.getByText("Locked space")).toHaveCount(0);
  await page.getByRole("button", { name: "Space actions" }).click();
  await page.getByRole("menuitem", { name: "Lock space" }).click();
  await expect(page.getByText("Locked space")).toBeVisible();
  const patch = calls.find((c) => c.method === "PATCH")!;
  expect(patch.headers["if-match"]).toBe('"4"');
  expect(patch.body).toEqual({ locked: true });
  await page.getByRole("button", { name: "Space actions" }).click();
  await page.getByRole("menuitem", { name: "Unlock space" }).click();
  await expect(page.getByText("Locked space")).toHaveCount(0);

  await page.unrouteAll();
  await mockSpaces(page, { me: { ...me, user_type: "regular" } as typeof me, spaces: [{ ...marketing, locked: true } as typeof marketing] });
  await page.goto(`/spaces/${marketing.id}`);
  await expect(page.getByText("Locked space")).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByRole("button", { name: "Space actions" }).click();
  await expect(page.getByRole("menuitem", { name: /Lock space|Unlock space/ })).toHaveCount(0);
});
