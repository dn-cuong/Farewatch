import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  // Load VITE_* from repo-root .env (same file Docker Compose uses).
  envDir: '..',
  server: {
    port: 5173,
    host: true,
  },
});
