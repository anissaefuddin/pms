import { expect, test } from "@playwright/test";
import { me, problem } from "./mock-api";
import { extra, grace, marketing, mockSpaces, ops, personal } from "./spaces-mock";

const sidebar = (page: import("@playwright/test").Page) => page.getByRole("navigation", { name: "Spaces" });

test.describe("sidebar", () => {
  test("Find filters the Spaces list by name", async ({ page }) => {
    await mockSpaces(page, { spaces: [marketing, personal, ops] });
    await page.goto("/");
    await sidebar(page).getByRole("button", { name: "Find a Space" }).click();
    await sidebar(page).getByRole("searchbox", { name: "Find a Space" }).fill("OPER");
    await expect(sidebar(page).getByRole("link", { name: "Operations" })).toBeVisible();
    await expect(sidebar(page).getByRole("link", { name: "Marketing" })).toHaveCount(0);
  });

  test("lists Personal first and navigates to a Space", async ({ page }) => {
    await mockSpaces(page, { spaces: [marketing, personal, ops] });
    await page.goto("/");
    const links = sidebar(page).getByRole("link");
    await expect(links).toHaveText(["Personal", "Marketing", "Operations", "Shared with me", "Blueprints"]);
    await sidebar(page).getByRole("link", { name: "Marketing" }).click();
    await expect(page).toHaveURL(new RegExp(`/spaces/${marketing.id}$`));
    await expect(page.getByRole("heading", { name: "Marketing", level: 1 })).toBeVisible();
    await expect(page.getByText("Campaigns and launches")).toBeVisible();
    await expect(page.getByRole("heading", { name: "No projects yet" })).toBeVisible();
    await expect(sidebar(page).getByRole("link", { name: "Marketing" })).toHaveAttribute("aria-current", "page");
  });

  test("New Space needs the create_space capability", async ({ page }) => {
    await mockSpaces(page, { me: { ...me, capabilities: { manage_users: false, create_space: false } } });
    await page.goto("/");
    await expect(sidebar(page).getByRole("link", { name: "Personal" })).toBeVisible();
    await expect(page.getByRole("button", { name: "New Space" })).toHaveCount(0);
  });

  test("keeps the fixed nav items before the Spaces list", async ({ page }) => {
    await mockSpaces(page);
    await page.goto("/");
    await expect(page.getByRole("navigation", { name: "Main navigation" }).getByRole("button")).toHaveText(["Dashboards", "Inbox", /^Search/, "Request forms", "Stream", "More"]);
  });
});

test.describe("create Space", () => {
  test("starts from a template with one idempotent request", async ({ page }) => {
    const template = { id: "project-management", name: "Project plan", category: "Project management", description: "A project with phases.",
      version: 1, compatible: true, counts: { projects: 1, tasks: 4, views: 1, forms: 1, custom_fields: 1 } };
    const base = { ...(extra()), "GET /space-templates": { body: { items: [template] } } };
    const { calls } = await mockSpaces(page, { extra: { ...base,
      "POST /space-templates/project-management/spaces": (body: Record<string, unknown>) => ({ status: 201, body: { id: "99999999-9999-4999-8999-999999999999", description: null, kind: "shared", version: 1, my_capabilities: [], ...body } }),
    } });
    await page.goto("/");
    await page.getByRole("button", { name: "New Space" }).click();
    const dialog = page.getByRole("dialog", { name: "New Space" });
    await dialog.getByLabel("Start from").selectOption("project-management");
    await expect(dialog.getByText("Includes 1 project, 4 tasks")).toBeVisible();
    await expect(dialog.getByLabel("Description")).toHaveCount(0);
    await dialog.getByLabel("Name").fill("Website relaunch");
    await dialog.getByRole("button", { name: "Create Space" }).click();
    await expect(dialog).toBeHidden();
    const body = calls.find((c) => c.method === "POST" && c.path === "/space-templates/project-management/spaces")?.body as Record<string, string>;
    expect(body.name).toBe("Website relaunch");
    expect(body.request_id).toMatch(/^[0-9a-f-]{36}$/);
    expect(calls.some((c) => c.method === "POST" && c.path === "/spaces")).toBe(false);
  });
  test("includes initial sharing in the create request", async ({ page }) => {
    const { calls } = await mockSpaces(page, { extra: extra() });
    await page.goto("/");
    await page.getByRole("button", { name: "New Space" }).click();
    const dialog = page.getByRole("dialog", { name: "New Space" });
    await dialog.getByLabel("Name").fill("Research");
    await dialog.locator("summary").click();
    await dialog.getByLabel("Add people or groups").fill("gra");
    await dialog.getByRole("button", { name: "Add Grace Hopper" }).click();
    await dialog.getByRole("button", { name: "Create Space" }).click();
    await expect(dialog).toBeHidden();
    expect(calls.find((c) => c.method === "POST" && c.path === "/spaces")?.body).toEqual({
      name: "Research", shares: [{ principal_type: "user", principal_id: grace.id, role: "read" }],
    });
  });
  test("posts the form, lands on the new Space", async ({ page }) => {
    const { calls } = await mockSpaces(page);
    await page.goto("/");
    await page.getByRole("button", { name: "New Space" }).click();
    const dialog = page.getByRole("dialog", { name: "New Space" });
    await dialog.getByRole("button", { name: "Create Space" }).click();
    await expect(dialog.getByRole("alert")).toHaveText("Enter a name for the Space");
    expect(calls.some((c) => c.method === "POST")).toBe(false);

    await dialog.getByLabel("Name").fill("Research");
    await dialog.getByLabel("Description").fill("Lab notes");
    await dialog.getByRole("button", { name: "Create Space" }).click();
    await expect(dialog).toBeHidden();
    expect(calls.find((c) => c.method === "POST" && c.path === "/spaces")?.body).toEqual({ name: "Research", description: "Lab notes" });
    await expect(page.getByRole("heading", { name: "Research", level: 1 })).toBeVisible();
    await expect(sidebar(page).getByRole("link", { name: "Research" })).toBeVisible();
  });

  test("shows the server message on 422", async ({ page }) => {
    await mockSpaces(page, { extra: { "POST /spaces": problem(422, "name already used", "validation_failed") } });
    await page.goto("/");
    await page.getByRole("button", { name: "New Space" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Name").fill("Marketing");
    await dialog.getByRole("button", { name: "Create Space" }).click();
    await expect(dialog.getByRole("alert")).toHaveText("name already used");
  });
});

test.describe("Space page", () => {
  test("rename sends If-Match with the Space version", async ({ page }) => {
    const { calls } = await mockSpaces(page);
    await page.goto(`/spaces/${marketing.id}`);
    await page.getByRole("button", { name: "Space actions" }).click();
    await page.getByRole("menuitem", { name: "Rename" }).click();
    const dialog = page.getByRole("dialog", { name: "Rename Space" });
    await expect(dialog.getByLabel("Name")).toHaveValue("Marketing");
    await dialog.getByLabel("Name").fill("Growth");
    await dialog.getByLabel("Description").fill("");
    await dialog.getByRole("button", { name: "Save" }).click();
    await expect(dialog).toBeHidden();
    const patch = calls.find((c) => c.method === "PATCH");
    expect(patch?.headers["if-match"]).toBe('"4"');
    expect(patch?.body).toEqual({ name: "Growth", description: null });
    await expect(page.getByRole("heading", { name: "Growth", level: 1 })).toBeVisible();
    await expect(sidebar(page).getByRole("link", { name: "Growth" })).toBeVisible();
  });

  test("409 shows the conflict banner and Reload brings the latest values", async ({ page }) => {
    const { spaces } = await mockSpaces(page);
    await page.goto(`/spaces/${marketing.id}`);
    await page.getByRole("button", { name: "Space actions" }).click();
    await page.getByRole("menuitem", { name: "Rename" }).click();
    const dialog = page.getByRole("dialog", { name: "Rename Space" });
    // Someone else renames the Space after this page loaded.
    spaces[1] = { ...spaces[1], name: "Marketing EMEA", version: 5 };
    await dialog.getByLabel("Name").fill("Growth");
    await dialog.getByRole("button", { name: "Save" }).click();
    await expect(dialog.getByRole("alert")).toContainText("This Space changed; reload");
    await dialog.getByRole("button", { name: "Reload" }).click();
    await expect(dialog.getByRole("alert")).toHaveCount(0);
    await expect(dialog.getByLabel("Name")).toHaveValue("Marketing EMEA");
  });

  test("428 asks for a reload too", async ({ page }) => {
    await mockSpaces(page, { extra: { [`PATCH /spaces/${marketing.id}`]: problem(428, "If-Match required", "precondition_required") } });
    await page.goto(`/spaces/${marketing.id}`);
    await page.getByRole("button", { name: "Space actions" }).click();
    await page.getByRole("menuitem", { name: "Rename" }).click();
    await page.getByRole("dialog").getByRole("button", { name: "Save" }).click();
    await expect(page.getByRole("dialog").getByRole("alert")).toContainText("out of date");
  });

  test("delete confirms, then opens the first remaining Space", async ({ page }) => {
    const { calls } = await mockSpaces(page);
    await page.goto(`/spaces/${marketing.id}`);
    await page.getByRole("button", { name: "Space actions" }).click();
    await page.getByRole("menuitem", { name: "Delete" }).click();
    const dialog = page.getByRole("dialog", { name: "Delete this Space?" });
    await expect(dialog.getByRole("button", { name: "Cancel" })).toBeFocused();
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
    await dialog.getByRole("button", { name: "Delete Space" }).click();
    await expect(page).toHaveURL(new RegExp(`/spaces/${personal.id}$`));
    expect(calls.some((c) => c.method === "DELETE" && c.path === `/spaces/${marketing.id}`)).toBe(true);
    await expect(sidebar(page).getByRole("link", { name: "Marketing" })).toHaveCount(0);
    await expect(sidebar(page).getByRole("link", { name: "Operations" })).toBeVisible();
  });

  test("delete shows the server message on 403", async ({ page }) => {
    await mockSpaces(page, { extra: { [`DELETE /spaces/${marketing.id}`]: problem(403, "only full access can delete", "forbidden") } });
    await page.goto(`/spaces/${marketing.id}`);
    await page.getByRole("button", { name: "Space actions" }).click();
    await page.getByRole("menuitem", { name: "Delete" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("button", { name: "Delete Space" }).click();
    await expect(dialog.getByRole("alert")).toHaveText("only full access can delete");
    await expect(page).toHaveURL(new RegExp(`/spaces/${marketing.id}$`));
  });

  test("Personal space has no Share, rename or delete", async ({ page }) => {
    await mockSpaces(page);
    await page.goto(`/spaces/${personal.id}`);
    await expect(page.getByRole("heading", { name: "Personal", level: 1 })).toBeVisible();
    await expect(page.getByRole("button", { name: "Share" })).toHaveCount(0);
    await page.getByRole("button", { name: "Space actions" }).click();
    await expect(page.getByRole("menuitem")).toHaveText(["Bookmarks", "Settings", "Import from Excel", "Export Space to Excel", "“+” menu", "Custom fields"]);
  });

  test("actions follow my_capabilities", async ({ page }) => {
    await mockSpaces(page, { spaces: [personal, { ...marketing, my_capabilities: ["view", "edit"] }] });
    await page.goto(`/spaces/${marketing.id}`);
    await expect(page.getByRole("button", { name: "Share" })).toHaveCount(0);
    await page.getByRole("button", { name: "Space actions" }).click();
    await expect(page.getByRole("menuitem")).toHaveText(["Bookmarks", "Settings", "Export Space to Excel", "“+” menu", "Rename"]);
  });

  test("a viewer gets no actions at all", async ({ page }) => {
    await mockSpaces(page, { spaces: [personal, { ...marketing, my_capabilities: ["view"] }] });
    await page.goto(`/spaces/${marketing.id}`);
    await expect(page.getByRole("heading", { name: "Marketing", level: 1 })).toBeVisible();
    await expect(page.getByRole("button", { name: "Share" })).toHaveCount(0);
    // Bookmarks and the read-only export stay available to any viewer; nothing that edits the Space is listed.
    await page.getByRole("button", { name: "Space actions" }).click();
    await expect(page.getByRole("menuitem")).toHaveText(["Bookmarks", "Export Space to Excel"]);
  });

  test("404 shows Space not found without detail", async ({ page }) => {
    await mockSpaces(page, { extra: { "GET /spaces/99999999-9999-4999-8999-999999999999": problem(404, "no such space", "not_found") } });
    await page.goto("/spaces/99999999-9999-4999-8999-999999999999");
    await expect(page.getByRole("heading", { name: "Space not found" })).toBeVisible();
    await expect(page.getByText("no such space")).toHaveCount(0);
    await page.getByRole("link", { name: "Go to your Personal space" }).click();
    await expect(page).toHaveURL(new RegExp(`/spaces/${me.personal_space_id}$`));
  });
});


test("sidebar expands ranked containers by keyboard, remembers collapse and opens More destinations", async ({ page }) => {
  const project = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
  const folder = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /spaces/${marketing.id}/items`]: { body: { items: [{ id: project, title: "Roadmap", type: "project" }] } },
    [`GET /items/${project}/children`]: { body: { items: [{ id: folder, title: "Specs", type: "folder" }] } },
    "GET /me/shared": { body: { items: [{ id: folder, title: "Shared specs", type: "folder" }] } },
  } });
  await page.goto(`/spaces/${marketing.id}`);
  await expect(page.getByText("Acme Workspace")).toBeVisible();
  const branch = page.getByRole("button", { name: "Expand Marketing" });
  await branch.focus();
  await branch.press("ArrowRight");
  await expect(page.getByRole("link", { name: "Roadmap" })).toBeVisible();
  await page.getByRole("button", { name: "Expand Roadmap" }).press("Enter");
  await expect(page.getByRole("link", { name: "Specs", exact: true })).toHaveAttribute("href", `/items/${folder}`);
  expect(calls.some((call) => call.path === `/items/${project}/children` && call.query.get("sort") === "manual"
    && call.query.get("type") === "project,folder")).toBe(true);
  await page.getByRole("button", { name: "Collapse sidebar" }).press("Enter");
  await page.reload();
  await expect(page.getByRole("button", { name: "Expand sidebar" })).toBeVisible();
  await page.getByRole("button", { name: "Expand sidebar" }).click();
  await page.getByRole("link", { name: "Shared with me" }).click();
  await expect(page.getByRole("link", { name: "Shared specs" })).toHaveAttribute("href", `/items/${folder}`);
  await page.getByRole("button", { name: "Account menu" }).click();
  await page.getByRole("menuitem", { name: "Recycle bin" }).click();
  await expect(page.getByRole("heading", { name: "Recycle bin" })).toBeVisible();
});

test("an empty Space and a filtered Table show distinct recovery actions", async ({ page }) => {
  const root = `/spaces/${marketing.id}`;
  const project = { id: marketing.id, type: "project", title: "Launch", assignees: [], locations: [{ space_id: marketing.id, parent_item_id: null }], my_capabilities: ["view"] };
  let empty = true;
  await mockSpaces(page, { extra: {
    [`GET ${root}/views`]: { body: { items: [{ id: marketing.id, type: "table", name: "Table", visibility: "public", position: 0, is_default: true, config: {} }] } },
    [`GET ${root}/items`]: () => ({ body: { items: empty ? [] : [project] } }),
  } });
  await page.goto(root);
  await expect(page.getByRole("heading", { name: "No projects yet" })).toBeVisible();
  await expect(page.getByText("No matching items")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "New item" }).last()).toBeVisible();

  empty = false;
  await page.goto(root + "?filter=missing");
  await expect(page.getByRole("status").filter({ hasText: "No matching items" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "No projects yet" })).toHaveCount(0);
  await page.getByRole("button", { name: "Clear filters" }).press("Enter");
  await expect(page.getByText("Launch", { exact: true })).toBeVisible();
});
