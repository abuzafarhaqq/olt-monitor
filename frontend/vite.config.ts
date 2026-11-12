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
        // Must follow the port the API actually listens on. Hardcoding 8080 here
        // meant that running app.sh with BACKEND_PORT=8090 left the browser
        // talking to whatever stale process still held 8080 — which answered
        // 404 for routes it did not have and hung on the synchronous collect.
        target: `http://127.0.0.1:${process.env.BACKEND_PORT ?? "8080"}`,
        changeOrigin: false,
      },
    },
  },
});
