import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  base: './',
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    hmr: {
      host: '127.0.0.1',
      clientPort: 5173,
    },
  },
  build: {
    emptyOutDir: false,
  },
  plugins: [react()],
});
