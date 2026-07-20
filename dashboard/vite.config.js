import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// The dashboard calls the backend under /api. In dev we proxy it to the backend
// so there are no cross-origin concerns; set VITE_PROXY_TARGET to point at your
// running API (nginx on :80 by default, or http://localhost:8080 for `go run`).
const target = process.env.VITE_PROXY_TARGET || 'http://localhost';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    host: '0.0.0.0',
    proxy: {
      '/api': { target, changeOrigin: true },
      '/health': { target, changeOrigin: true },
    },
  },
});
