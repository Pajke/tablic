import { defineConfig } from 'vite'

export default defineConfig({
  build: {
    target: 'esnext', // enables top-level await
    rollupOptions: {
      output: {
        // Merge all node_modules into one vendor chunk so PixiJS's internal
        // dynamic imports (autoDetectRenderer) resolve within the same chunk
        // — no separate HTTP requests for renderer chunks via the reverse proxy
        manualChunks: (id) => (id.includes('node_modules') ? 'vendor' : undefined),
      },
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/ws': {
        target: 'ws://localhost:3579',
        ws: true,
      },
      '/api': {
        target: 'http://localhost:3579',
      },
    },
  },
})
