import { defineConfig } from "@playwright/test";

const port = Number(process.env.PMS_TEST_BROWSER_PORT ?? "4173");
const baseURL = `http://127.0.0.1:${port}`;

export default defineConfig({
  testDir: "./tests",
  outputDir: port === 4173 ? "test-results" : `test-results/${port}`,
  use: {
    baseURL,
    reducedMotion: "reduce", // dialogs and toasts are not mid-fade when axe reads their colours
    channel: process.env.PLAYWRIGHT_CHANNEL || undefined,
  },
  webServer: {
    command: `npm run dev -- --host 127.0.0.1 --port ${port} --strictPort`,
    url: baseURL,
    reuseExistingServer: !process.env.CI && !process.env.PMS_FIXTURE_API,
  },
});
