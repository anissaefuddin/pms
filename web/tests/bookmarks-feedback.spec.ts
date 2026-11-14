import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { problem } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

test("bookmarks open in a new tab with noopener, can be added and removed, and bad URLs are refused; axe is clean", async ({ page }) => {
  const list = [{ id: "b1", url: "https://example.test/doc", title: "Docs", description: "", position: 0 }];
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /spaces/${marketing.id}/bookmarks`]: () => ({ body: { items: list } }),
    [`POST /spaces/${marketing.id}/bookmarks`]: (body: any) => {
      if (!/^https?:/.test(body.url)) return problem(422, "url must be an absolute http or https address", "validation_failed");
      list.push({ id: "b2", description: "", position: 1, ...body });
      return { status: 201, body: list[1] };
    },
    [`DELETE /spaces/${marketing.id}/bookmarks/b1`]: () => { list.shift(); return { status: 204 }; },
  } });
  await page.goto(`/spaces/${marketing.id}`);
  await page.getByRole("button", { name: "Bookmarks" }).click();
  const link = page.getByRole("link", { name: /Docs/ });
  await expect(link).toHaveAttribute("target", "_blank");
  await expect(link).toHaveAttribute("rel", /noopener/);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByLabel("URL").fill("https://example.test/b");
  await page.getByLabel("Title").fill("Second");
  await page.getByRole("button", { name: "Add bookmark" }).click();
  await expect(page.getByRole("link", { name: /Second/ })).toBeVisible();
  expect(calls.find((c) => c.method === "POST" && c.path.endsWith("/bookmarks"))?.body).toMatchObject({ url: "https://example.test/b", title: "Second" });
  await page.getByRole("button", { name: "Delete bookmark Docs" }).click();
  await expect(page.getByRole("link", { name: /Docs/ })).toHaveCount(0);
});

test("the profile menu sends feedback with the current page and shows the thanks; axe is clean", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: { "POST /feedback": { status: 201, body: { id: "f1" } } } });
  await page.goto(`/spaces/${marketing.id}`);
  await page.getByRole("button", { name: "Account menu" }).click();
  await page.getByRole("menuitem", { name: "Send feedback" }).click();
  await page.getByLabel("Subject").fill("Nice");
  await page.getByLabel("Rating").selectOption("4");
  await expect(page.locator("[data-starting-style], [data-ending-style]")).toHaveCount(0); // let the dialog finish fading in before axe measures contrast
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByRole("button", { name: "Send feedback" }).click();
  await expect(page.getByText("Your feedback was sent.")).toBeVisible();
  expect(calls.find((c) => c.path === "/feedback")?.body).toMatchObject({ subject: "Nice", rating: 4, page: `/spaces/${marketing.id}` });
});
