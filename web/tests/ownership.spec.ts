import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me, problem } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

const ada = { id: me.id, name: "Ada Lovelace", email: "ada@example.test" };
const grace = { id: "55555555-5555-4555-8555-555555555555", name: "Grace Hopper", email: "grace@example.test" };

test("the owner hands ownership over after a confirmation, with the password; axe is clean", async ({ page }) => {
  let owner = ada;
  const { calls } = await mockSpaces(page, { extra: {
    "GET /account/ownership": () => ({ body: { owner, is_owner: owner.id === ada.id, candidates: owner.id === ada.id ? [grace] : [] } }),
    "POST /account/ownership": (body: any) => { owner = grace; return { body: { owner, is_owner: false, candidates: [] } }; },
  } });
  await page.goto("/settings/ownership");
  await expect(page.getByRole("heading", { name: "Account ownership", level: 2 })).toBeVisible();
  await expect(page.getByText("Ada Lovelace (ada@example.test)")).toBeVisible();
  await expect(page.getByText("You are the owner.")).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  const submit = page.getByRole("button", { name: "Transfer ownership" });
  await expect(submit).toBeDisabled();
  await page.getByLabel("New owner").selectOption(grace.id);
  await page.getByLabel("Your password").fill("correct horse");
  await submit.click();
  await expect(page.getByRole("dialog")).toContainText("Transfer ownership to Grace Hopper?");
  await expect(page.getByRole("dialog")).toContainText("You stay an administrator");
  await page.getByRole("button", { name: "Transfer", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(calls.find((c) => c.method === "POST" && c.path === "/account/ownership")!.body).toEqual({ new_owner_id: grace.id, password: "correct horse" });
  await expect(page.getByText("Grace Hopper (grace@example.test)")).toBeVisible();
  await expect(page.getByText("Only the owner can hand ownership over.")).toBeVisible();
  await expect(page.getByRole("form", { name: "Transfer ownership" })).toHaveCount(0);
});

test("a wrong password and a stale sign-in are explained in the dialog", async ({ page }) => {
  let reply = problem(401, "the password is not correct", "invalid_password");
  await mockSpaces(page, { extra: {
    "GET /account/ownership": { body: { owner: ada, is_owner: true, candidates: [grace] } },
    "POST /account/ownership": () => reply,
  } });
  await page.goto("/settings/ownership");
  await page.getByLabel("New owner").selectOption(grace.id);
  await page.getByRole("button", { name: "Transfer ownership" }).click();
  await page.getByRole("button", { name: "Transfer", exact: true }).click();
  await expect(page.getByRole("dialog").getByRole("alert")).toContainText("The password is not correct.");
  reply = problem(403, "sign in again", "recent_sign_in_required");
  await page.getByRole("button", { name: "Transfer", exact: true }).click();
  await expect(page.getByRole("dialog").getByRole("alert")).toContainText("Sign in again, then transfer ownership within 15 minutes.");
});

test("another administrator sees the owner but no form, an owner alone is told to add an administrator, and a regular user is told it is admin only", async ({ page }) => {
  await mockSpaces(page, { extra: { "GET /account/ownership": { body: { owner: grace, is_owner: false, candidates: [] } } } });
  await page.goto("/settings/ownership");
  await expect(page.getByText("Grace Hopper (grace@example.test)")).toBeVisible();
  await expect(page.getByRole("form", { name: "Transfer ownership" })).toHaveCount(0);
  await page.unrouteAll();
  await mockSpaces(page, { extra: { "GET /account/ownership": { body: { owner: ada, is_owner: true, candidates: [] } } } });
  await page.goto("/settings/ownership");
  await expect(page.getByText("There is no other active administrator")).toBeVisible();
  await page.unrouteAll();
  await mockSpaces(page, { me: { ...me, user_type: "regular", capabilities: { manage_users: false, create_space: true } } as typeof me });
  await page.goto("/settings/ownership");
  await expect(page.getByRole("alert")).toContainText("Only administrators can see who owns the account.");
  await expect(page.getByRole("link", { name: "Account ownership" })).toHaveCount(0);
});
