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
    environment: "node",
    include: ["src/**/*.test.ts"],
    css: { include: [/\?raw/] },
  },
});
