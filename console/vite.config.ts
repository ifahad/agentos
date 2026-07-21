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
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
