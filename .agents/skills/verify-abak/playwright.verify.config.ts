import { defineConfig } from "@playwright/test";

// Run only through `abak-verify journey <name>`, which sets these and starts no server.
const evidence = process.env.VERIFY_EVIDENCE_DIR ?? "/tmp/abak-verify-evidence";

export default defineConfig({
  testDir: "./journeys",
  outputDir: `${evidence}/test-output`,
  workers: 1,
  reporter: [["list"], ["json", { outputFile: `${evidence}/results.json` }]],
  use: {
    baseURL: process.env.VERIFY_WEB_URL ?? "http://localhost:53000",
    channel: "chrome",
    viewport: { width: 1280, height: 800 },
    trace: "on",
    screenshot: "on",
  },
});
