import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const e = process.env;
test.skip(!e.PMS_FIXTURE_M9_ADMIN, "Requires the local Go/PostgreSQL M9 fixture");

test("E2E-PAR-04: a rule built in the UI acts on a new item and shows in the log", async ({ browser }) => {
  const baseURL = test.info().project.use.baseURL!;
  const context = await browser.newContext({ baseURL, colorScheme: "light" });
  await context.addCookies([{ name: "pms_session", value: e.PMS_FIXTURE_M9_ADMIN!.split("=")[1], url: baseURL }]);
  const page = await context.newPage();
  await page.goto(`/spaces/${e.PMS_FIXTURE_M9_WORK}/automation`);
  await page.getByRole("button", { name: "New rule" }).click();
  const dialog = page.getByRole("dialog", { name: "New automation rule" });
  await dialog.getByLabel("Rule name").fill("Welcome");
  await dialog.getByLabel("Trigger").selectOption("item.created");
  await dialog.getByLabel("Action 1 comment").fill("Welcome {item.title}");
  await expect(dialog).toHaveCSS("opacity", "1");
  expect((await new AxeBuilder({ page }).include('[role="dialog"]').analyze()).violations).toEqual([]);
  await dialog.getByRole("button", { name: "Publish" }).click();
  await expect(page.getByRole("cell", { name: "Welcome", exact: true })).toBeVisible();
  await page.getByRole("checkbox", { name: /Enable Welcome/ }).click();
  await expect(page.getByRole("checkbox", { name: /Enable Welcome/ })).toBeChecked();

  const created = await page.request.post("/api/v1/items", { data: { type: "task", title: "Crash on save", locations: [{ space_id: e.PMS_FIXTURE_M9_WORK }] } });
  expect(created.status()).toBe(201);
  const id = (await created.json()).id as string;
  await expect.poll(async () => ((await (await page.request.get(`/api/v1/items/${id}/comments`)).json()).items as unknown[]).length, { timeout: 15000 }).toBe(1);

  await page.getByRole("button", { name: /Log/ }).first().click();
  const log = page.getByRole("dialog");
  await expect(log).toContainText("Crash on save");
  await expect(log).toContainText("Done");
  await expect(log).toHaveCSS("opacity", "1");
  expect((await new AxeBuilder({ page }).include('[role="dialog"]').analyze()).violations).toEqual([]);
});

test("E2E-PAR-06: reviewer decisions use Any or All and stay in history", async ({ browser, baseURL }) => {
  const admin = await browser.newContext({ baseURL });
  const reviewer = await browser.newContext({ baseURL });
  try {
    for (const [context, cookie] of [[admin, e.PMS_FIXTURE_M9_ADMIN], [reviewer, e.PMS_FIXTURE_M9_REVIEWER]] as const) {
      await context.addCookies([{ name: "pms_session", value: cookie!.split("=")[1], url: baseURL! }]);
    }
    const page = await admin.newPage();
    const created = await page.request.post("/api/v1/items", { data: { type: "task", title: "Review launch", locations: [{ space_id: e.PMS_FIXTURE_M9_WORK }] } });
    expect(created.status()).toBe(201);
    const id = (await created.json()).id as string;
    await page.goto(`/tasks/${id}`);
    const section = page.getByRole("region", { name: "Approvals" });
    for (const policy of ["any", "all"]) {
      await section.getByRole("button", { name: "Add approval" }).click();
      await section.getByLabel("Description").fill(policy);
      await section.getByLabel("Approval policy").selectOption(policy);
      for (const name of ["Bea Reviewer", "Cal Reviewer"]) {
        await section.getByRole("searchbox", { name: /Approvers/ }).fill(name.split(" ")[0]);
        await section.getByRole("button", { name, exact: true }).click();
      }
      await section.getByRole("button", { name: "Add approval" }).click();
      await expect(section.getByText(policy, { exact: true })).toBeVisible();
    }
    const review = await reviewer.newPage();
    await review.goto(`/tasks/${id}`);
    for (const policy of ["any", "all"]) {
      const card = review.getByRole("region", { name: "Approvals" }).getByRole("article").filter({ has: review.getByText(policy, { exact: true }) });
      await card.getByRole("button", { name: "Approve", exact: true }).click();
      await expect(card).toContainText(policy === "any" ? "1/2 approved · Approved" : "1/2 approved · Pending");
    }
    await review.getByText("Decision history", { exact: true }).click();
    await expect(review.getByRole("list", { name: "Decision history" })).toContainText("Bea Reviewer");
  } finally {
    await Promise.all([admin.close(), reviewer.close()]);
  }
});
