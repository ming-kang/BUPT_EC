import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    // Reject Vite 7's new Safari 16 default baseline to keep old-iOS (<16)
    // devices working (~Vite 6 'modules' baseline); revisit in batch 4.
    target: ['es2020', 'safari14'],
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (
            id.includes('/node_modules/react/') ||
            id.includes('/node_modules/react-dom/') ||
            id.includes('/node_modules/scheduler/')
          ) {
            return 'react-vendor';
          }
        },
      },
    },
  },
  test: {
    // Default stays node for pure helper tests; lifecycle files opt into jsdom
    // via their file-level @vitest-environment directive.
    environment: 'node',
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
});
