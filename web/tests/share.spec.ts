import { expect, test, type Page } from "@playwright/test";
import { me, problem } from "./mock-api";
import { extra, grace, marketing, mockSpaces, team } from "./spaces-mock";

const url = `/spaces/${marketing.id}`;
async function openShare(page: Page) {
  await page.goto(url);
  await page.getByRole("button", { name: "Share" }).click();
  const dialog = page.getByRole("dialog", { name: "Share “Marketing”" });
  await expect(dialog.getByText("Linus Pauling")).toBeVisible();
  return dialog;
}

test("lists current access; inherited rows are locked", async ({ page }) => {
  await mockSpaces(page, { extra: extra() });
  const dialog = await openShare(page);
  const inherited = dialog.getByRole("listitem").filter({ hasText: "Everyone in Sales" });
  await expect(inherited.getByText("Inherited")).toBeVisible();
  await expect(inherited.getByRole("combobox")).toHaveCount(0);
  await expect(inherited.getByRole("button")).toHaveCount(0);
  await expect(dialog.getByLabel("Role for Linus Pauling")).toHaveValue("editor");
  await expect(dialog.getByText("Can view only.")).toBeVisible();
});

test("search needs 2 characters, then adds a user with a role; save omits inherited rows", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: extra() });
  const dialog = await openShare(page);
  await dialog.getByLabel("Add people or groups").fill("g");
  await page.waitForTimeout(500);
  expect(calls.filter((c) => c.path === "/users")).toHaveLength(0);

  await dialog.getByLabel("Add people or groups").fill("gra");
  await expect(dialog.getByText("grace@example.test")).toBeVisible();
  expect(calls.find((c) => c.path === "/users")?.query.get("q")).toBe("gra");
  await dialog.getByLabel("Role for Grace Hopper").selectOption("editor");
  await dialog.getByRole("button", { name: "Add Grace Hopper" }).click();
  await expect(dialog.getByLabel("Role for Grace Hopper")).toHaveValue("editor");

  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog).toBeHidden();
  const put = calls.find((c) => c.method === "PUT");
  expect(put?.headers["if-match"]).toBe('"7"');
  expect(put?.body).toEqual({ shares: [
    { principal_type: "user", principal_id: me.id, role: "full" },
    { principal_type: "user", principal_id: "99999999-9999-4999-8999-999999999990", role: "editor" },
    { principal_type: "user", principal_id: grace.id, role: "editor" },
  ] });
});

test("groups are searchable, roles can change and rows can be removed", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: extra() });
  const dialog = await openShare(page);
  await dialog.getByLabel("Add people or groups").fill("design");
  await dialog.getByRole("button", { name: "Add Design team" }).click();
  await dialog.getByLabel("Role for Linus Pauling").selectOption("limited");
  await dialog.getByRole("button", { name: "Remove Linus Pauling" }).click();
  await expect(dialog.getByText("Linus Pauling")).toHaveCount(0);
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog).toBeHidden();
  expect(calls.find((c) => c.method === "PUT")?.body.shares).toEqual([
    { principal_type: "user", principal_id: me.id, role: "full" },
    { principal_type: "group", principal_id: team.id, role: "read" },
  ]);
});

test("Save stays disabled until something changes", async ({ page }) => {
  await mockSpaces(page, { extra: extra() });
  const dialog = await openShare(page);
  await expect(dialog.getByRole("button", { name: "Save" })).toBeDisabled();
  await dialog.getByLabel("Role for Linus Pauling").selectOption("full");
  await expect(dialog.getByRole("button", { name: "Save" })).toBeEnabled();
});

test("lowering my own access asks for confirmation first", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: extra() });
  const dialog = await openShare(page);
  await dialog.getByLabel("Role for Ada Lovelace").selectOption("read");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog.getByRole("alert")).toContainText("lowers or removes your own access");
  expect(calls.some((c) => c.method === "PUT")).toBe(false);

  await dialog.getByRole("button", { name: "Go back" }).click();
  await expect(dialog.getByRole("alert")).toHaveCount(0);
  await dialog.getByRole("button", { name: "Save" }).click();
  await dialog.getByRole("button", { name: "Save anyway" }).click();
  await expect(dialog).toBeHidden();
  expect(calls.find((c) => c.method === "PUT")?.body.shares[0]).toEqual({ principal_type: "user", principal_id: me.id, role: "read" });
});

test("removing my own row also asks, and so does losing access through a group", async ({ page }) => {
  const rows = [
    { principal_type: "group", principal_id: team.id, principal_name: "Design team", role: "full", inherited_from: null },
    { principal_type: "user", principal_id: "99999999-9999-4999-8999-999999999990", principal_name: "Linus Pauling", role: "editor", inherited_from: null },
  ];
  await mockSpaces(page, { extra: extra({
    [`GET /spaces/${marketing.id}/shares`]: { headers: { ETag: '"7"' }, body: { items: rows } },
    "GET /groups": { body: { items: [{ ...team, member_ids: [me.id] }] } },
  }) });
  const dialog = await openShare(page);
  await dialog.getByRole("button", { name: "Remove Design team" }).click();
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog.getByRole("alert")).toContainText("lowers or removes your own access");
});

test("server 422 is shown and the dialog stays open", async ({ page }) => {
  await mockSpaces(page, { extra: extra({ [`PUT /spaces/${marketing.id}/shares`]: problem(422, "role exceeds the user type ceiling", "role_exceeds_ceiling") }) });
  const dialog = await openShare(page);
  await dialog.getByLabel("Role for Linus Pauling").selectOption("full");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog.getByRole("alert")).toHaveText("role exceeds the user type ceiling");
  await expect(dialog).toBeVisible();
});

test("server 403 is shown too", async ({ page }) => {
  await mockSpaces(page, { extra: extra({ [`PUT /spaces/${marketing.id}/shares`]: problem(403, "you cannot grant a higher role", "forbidden") }) });
  const dialog = await openShare(page);
  await dialog.getByLabel("Role for Linus Pauling").selectOption("full");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog.getByRole("alert")).toHaveText("you cannot grant a higher role");
});

test("unknown email: admin sees a disabled invite button with a tooltip", async ({ page }) => {
  await mockSpaces(page, { extra: extra() });
  const dialog = await openShare(page);
  await dialog.getByLabel("Add people or groups").fill("nobody@example.test");
  await expect(dialog.getByText("No user with this address")).toBeVisible();
  const invite = dialog.getByRole("button", { name: "Invite as external user" });
  await expect(invite).toBeDisabled();
  await expect(dialog.getByTitle("Available after user management ships")).toBeVisible();
});

test("unknown email: non-admin gets no invite button", async ({ page }) => {
  await mockSpaces(page, { me: { ...me, user_type: "regular" }, extra: extra() });
  const dialog = await openShare(page);
  await dialog.getByLabel("Add people or groups").fill("nobody@example.test");
  await expect(dialog.getByText("No user with this address")).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Invite as external user" })).toHaveCount(0);
});

test("without the share capability the dialog is a read-only list", async ({ page }) => {
  await page.clock.install();
  const { spaces } = await mockSpaces(page, { extra: extra() });
  await openShare(page);
  // Access is revoked while the dialog is open: once the Space data is stale, refocusing the tab refetches it and `share` is gone.
  spaces[1] = { ...spaces[1], my_capabilities: marketing.my_capabilities.filter((c) => c !== "share") };
  await page.clock.fastForward(60_000);
  await page.evaluate(() => document.dispatchEvent(new Event("visibilitychange", { bubbles: true })));
  const dialog = page.getByRole("dialog", { name: "Share “Marketing”" });
  await expect(dialog.getByLabel("Add people or groups")).toHaveCount(0);
  await expect(dialog.getByRole("combobox")).toHaveCount(0);
  await expect(dialog.getByRole("button", { name: /^Remove/ })).toHaveCount(0);
  await expect(dialog.getByRole("button", { name: "Save" })).toHaveCount(0);
  await expect(dialog.getByText("Linus Pauling")).toBeVisible();
});
