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
    await expect(recipient.getByText("Live", { exact: true })).toBeVisible();

    const destination = `/spaces/${space}`;
    await page.goto(`/login?next=${encodeURIComponent(destination)}`);
    await page.getByRole("textbox", { name: "Email" }).fill(email!);
    await page.getByLabel("Password").fill(password!);
    await page.getByRole("button", { name: "Log in" }).click();
    await expect(page).toHaveURL(destination);
    await expect(page.getByRole("heading", { name: "Browser collaboration" })).toBeVisible();

    await page.locator("header").first().getByRole("button", { name: "New item" }).click();
    const project = page.getByRole("dialog", { name: "Create item" });
    await project.getByLabel("Name").fill("Pilot launch");
    await project.getByRole("radio", { name: "Board", exact: true }).check();
    await project.getByRole("button", { name: "Create item" }).click();
    await expect(page.getByRole("heading", { name: "Pilot launch" })).toBeVisible();

    const initial = page.getByRole("region", { name: "New", exact: true });
    const done = page.getByRole("region", { name: "Completed", exact: true });
    await initial.getByRole("button", { name: "Add item" }).click();
    const task = page.getByRole("dialog", { name: "Add subitem" });
    await task.getByLabel("Name").fill("Prepare pilot checklist");
    await task.getByRole("button", { name: "Create item" }).click();

    await expect(page.getByRole("heading", { name: "Prepare pilot checklist" })).toBeVisible();
    await page.goBack();
    await expect(page.getByRole("heading", { name: "Pilot launch" })).toBeVisible();
    const card = initial.getByRole("listitem").filter({ hasText: "Prepare pilot checklist" });
    await card.dragTo(done);
    await expect(done.getByRole("button", { name: "Prepare pilot checklist" })).toBeVisible();
    await expect(initial.getByRole("button", { name: "Prepare pilot checklist" })).toHaveCount(0);

    await done.getByRole("button", { name: "Prepare pilot checklist" }).click();
    const comments = page.getByRole("region", { name: "Comments" });
    const editor = comments.locator('[contenteditable="true"][aria-label="Comment"]');
    await editor.fill("Please review");
    await comments.getByRole("textbox", { name: "Find a user or group to mention" }).fill("Pilot Recipient");
    await comments.getByRole("button", { name: "Pilot Recipient" }).click();
    await editor.press("Control+Enter");
    await expect(recipient.getByRole("button", { name: /mentioned you in Prepare pilot checklist/ })).toBeVisible({ timeout: 3000 });
  } finally {
    await context.close();
  }
});
