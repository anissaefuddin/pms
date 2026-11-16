import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";

const event = (id: number, action: string, over: Record<string, unknown> = {}) => ({
  id, at: "2026-11-14T03:00:00Z", actor_id: null, actor_name: "ada@example.test", action, target_type: "user", target_id: "u1", ip: "203.0.113.9", details: {}, ...over });

test("admin searches loaded rows, opens an event drawer that closes with Escape and returns focus, and downloads the shown rows", async ({ page }) => {
  await mockSpaces(page, { extra: { "GET /audit-events": { body: { items: [
    event(12, "auth.login", { details: { method: "password" } }),
    event(10, "share.space_changed", { actor_name: "ops, lead", target_type: "space", target_id: "s1", details: { note: 'said "ok"' } }),
  ], next_cursor: null } } } });
  await page.goto("/settings/audit");
  await expect(page.getByRole("row")).toHaveCount(3);

  await page.getByLabel("Search loaded events").fill("ops, lead");
  await expect(page.getByRole("row")).toHaveCount(2);
  await expect(page.getByRole("row", { name: /Changed Space sharing/ })).toBeVisible();

  const view = page.getByRole("button", { name: "View event: Changed Space sharing" });
  await view.click();
  const drawer = page.getByRole("dialog", { name: "Event details" });
  await expect(drawer).toContainText("share.space_changed");
  await expect(drawer).toContainText("s1");
  await expect(drawer).toContainText('said "ok"');
  await page.keyboard.press("Escape");
  await expect(drawer).toHaveCount(0);
  await expect(view).toBeFocused();

  const [file] = await Promise.all([
    page.waitForEvent("download"),
    page.getByRole("button", { name: "Download shown rows (CSV)" }).click(),
  ]);
  expect(file.suggestedFilename()).toMatch(/^audit-events-\d{4}-\d{2}-\d{2}\.csv$/);
  const lines = readFileSync(await file.path(), "utf8").replace(/^﻿/, "").split("\r\n");
  expect(lines).toHaveLength(2);
  expect(lines[0]).toBe("id,at,actor_id,actor_name,action,target_type,target_id,ip,details");
  expect(lines[1]).toContain('"ops, lead"');
  expect(lines[1]).toContain("share.space_changed,space,s1");
});
