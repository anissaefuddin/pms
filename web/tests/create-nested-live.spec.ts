import { expect, test } from "@playwright/test";

// Live: Task → Subtask → Nested Subtask created through the Table and the Item Detail panel, read back after reload.
// Runs only from the Go fixture (TestBrowserCreateNested) which provides a real backend and an admin session.
const e = process.env;
test.skip(!e.PMS_FIXTURE_NESTED_ADMIN, "Requires the local Go/PostgreSQL nested-create fixture");

test("Task → Subtask → Nested Subtask via Table and Item Detail stays consistent after reload", async ({ browser }) => {
  const baseURL = test.info().project.use.baseURL!;
  const context = await browser.newContext({ baseURL, colorScheme: "light" });
  await context.addCookies([{ name: "pms_session", value: e.PMS_FIXTURE_NESTED_ADMIN!.split("=")[1], url: baseURL }]);
  const page = await context.newPage();
  await page.goto(`/spaces/${e.PMS_FIXTURE_NESTED_SPACE}`);

  // 1. Table: Add subitem on the Project row creates a Task under it.
  const projectRow = page.getByRole("row", { name: /Nested project/ });
  await projectRow.getByRole("button", { name: /^(Move|Actions)/ }).click();
  await page.getByRole("menuitem", { name: "Add subitem" }).click();
  const dialog = page.getByRole("dialog", { name: "Add subitem" });
  await dialog.locator('input[name="title"]').fill("Level one");
  await dialog.getByRole("button", { name: "Create item" }).click();
  await expect(page.getByRole("dialog", { name: "Add subitem" })).toHaveCount(0);

  // 2. Item Detail: the new Task gets a Subtask from the panel.
  await expect(page).toHaveURL(/task=/);
  await page.getByRole("button", { name: "Add subitem", exact: true }).click();
  // The panel opens the created item, so the URL changes once creation finishes: remember it before creating, then wait.
  const levelOneUrl = page.url();
  await page.getByRole("dialog", { name: "Add subitem" }).locator('input[name="title"]').fill("Level two");
  await page.getByRole("dialog", { name: "Add subitem" }).getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => page.url()).not.toBe(levelOneUrl);

  // 3. Nested Subtask under the Subtask, again from the panel.
  const levelTwoUrl = page.url();
  await page.getByRole("button", { name: "Add subitem", exact: true }).click();
  await page.getByRole("dialog", { name: "Add subitem" }).locator('input[name="title"]').fill("Level three");
  await page.getByRole("dialog", { name: "Add subitem" }).getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => page.url()).not.toBe(levelTwoUrl);
  const leaf = page.url();

  // 4. Read back after reload: the leaf is still open under the same chain.
  await page.reload();
  await expect(page).toHaveURL(leaf);
  // Check the chain through the API as the browser session sees it: leaf → Level two → Level one → Project.
  const item = async (id: string) => (await page.request.get(`/api/v1/items/${id}`)).json();
  const leafItem = await item(new URL(leaf).searchParams.get("task")!);
  expect(leafItem.title).toBe("Level three");
  const two = await item(leafItem.locations[0].parent_item_id);
  expect(two.title).toBe("Level two");
  const one = await item(two.locations[0].parent_item_id);
  expect(one.title).toBe("Level one");
  const project = await item(one.locations[0].parent_item_id);
  expect(project.title).toBe("Nested project");
  expect(project.type).toBe("project");
  await context.close();
});
