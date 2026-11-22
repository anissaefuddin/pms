import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

// SET-01: the person uploads a photo on Settings > Profile; it replaces the initials in the menu and page, and can be removed.
const png = Buffer.from("89504e470d0a1a0a0000000d49484452", "hex");

test("upload and remove a profile photo", async ({ page }) => {
  let photo = 0;
  await mockSpaces(page);
  await page.route("**/api/v1/me/avatar", async (route) => {
    if (route.request().method() === "PUT") photo = 1700000000000;
    if (route.request().method() === "DELETE") photo = 0;
    if (route.request().method() === "GET") return route.fulfill({ contentType: "image/png", body: png });
    await route.fulfill({ contentType: "application/json", body: JSON.stringify({ ...me, avatar_version: photo }) });
  });
  await page.goto("/settings/profile");
  await expect(page.locator("img[src*='/api/v1/me/avatar']")).toHaveCount(0);

  await page.locator("input[type=file]").setInputFiles({ name: "me.png", mimeType: "image/png", buffer: png });
  await expect(page.locator("img[src*='/api/v1/me/avatar?v=1700000000000']").first()).toBeVisible();

  await page.getByRole("button", { name: "Remove photo" }).click();
  await expect(page.locator("img[src*='/api/v1/me/avatar']")).toHaveCount(0);
});

test("a file that is not an image is refused before upload", async ({ page }) => {
  let uploads = 0;
  await mockSpaces(page);
  await page.route("**/api/v1/me/avatar", async (route) => {
    uploads++;
    await route.fulfill({ contentType: "application/json", body: JSON.stringify(me) });
  });
  await page.goto("/settings/profile");
  await page.locator("input[type=file]").setInputFiles({ name: "notes.txt", mimeType: "text/plain", buffer: Buffer.from("hi") });
  await expect(page.getByText("Use a PNG, JPEG or WebP image.")).toBeVisible();
  expect(uploads).toBe(0);
});
