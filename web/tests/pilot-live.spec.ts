import { expect, test } from "@playwright/test";

const { PMS_FIXTURE_SPACE: space, PMS_FIXTURE_LOGIN_EMAIL: email, PMS_FIXTURE_LOGIN_PASSWORD: password,
  PMS_FIXTURE_SECOND_COOKIE: secondCookie } = process.env;

test.skip(!space || !email || !password || !secondCookie, "Requires the local PostgreSQL browser fixture");

test("pilot workflow logs in, creates project and task, drags status, and notifies another user", async ({ page, browser, baseURL }) => {
  const context = await browser.newContext({ baseURL });
  try {
    await context.addCookies([{ name: "pms_session", value: secondCookie!.split("=")[1], url: baseURL! }]);
    const recipient = await context.newPage();
    await recipient.goto("/inbox");
    await expect(recipient.locator('[data-live="true"]')).toBeAttached();

    const destination = `/spaces/${space}`;
    await page.goto(`/login?next=${encodeURIComponent(destination)}`);
    await page.getByRole("textbox", { name: "Email" }).fill(email!);
    await page.getByLabel("Password").fill(password!);
    await page.getByRole("button", { name: "Log in" }).click();
    await expect(page).toHaveURL(destination);
    await expect(page.getByRole("heading", { name: "Browser collaboration" })).toBeVisible();

    await page.getByRole("main").getByRole("button", { name: "New item" }).click();
    const project = page.getByRole("dialog", { name: "Create item" });
    await project.getByLabel("Name").fill("Pilot launch");
    await project.locator("label").filter({ hasText: /^Board/ }).click(); // click the card: the sr-only radio sits under its illustration
    await project.getByRole("button", { name: "Create item" }).click();
    await expect(page.getByRole("heading", { name: "Pilot launch" })).toBeVisible();

    const initial = page.getByRole("region", { name: "New", exact: true });
    const done = page.getByRole("region", { name: "Completed", exact: true });
    await page.getByRole("button", { name: "Add task", exact: true }).click(); // the empty board has no column to add to yet
    const task = page.getByRole("dialog", { name: "Add subitem" });
    await task.getByLabel("Name").fill("Prepare pilot checklist");
    await task.getByRole("button", { name: "Create item" }).click();

    await expect(page.getByRole("heading", { name: "Prepare pilot checklist", exact: true })).toBeVisible();
    await page.goBack();
    await expect(page.getByRole("heading", { name: "Pilot launch" })).toBeVisible();
    const card = initial.getByRole("listitem").filter({ hasText: "Prepare pilot checklist" });
    await expect(async () => { // the drop target can render after the first drag on a slow runner
      await card.dragTo(done);
      await expect(done.getByRole("button", { name: "Prepare pilot checklist", exact: true })).toBeVisible({ timeout: 3000 });
    }).toPass({ timeout: 20000 });
    await expect(initial.getByRole("button", { name: "Prepare pilot checklist", exact: true })).toHaveCount(0);

    await done.getByRole("button", { name: "Prepare pilot checklist", exact: true }).click();
    const comments = page.getByRole("region", { name: "Comments" });
    const editor = comments.locator('[contenteditable="true"][aria-label="Comment"]');
    await editor.fill("Please review");
    await comments.getByRole("textbox", { name: "Find a user or group to mention" }).fill("Pilot Recipient");
    await comments.getByRole("button", { name: "Pilot Recipient" }).click();
    // The mention popover must close before the send shortcut, or Ctrl+Enter can land in the popover and drop the comment.
    await expect(comments.getByRole("button", { name: "Pilot Recipient" })).toHaveCount(0);
    const posted = page.waitForResponse((response) => response.request().method() === "POST" && /\/comments$/.test(new URL(response.url()).pathname));
    await comments.getByRole("button", { name: "Post comment", exact: true }).click();
    expect((await posted).status()).toBe(201);
    // Delivery first (the API lists the mention for the recipient), then the inbox row that shows it.
    await expect.poll(async () => JSON.stringify((await (await recipient.request.get("/api/v1/notifications")).json()).items ?? []), { timeout: 15000 })
      .toContain("Prepare pilot checklist");
    await expect(recipient.getByRole("button", { name: /mentioned you in Prepare pilot checklist/ })).toBeVisible({ timeout: 15000 });
  } finally {
    await context.close();
  }
});
