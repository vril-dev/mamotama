import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');

  return {
    base: env.VITE_APP_BASE_PATH || '/',
    plugins: [react(), tailwindcss()],
    server: {
      host: true,
      port: 5173
    },
  };
});
