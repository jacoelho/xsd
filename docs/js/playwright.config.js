import { fileURLToPath } from "node:url";

import { defineConfig } from "@playwright/test";

const repoRoot = fileURLToPath(new URL("../..", import.meta.url));

export default defineConfig({
  testDir: "./browser",
  fullyParallel: false,
  reporter: "line",
  timeout: 60_000,
  use: {
    baseURL: "http://127.0.0.1:8765",
    trace: "retain-on-failure",
  },
  webServer: {
    command: "make web",
    cwd: repoRoot,
    reuseExistingServer: false,
    timeout: 120_000,
    url: "http://127.0.0.1:8765",
  },
});
