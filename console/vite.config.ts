/// <reference types="vitest/config" />
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// Same-origin API paths. In production nginx proxies these to the gateway and
// runtime services; in dev the vite server does the same against localhost.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api/gateway": {
        target: "http://localhost:8080",
        rewrite: (path) => path.replace(/^\/api\/gateway/, ""),
      },
      "/api/runtime": {
        target: "http://localhost:18000",
        rewrite: (path) => path.replace(/^\/api\/runtime/, ""),
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        // React and framer-motion change only when a dependency is bumped,
        // while the app changes on every deploy. Splitting them means a
        // console update re-downloads app code alone instead of invalidating
        // the whole bundle. Nothing here reaches the network at runtime — the
        // chunks are served by the same nginx as the rest of the console.
        manualChunks: {
          vendor: ["react", "react-dom", "framer-motion"],
        },
      },
    },
  },
  test: {
    // node stays the default: the 36 suites that predate the DOM harness are
    // pure logic and run faster without one, and styles.partition.test.ts reads
    // CSS through ?raw, which needs no document. A component or hook test opts
    // in per file with a `// @vitest-environment jsdom` docblock — explicit at
    // the point of use, rather than a glob in here that has to be kept in step
    // with wherever tests happen to live.
    environment: "node",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    // Registers DOM cleanup between tests, and no-ops under the node
    // environment. See src/test/setup.ts for why it cannot be left implicit.
    setupFiles: ["src/test/setup.ts"],
    css: { include: [/\?raw/] },
  },
});
