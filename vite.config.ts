import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    // Stage 2 will proxy REST + SSE to the Go API here.
    // proxy: { "/api": { target: "http://127.0.0.1:8080", changeOrigin: true } },
  },
});
