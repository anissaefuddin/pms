import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createRouter } from "@tanstack/react-router";
import "./i18n";
import "@fontsource-variable/inter";
import "@fontsource-variable/jetbrains-mono";
import "./index.css";
import { routeTree } from "./routeTree.gen";

const router = createRouter({ routeTree });
declare module "@tanstack/react-router" { interface Register { router: typeof router } }

const systemTheme = window.matchMedia("(prefers-color-scheme: dark)");
function applySystemTheme({ matches }: MediaQueryList | MediaQueryListEvent) {
  document.documentElement.classList.toggle("dark", matches);
}
applySystemTheme(systemTheme);
systemTheme.addEventListener("change", applySystemTheme);

// A new release deletes the old hashed chunks. Reload once to fetch the new index.html instead of showing an error screen.
window.addEventListener("vite:preloadError", () => {
  try {
    if (sessionStorage.getItem("pms:chunk-reload") === "1") return; // already retried: let the error surface
    sessionStorage.setItem("pms:chunk-reload", "1");
  } catch { /* storage blocked: fall through and reload anyway */ }
  location.reload();
});

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 30_000 } } });
ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode><QueryClientProvider client={queryClient}><RouterProvider router={router} /></QueryClientProvider></React.StrictMode>,
);
