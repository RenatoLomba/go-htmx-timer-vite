import { resolve } from "path";
import { defineConfig } from "vite";

export default defineConfig({
  build: {
    lib: {
      entry: resolve(__dirname, "src/main.ts"),
      name: "htmx-go-timer-vite",
      fileName: "bundle",
      formats: ["es"],
    },
    outDir: resolve(__dirname, "public", "dist"),
    copyPublicDir: false,
  },
});
