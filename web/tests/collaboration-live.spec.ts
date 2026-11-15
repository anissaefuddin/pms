import { expect, test, type Page } from "@playwright/test";

const { PMS_FIXTURE_API: api, PMS_FIXTURE_ORIGIN: origin, PMS_FIXTURE_ITEM: item,
  PMS_FIXTURE_FIRST_COOKIE: firstCookie, PMS_FIXTURE_SECOND_COOKIE: secondCookie } = process.env;

test.skip(!api || !origin || !item || !firstCookie || !secondCookie, "Requires the local Go/PostgreSQL fixture");

async function connect(page: Page, cookie: string, path = `/tasks/${item}`) {
  await page.context().addCookies([{ name: "pms_session", value: cookie.split("=")[1], url: test.info().project.use.baseURL! }]);
  await page.goto(path);
}

test("two sessions preserve a conflicting draft and reapply with the new ETag", async ({ page, browser }) => {
  const context = await browser.newContext({ baseURL: test.info().project.use.baseURL });
  try {
    const other = await context.newPage();
    await Promise.all([connect(page, firstCookie!), connect(other, secondCookie!)]);
    const firstTitle = page.getByRole("textbox", { name: "Name", exact: true });
    const secondTitle = other.getByRole("textbox", { name: "Name", exact: true });
    await expect(secondTitle).toHaveValue("Original title");
    await expect(other.locator('[data-live="true"]')).toBeAttached();
    await secondTitle.fill("Second session draft");
    await firstTitle.fill("First session edit");
    await firstTitle.press("Enter");
    await expect(page.getByRole("heading", { name: "First session edit" })).toBeVisible();

    await expect(other.getByRole("heading", { name: "First session edit" })).toBeVisible({ timeout: 3000 });
    await secondTitle.press("Enter");
    await expect(other.getByRole("alert")).toContainText("Review the latest version");
    await expect(secondTitle).toHaveValue("Second session draft");
    await expect(other.getByRole("heading", { name: "First session edit" })).toBeVisible();
    await other.getByRole("button", { name: "Reapply my changes" }).click();
    await expect(other.getByRole("heading", { name: "Second session draft" })).toBeVisible();
    await expect(secondTitle).toHaveValue("Second session draft");
    await page.reload();
    await expect(page.getByRole("heading", { name: "Second session draft" })).toBeVisible();
  } finally {
    await context.close();
  }
});

test("mentions update a second user's Inbox and attachments still upload", async ({ page, browser }) => {
  const context = await browser.newContext({ baseURL: test.info().project.use.baseURL });
  try {
    const recipient = await context.newPage();
    await Promise.all([connect(page, firstCookie!), connect(recipient, secondCookie!, "/inbox")]);
    await expect(recipient.locator('[data-live="true"]')).toBeAttached();
    await expect(recipient.getByText("You're all caught up")).toBeVisible();

    const comments = page.getByRole("region", { name: "Comments" });
    await comments.locator('[contenteditable="true"][aria-label="Comment"]').fill("Review this update");
    await comments.getByRole("button", { name: "Mention followers", exact: true }).click();
    await comments.getByRole("button", { name: "Post comment" }).click();
    await expect(comments.getByText("@Followers", { exact: true })).toBeVisible();
    await expect(recipient.getByRole("link", { name: "Inbox, 1 unread" })).toBeVisible({ timeout: 3000 });
    await recipient.getByRole("button", { name: /mentioned you in/ }).click();
    await expect(recipient.getByRole("link", { name: "Open item" })).toBeVisible(); // S-UIP.1.5: the notification opens in the Inbox detail pane
    await expect(recipient.getByRole("link", { name: "Inbox, 1 unread" })).toHaveCount(0);
    await expect(page.getByRole("link", { name: "Inbox, 1 unread" })).toHaveCount(0);

    await expect(page.getByText("Maximum file size: 1 MB")).toBeVisible();
    const chooserPromise = page.waitForEvent("filechooser");
    await page.getByRole("button", { name: "Choose file" }).click();
    const chooser = await chooserPromise;
    await chooser.setFiles({
      name: "notes.txt", mimeType: "text/plain", buffer: Buffer.from("Notes for tomorrow"),
    });
    await expect(page.getByRole("link", { name: "notes.txt" })).toBeVisible();
  } finally {
    await context.close();
  }
});

test("SSE reconnect resync refreshes a task in a second browser session", async ({ page, browser }) => {
  const context = await browser.newContext({ baseURL: test.info().project.use.baseURL });
  let resume!: () => void;
  const reconnect = new Promise<void>((resolve) => { resume = resolve; });
  try {
    let reconnectRequested = false;
    let streamRequests = 0;
    await context.route("**/api/v1/events", async (route) => {
      if (++streamRequests === 1) {
        // A truncated connection models a disconnect before Chrome receives a complete event.
        await route.abort("connectionreset");
        return;
      }
      reconnectRequested = true;
      await reconnect;
      await route.continue({
        headers: { ...await route.request().allHeaders(), "last-event-id": "previous-process:0" },
      });
    });
    const watcher = await context.newPage();
    await Promise.all([connect(page, firstCookie!), connect(watcher, secondCookie!)]);
    const watcherTitle = watcher.getByRole("textbox", { name: "Name", exact: true });
    await expect(watcherTitle).toBeVisible();
    const before = await watcherTitle.inputValue();
    await expect.poll(() => reconnectRequested).toBe(true);
    await expect(watcher.getByText("Connecting…", { exact: true })).toBeVisible();
    const title = page.getByRole("textbox", { name: "Name", exact: true });
    await title.fill("During disconnect");
    await title.press("Enter");
    await expect(page.getByRole("heading", { name: "During disconnect", exact: true })).toBeVisible();
    await expect(watcher.getByRole("heading", { name: before, exact: true })).toBeVisible();

    resume();
    await expect(watcher.getByRole("heading", { name: "During disconnect", exact: true })).toBeVisible({ timeout: 3000 });
    await expect(watcher.getByText("Live", { exact: true })).toBeVisible();

    await title.fill("After reconnect");
    await title.press("Enter");
    await expect(watcher.getByRole("heading", { name: "After reconnect", exact: true })).toBeVisible({ timeout: 3000 });
  } finally {
    resume();
    await context.close();
  }
});
