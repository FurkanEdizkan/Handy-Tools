import { defineConfig } from 'vite';

// The extension is a plain multi-entry Vite build — no MV3 plugin dependency.
// public/ (manifest.json + icons/) is copied verbatim to dist/. The two HTML
// pages live at the project root so they emit as dist/popup.html and
// dist/options.html; the service worker gets a stable, unhashed filename so
// manifest.json can reference it.
export default defineConfig({
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: 'es2022',
    sourcemap: false,
    rollupOptions: {
      input: {
        popup: 'popup.html',
        options: 'options.html',
        'service-worker': 'src/background/service-worker.ts',
      },
      output: {
        entryFileNames: (chunk) =>
          chunk.name === 'service-worker'
            ? 'service-worker.js'
            : 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash][extname]',
      },
    },
  },
});
