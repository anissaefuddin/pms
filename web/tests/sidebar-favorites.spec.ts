import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const project = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const loc = { space_id: marketing.id, space_name: "Marketing" };
const projectItem = { id: project, type: "project", title: "Roadmap", seq: 1, version: 1, locations: [loc], assignees: [], my_capabilities: ALL,
  status: { id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", name: "New", color: "slate", group: "active" }, created_by: { id: me.id, name: "Ada" } };
const common = {
  [`GET /spaces/${marketing.id}/items`]: { body: { items: [projectItem] } },
  [`GET /items/${project}`]: { headers: { ETag: '"1"' }, body: projectItem },
  [`GET /items/${project}/children`]: { body: { items: [] } },
};

test("Favorites lists starred items; after opening one it is offered under More", async ({ page }) => {
  await mockSpaces(page, { extra: { ...common, "GET /stars": { body: { items: [{ id: project, title: "Roadmap", type: "project" }] } } } as never });
  await page.goto("/");
  await page.getByRole("link", { name: "Roadmap" }).click();
  await expect(page).toHaveURL(`/items/${project}`);
  await page.getByRole("button", { name: "More", exact: true }).click();
  await expect(page.getByRole("menuitem", { name: "Roadmap" })).toBeVisible();
});

test("the row menu stars a tree item", async ({ page }) => {
  let starred = 0;
  await mockSpaces(page, { extra: { ...common, "GET /stars": { body: { items: [] } } } as never });
  await page.route(`**/api/v1/items/${project}/star`, (route) => { starred += route.request().method() === "PUT" ? 1 : 0; return route.fulfill({ status: 204, body: "" }); });
  await page.goto(`/spaces/${marketing.id}`);
  const tree = page.getByRole("navigation", { name: "Spaces" });
  await tree.getByRole("button", { name: "Expand Marketing" }).click();
  await expect(tree.getByRole("link", { name: "Roadmap" })).toBeVisible();
  await page.getByRole("button", { name: "More actions for Roadmap" }).click();
  await page.getByRole("menuitem", { name: "Star this item" }).click();
  await expect.poll(() => starred).toBe(1);
});
