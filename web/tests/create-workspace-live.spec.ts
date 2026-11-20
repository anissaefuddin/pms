import { expect, test } from "@playwright/test";

// R3 live: Workspace Quick Create. Global Create → Task → name only → Enter. The item lands in the user's Personal Space.
// Runs only from the Go fixture (TestBrowserWorkspaceQuickCreate) with a real backend and an admin session.
const e = process.env;
test.skip(!e.PMS_FIXTURE_WS_ADMIN, "Requires the local Go/PostgreSQL workspace create fixture");

test("Workspace: Create → Task, name only, Enter saves it in the Personal Space", async ({ browser }) => {
  const baseURL = test.info().project.use.baseURL!;
  const context = await browser.newContext({ baseURL, colorScheme: "light" });
  await context.addCookies([{ name: "pms_session", value: e.PMS_FIXTURE_WS_ADMIN!.split("=")[1], url: baseURL }]);
  const page = await context.newPage();
  await page.goto("/stream");

  await page.getByRole("button", { name: "Create", exact: true }).click();
  await page.getByRole("menuitem", { name: /^Task\b/ }).click();
  const name = page.locator('input[name="title"]');
  await expect(name).toBeFocused();
  await name.fill("Quick name only");
  await name.press("Enter");

  // Creation opens the new Task; its id is in the URL.
  await expect.poll(() => page.url()).toMatch(/task=|\/tasks\//);
  const id = new URL(page.url()).searchParams.get("task") ?? new URL(page.url()).pathname.split("/").pop()!;
  const me = await (await page.request.get("/api/v1/me")).json();
  const item = await (await page.request.get(`/api/v1/items/${id}`)).json();
  expect(item.title).toBe("Quick name only");
  expect(item.type).toBe("task");
  expect(item.locations[0].space_id).toBe(me.personal_space_id);
  expect(item.locations[0].parent_item_id ?? null).toBeNull();
  await context.close();
});
