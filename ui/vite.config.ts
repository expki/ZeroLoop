import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:3080', changeOrigin: true },
      '/ws': { target: 'ws://localhost:3080', ws: true },
      '/health': { target: 'http://localhost:3080' },
      '/graphql': { target: 'http://localhost:3080' },
    },
  },
})
