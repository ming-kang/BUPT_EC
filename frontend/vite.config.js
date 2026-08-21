import process from 'node:process';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { visualizer } from 'rollup-plugin-visualizer';

// Vendor chunk matchers. Anchor on the real package-name segment
// (`/node_modules/<pkg>/`) so pnpm's `.pnpm/<pkg>@<ver>_<peer-suffix>`
// directory names cannot cause false positives; never use bare
// substring checks like id.includes('antd').
const REACT_ID = /\/node_modules\/(react|react-dom|scheduler)\//;
const ANTD_ID = /\/node_modules\/(antd|@ant-design|rc-[a-z-]+|@rc-component)\//;

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react(),
    // BUNDLE_REPORT=1 pnpm build → treemap report for humans.
    // The report must NOT be written into dist/: dist is go:embed'ed into
    // release binaries and served publicly. bundle-stats.local.html sits in
    // frontend/ and is gitignored via the existing *.local rule.
    process.env.BUNDLE_REPORT === '1' &&
      visualizer({
        filename: 'bundle-stats.local.html',
        gzipSize: true,
        brotliSize: true,
        template: 'treemap',
        open: false,
      }),
  ].filter(Boolean),
  build: {
    // Reject Vite 7's new Safari 16 default baseline to keep old-iOS (<16)
    // devices working (~Vite 6 'modules' baseline); revisit in batch 4.
    target: ['es2020', 'safari14'],
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (REACT_ID.test(id)) {
            return 'react-vendor';
          }
          if (ANTD_ID.test(id)) {
            return 'antd-vendor';
          }
        },
      },
    },
  },
  test: {
    // Default stays node for pure helper tests; lifecycle files opt into jsdom
    // via their file-level @vitest-environment directive.
    environment: 'node',
    // DOM-only stubs (matchMedia for antd) live here; the file guards on
    // `typeof window` so node-environment tests are untouched.
    setupFiles: ['./src/test/setup.ts'],
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
