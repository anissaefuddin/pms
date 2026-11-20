import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

// Pinned navigation order can be changed with Alt+Arrow (keyboard) and by dragging; the new order is saved at once.
test("Alt+ArrowDown moves a pinned entry and saves the new order", async ({ page }) => {
  let profile = { ...me, sidebar_pinned: ["request_forms", "stream", "reports"], sidebar_hidden: [] as string[] };
  const { calls } = await mockSpaces(page, { extra: {
    "GET /me": () => ({ body: profile }),
    "PATCH /me": (body) => { profile = { ...profile, ...body }; return { body: profile }; },
  } });
  await page.goto("/settings/appearance");
  const stream = page.getByRole("listitem", { name: /^Stream, position 2 of 3/ });
  await stream.focus();
  await page.keyboard.press("Alt+ArrowDown");
  await expect.poll(() => calls.filter((c) => c.method === "PATCH" && c.path === "/me").at(-1)?.body)
    .toMatchObject({ sidebar_pinned: ["request_forms", "reports", "stream"] });
});
