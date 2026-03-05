/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../dist',
    emptyOutDir: false,
  },
  server: {
    host: '0.0.0.0',
    port: 9369,
    proxy: {
      '/api': { target: 'http://localhost:9368', changeOrigin: true },
      '/ws': { target: 'ws://localhost:9368', ws: true },
      '/health': { target: 'http://localhost:9368' },
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  },
})
