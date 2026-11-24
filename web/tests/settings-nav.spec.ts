import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

// Three stories each added their own "Apps & Integrations" link to the settings menu; it must appear once, for everybody
// (the page shows personal tabs to all and administrator tabs to administrators).
test("the settings menu lists Apps & Integrations once, for an administrator and for a regular user", async ({ page }) => {
  await mockSpaces(page);
  await page.goto("/settings/profile");
  const nav = page.getByRole("navigation", { name: "Settings" });
  await expect(nav.getByRole("link", { name: "Apps & Integrations" })).toHaveCount(1);
  const links = await nav.getByRole("link").allTextContents();
  expect(links.filter((t) => t === "Apps & Integrations")).toHaveLength(1);
  expect(new Set(links).size).toBe(links.length); // no link of any kind is listed twice

  await page.unrouteAll();
  await mockSpaces(page, { me: { ...me, user_type: "regular", capabilities: { manage_users: false, create_space: true } } as typeof me });
  await page.goto("/settings/profile");
  // Integrations are administrator-only; a regular user gets the Developer tools entry instead.
  await expect(nav.getByRole("link", { name: "Apps & Integrations" })).toHaveCount(0);
  await expect(nav.getByRole("link", { name: "Developer tools" })).toBeVisible();
});
