import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    // The output is embedded into the Go binary (web/embed.go), so a
    // deployment is one file and there is no window in which the API is new
    // and the assets are old.
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    port: 5173,
    proxy: {
      // Development only. In production the Go binary serves both.
      '/api': { target: 'http://127.0.0.1:8093', changeOrigin: true },
    },
  },
})
