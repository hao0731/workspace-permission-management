import { reactRouter } from "@react-router/dev/vite";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vitest/config";

const isVitest = process.env.VITEST === "true";

export default defineConfig({
  plugins: isVitest ? [tailwindcss()] : [tailwindcss(), reactRouter()],
  resolve: {
    tsconfigPaths: true,
  },
  test: {
    environment: "jsdom",
    setupFiles: "./app/test-setup.ts",
  },
});
