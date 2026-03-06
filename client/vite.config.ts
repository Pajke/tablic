import { defineConfig } from 'vite'

export default defineConfig({
  build: {
    target: 'esnext', // enables top-level await
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
