import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

// S-UIP2 lane C: Customize menu dialog, account menu header, settings entries, recycle bin header.
test("Customize sidebar unpins a shortcut, saves the pin order, and the sidebar follows", async ({ page }) => {
  let profile: Record<string, unknown> = { ...me, sidebar_hidden: [], sidebar_pinned: ["request_forms", "stream"] };
  const { calls } = await mockSpaces(page, { extra: {
    "GET /me": () => ({ body: profile }),
    "PATCH /me": (body) => { profile = { ...profile, ...body }; return { body: profile }; },
  } as never });
  await page.goto("/");
  const nav = page.getByRole("navigation", { name: "Main navigation" });
  await expect(nav.getByRole("button", { name: "Stream" })).toBeVisible();
  await page.getByRole("button", { name: "More", exact: true }).click();
  await page.getByRole("menuitem", { name: "Customize sidebar" }).click();
  const dialog = page.getByRole("dialog", { name: "Customize sidebar" });
  await dialog.getByRole("checkbox", { name: "Pin Stream" }).uncheck();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await dialog.getByRole("button", { name: "Done" }).click();
  await expect.poll(() => calls.find((call) => call.method === "PATCH" && call.path === "/me")?.body).toMatchObject({ sidebar_pinned: ["request_forms"] });
  await expect(nav.getByRole("button", { name: "Stream" })).toHaveCount(0);
});

test("the account menu shows who is signed in and links to appearance and email preferences", async ({ page }) => {
  await mockSpaces(page);
  await page.goto("/");
  await page.getByRole("button", { name: "Account menu" }).click();
  await expect(page.getByRole("menuitem", { name: "Workspace appearance" })).toBeVisible();
  await page.getByRole("menuitem", { name: "Workspace appearance" }).click();
  await expect(page).toHaveURL(/\/settings\/appearance$/);
  const settings = page.getByRole("navigation", { name: "Settings" });
  for (const name of ["Profile", "Work schedule", "Account", "Email preferences", "Workspace appearance"]) await expect(settings.getByRole("link", { name, exact: true })).toHaveCount(1);
  await settings.getByRole("link", { name: "Email preferences" }).click();
  await expect(page).toHaveURL(/\/settings\/email$/);
});

test("Recycle bin shows Empty bin beside the title", async ({ page }) => {
  const item = { id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", type: "task", title: "Old task", seq: 1, version: 1, locations: [], assignees: [], my_capabilities: [] };
  await mockSpaces(page, { extra: { "GET /recycle-bin": { body: { items: [item] } } } as never });
  await page.goto("/recycle-bin");
  await expect(page.getByRole("button", { name: "Empty Recycle bin" })).toBeVisible();
  await expect(page.getByText("Old task")).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});
