import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: false,
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    restoreMocks: true,
    clearMocks: true,
  },
  server: {
    port: 3000,
    // Proxy API requests to the Go backend during development
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:7380',
        changeOrigin: true,
      },
      '/__aegis': {
        target: 'http://127.0.0.1:7380',
        changeOrigin: true,
      },
    },
  },
});
