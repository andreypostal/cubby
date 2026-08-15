import path from "node:path"

import { defineConfig } from "vite"
import react from "@vitejs/plugin-react-swc"
import tailwindcss from "@tailwindcss/vite"

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "."),
    },
  },
  build: {
    // The Go binary embeds this directory via go:embed in
    // server/pkg/modules/web, so building the UI is what makes the server
    // compile. Vite also copies public/.gitkeep in here on every build, which is
    // what keeps that package compiling on a fresh clone.
    outDir: path.resolve(import.meta.dirname, "../server/pkg/modules/web/dist"),
    // outDir sits outside this project root, where Vite refuses to clear it
    // unless asked explicitly. Without this, stale content-hashed assets pile up
    // and get baked into the binary forever.
    emptyOutDir: true,
    // Served by the Go binary via go:embed, which skips paths starting with
    // "_" or "." unless the embed pattern opts in. Keeping asset names plain
    // avoids depending on that.
    assetsDir: "assets",
  },
  server: {
    proxy: {
      // Same origin in dev, matching production where the Go binary serves both
      // the UI and the API. Keeps CORS out of the picture entirely.
      "/api": "http://127.0.0.1:8080",
    },
  },
})
