import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const e = process.env;
test.skip(!e.PMS_FIXTURE_FB_ADMIN, "Requires the local Go/PostgreSQL form-blueprint fixture");

test("E2E-PAR-05: a form branches to Blueprints; an outside visitor fills it and gets the item and children of the branch taken", async ({ browser }) => {
  const baseURL = test.info().project.use.baseURL!;
  const context = await browser.newContext({ baseURL, colorScheme: "light" });
  await context.addCookies([{ name: "pms_session", value: e.PMS_FIXTURE_FB_ADMIN!.split("=")[1], url: baseURL }]);
  const admin = await context.newPage();
  await admin.goto(`/spaces/${e.PMS_FIXTURE_FB_SPACE}/forms/new`);
  await admin.getByLabel("Form name").fill("Intake");
  const add = async (kind: string) => {
    await admin.getByRole("button", { name: "Add question to page 1" }).click();
    await admin.getByRole("menuitem", { name: kind }).click();
  };
  await add("Single line text");
  await admin.getByLabel("Label", { exact: true }).fill("Title");
  await add("Single choice");
  await admin.getByLabel("Label", { exact: true }).nth(1).fill("Kind");
  await admin.getByLabel("Option 1", { exact: true }).fill("Bug");
  await admin.getByLabel("Option 2", { exact: true }).fill("Idea");
  await add("Paragraph");
  await admin.getByLabel("Label", { exact: true }).nth(2).fill("Steps");
  await admin.getByLabel("Required").nth(2).check();
  await admin.getByLabel(/Show "Steps" conditionally/).check();
  await admin.getByLabel('Earlier question for "Steps"').selectOption("kind");
  await admin.getByLabel('Value for "Steps"').selectOption("Bug");
  await admin.getByLabel("Destination Space").selectOption(e.PMS_FIXTURE_FB_SPACE!);
  await admin.getByLabel("Item title comes from").selectOption("title");
  for (const [n, kind, type, blueprint] of [[1, "Bug", "bug", "Bug triage"], [2, "Idea", "idea", "Idea review"]] as const) {
    await admin.getByRole("button", { name: "Add a route" }).click();
    await admin.getByLabel(`Route ${n}: when`).selectOption("kind");
    await admin.getByLabel(`Route ${n}: is`).selectOption(kind);
    await admin.getByLabel(`Route ${n}: create`).selectOption(type);
    await admin.getByLabel(`Route ${n}: blueprint`).selectOption({ label: blueprint });
  }
  await expect(admin.getByText("Route 2: when Kind is Idea, create Idea with blueprint: Idea review")).toBeVisible();
  const axe = await new AxeBuilder({ page: admin }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
  expect(axe.violations.map((v) => `${v.id}: ${v.nodes[0]?.html}`)).toEqual([]);
  await admin.getByRole("button", { name: "Save form" }).click();
  await expect(admin).toHaveURL(/\/forms\/[0-9a-f-]{36}$/);
  const formID = admin.url().split("/").pop()!;
  // A new form is a draft (RF-06): publish it before the public link can be used.
  expect((await admin.request.post(`/api/v1/forms/${formID}/publish`, { headers: { Origin: baseURL } })).ok()).toBeTruthy();
  const link = await admin.request.post(`/api/v1/forms/${formID}/public-link`, { headers: { Origin: baseURL } });
  expect(link.ok()).toBeTruthy();
  const token = (await link.json()).public_token as string;

  // An outside visitor with no session fills the public link twice, taking each branch.
  const visitor = await (await browser.newContext({ baseURL })).newPage();
  const submit = async (title: string, kind: string, steps?: string) => {
    await visitor.goto(`/f/${token}`);
    await visitor.getByLabel("Title").fill(title);
    await visitor.getByRole("radio", { name: kind }).check();
    if (steps) await visitor.getByLabel(/Steps/).fill(steps);
    else await expect(visitor.getByLabel(/Steps/)).toHaveCount(0);
    await visitor.getByLabel("Your name").fill("Pat Visitor");
    await visitor.getByLabel("Your email").fill("pat@example.com");
    await visitor.getByRole("button", { name: "Submit" }).click();
    await expect(visitor.getByRole("heading", { name: "Request received" })).toBeVisible();
  };
  await submit("Crash on save", "Bug", "open the file");
  await submit("Dark mode", "Idea");

  const list = (await (await admin.request.get(`/api/v1/spaces/${e.PMS_FIXTURE_FB_SPACE}/items`)).json()).items as { id: string; title: string; item_type: { key: string }; child_count: number }[];
  const bug = list.find((i) => i.title === "Crash on save")!;
  const idea = list.find((i) => i.title === "Dark mode")!;
  expect([bug.item_type.key, bug.child_count]).toEqual(["bug", 2]);
  expect([idea.item_type.key, idea.child_count]).toEqual(["idea", 1]);
  await admin.goto(`/tasks/${bug.id}`);
  await expect(admin.getByText("Reproduce")).toBeVisible();
  await expect(admin.getByText("Evaluate")).toHaveCount(0);
});
