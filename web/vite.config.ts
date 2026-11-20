import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // The Go server, on its default HUSHTONGUE_ADDR
    proxy: { "/api": "http://localhost:8080" },
  },
});
