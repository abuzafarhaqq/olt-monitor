import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    // Vite refuses unknown Host headers. Needed when the dev server is reached
    // from another machine by IP or hostname; harmless on localhost.
    allowedHosts: true,
    // Same-origin in dev, so the session cookie works without CORS: the browser
    // talks to :5173, Vite forwards /api to the Go server.
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8080",
        changeOrigin: false,
      },
    },
  },
});
