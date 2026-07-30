import { defineConfig, devices } from "@playwright/test";

// FB-22/23/24/25 repro harness. Chromium only — see e2e/question-editor.spec.ts
// for why (jsdom/vitest cannot drive document.execCommand or real selection).
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  reporter: "list",
  use: {
    baseURL: "http://localhost:3000",
    trace: "retain-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: "npm run dev",
    url: "http://localhost:3000",
    reuseExistingServer: true,
  },
});
