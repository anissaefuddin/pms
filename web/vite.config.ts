import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";
import { readFileSync } from "node:fs";

const version = JSON.parse(readFileSync(path.resolve(__dirname, "package.json"), "utf8")).version;

export default defineConfig({
  define: { __APP_VERSION__: JSON.stringify(version) },
  plugins: [tanstackRouter({ target: "react", autoCodeSplitting: true }), react(), tailwindcss()],
  resolve: { alias: { "@": path.resolve(__dirname, "src") } },
  server: { proxy: { "/api": {
    target: process.env.PMS_FIXTURE_API ?? "http://localhost:8080",
    headers: process.env.PMS_FIXTURE_ORIGIN ? { Origin: process.env.PMS_FIXTURE_ORIGIN } : undefined,
  } } },
  build: { outDir: "dist", emptyOutDir: true },
});
