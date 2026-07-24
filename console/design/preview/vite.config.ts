import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// Builds the real component previews (design/preview/main.tsx) into a single
// IIFE bundle + one CSS file under design/assets/, referenced by the @dsCard
// HTML shells synced to the Claude Design project.
export default defineConfig({
  root: __dirname,
  plugins: [react()],
  build: {
    outDir: "../assets",
    emptyOutDir: true,
    cssCodeSplit: false,
    lib: {
      entry: "main.tsx",
      name: "AgentOSDesignPreview",
      formats: ["iife"],
    },
  },
});
