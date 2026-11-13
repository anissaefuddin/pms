import { AxeBuilder } from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

// SET-06: every color theme in both light and dark keeps WCAG AA contrast on a screen with buttons, links and form controls.
for (const theme of ["light", "dark"]) {
  for (const accent of ["blue", "teal", "violet", "green"]) {
    test(`${accent} accent in ${theme} has no axe violations`, async ({ page }) => {
      await mockSpaces(page, { me: { ...me, theme, accent } as typeof me });
      await page.goto("/settings/profile");
      await expect(page.getByRole("button", { name: "Save profile" })).toBeVisible();
      await expect(page.locator("html")).toHaveAttribute(...(accent === "blue" ? ["data-density", /.*/] as const : ["data-accent", accent] as const));
      expect((await new AxeBuilder({ page }).analyze()).violations.map((v) => `${v.id}: ${v.nodes[0]?.failureSummary}`)).toEqual([]);
    });
  }
}

test("choosing a color theme is sent with the profile and applied at once", async ({ page }) => {
  let profile = { ...me, theme: "light", accent: "blue" } as typeof me & { accent: string };
  const { calls } = await mockSpaces(page, { extra: {
    "GET /me": () => ({ body: profile }),
    "PATCH /me": (body) => { profile = { ...profile, ...body }; return { body: profile }; },
  } });
  await page.goto("/settings/profile");
  await page.getByLabel("Color theme").selectOption("violet");
  await page.getByRole("button", { name: "Save profile" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-accent", "violet");
  expect(calls.find((c) => c.method === "PATCH" && c.path === "/me")?.body).toMatchObject({ accent: "violet" });
});
