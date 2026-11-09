import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./vitest.setup.ts'],
    // Inline Oxygen UI and its MUI deps so Vite transforms them (as the console's own
    // vitest config does) instead of Node importing their CSS and extensionless paths.
    server: { deps: { inline: [/@wso2\/oxygen-ui/, /@mui\//] } },
  },
});
