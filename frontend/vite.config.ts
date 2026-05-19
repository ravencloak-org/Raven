import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// VITE_BASE_PATH lets the build target a path-prefixed deployment (e.g. the
// public demo at https://demo.ravencloak.org/raven/). Default is root.
const basePath = process.env.VITE_BASE_PATH ?? '/'

export default defineConfig({
  base: basePath,
  plugins: [vue(), tailwindcss()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        // Go API defaults to RAVEN_SERVER_PORT=8081 (`internal/config/config.go`);
        // earlier 8080 was a leftover from pre-Phase-1 wiring.
        target: 'http://localhost:8081',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    exclude: ['e2e/**', 'node_modules/**'],
  },
})
