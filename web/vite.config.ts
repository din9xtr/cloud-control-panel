import { writeFileSync } from "node:fs";
import { fileURLToPath, URL } from "node:url";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    {
      name: "keep-dist-placeholder",
      closeBundle: () => writeFileSync(new URL("../internal/web/dist/.gitkeep", import.meta.url), ""),
    },
  ],
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  build: {
    outDir: "../internal/web/dist",
    emptyOutDir: true,
    sourcemap: false,
    target: "es2022",
    assetsInlineLimit: (file) => (file.includes("/src/brand/") ? false : undefined),
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            { name: "react", test: /node_modules[\\/](react|react-dom|scheduler)[\\/]/ },
            { name: "tanstack", test: /node_modules[\\/]@tanstack[\\/]/ },
            { name: "tus", test: /node_modules[\\/](tus-js-client|js-base64|url-parse|requires-port|querystringify|buffer-from|combine-errors|lodash\.throttle|proper-lockfile|is-stream)[\\/]/ },
          ],
        },
      },
    },
  },
  server: {
    proxy: {
      "/api": "http://localhost:8080",
      "/s/": "http://localhost:8080",
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    css: false,
  },
});
