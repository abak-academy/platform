import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:3000";

// FB-22/23/24/25 repro harness. Chromium only — see e2e/question-editor.spec.ts
// for why (jsdom/vitest cannot drive document.execCommand or real selection).
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  reporter: "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: `npm run dev -- -p ${new URL(baseURL).port || "3000"}`,
    url: baseURL,
    reuseExistingServer: true,
  },
});
