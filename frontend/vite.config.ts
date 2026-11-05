import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Single-route slice: Opportunities only. Base /app/ keeps Go routes
// (/jobs, /static, /api) distinct from Vite production assets.
export default defineConfig({
  plugins: [react()],
  base: "/app/",
  build: {
    outDir: "dist",
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    port: 5173,
    proxy: {
      "/api": "http://127.0.0.1:3000",
      "/jobs": "http://127.0.0.1:3000",
      "/static": "http://127.0.0.1:3000",
    },
  },
});
