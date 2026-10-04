import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";
export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: "/",
  resolve: { alias: { "@": path.resolve(import.meta.dirname, "./src") } },
  build: { outDir: "build", assetsDir: "assets", manifest: true },
  server: {
    host: "127.0.0.1",
    proxy: {
      "/api": { target: "http://localhost:8080", changeOrigin: false },
      "/ready": { target: "http://localhost:8080" },
    },
  },
});
