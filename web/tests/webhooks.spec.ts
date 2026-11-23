import { AxeBuilder } from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";

const hid = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee";

test("creates a webhook, shows the secret once, pauses it and reads its log", async ({ page }) => {
  let items: object[] = [];
  const { calls } = await mockSpaces(page, { extra: {
    "GET /webhooks": () => ({ body: { items, event_types: ["item.created", "item.updated"] } }),
    "POST /webhooks": (body: Record<string, unknown>) => {
      items = [{ id: hid, name: body.name, url: body.url, event_types: body.event_types, enabled: true, consecutive_failures: 0, disabled_reason: null }];
      return { status: 201, body: { ...items[0], secret: "whsec_abc" } };
    },
    [`PATCH /webhooks/${hid}`]: (body: Record<string, unknown>) => { items = [{ ...(items[0] as object), ...body }]; return { body: items[0] }; },
    [`GET /webhooks/${hid}/deliveries`]: { body: { items: [{ id: "d1", event_type: "item.created", status: "success", attempts: 1, response_status: 200, last_error: null, created_at: "2026-11-14T10:00:00Z" }] } },
  } as never });
  await page.goto("/settings/developer");
  await page.getByLabel("Name").fill("CI hook");
  await page.getByLabel("HTTPS endpoint URL").fill("https://example.com/h");
  await page.getByRole("checkbox", { name: "item.created" }).check();
  await page.getByRole("button", { name: "Add webhook" }).click();
  await expect(page.getByTestId("webhook-secret")).toHaveText("whsec_abc");
  await page.getByRole("button", { name: "I saved it" }).click();
  await expect(page.getByTestId("webhook-secret")).toHaveCount(0);
  await page.getByRole("button", { name: "Pause" }).click();
  await expect.poll(() => calls.find((c) => c.method === "PATCH")?.body).toEqual({ enabled: false });
  await page.getByRole("button", { name: "Delivery log" }).click();
  await expect(page.getByRole("cell", { name: "success" })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});
