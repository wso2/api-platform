/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import basicSsl from '@vitejs/plugin-basic-ssl';
import react from '@vitejs/plugin-react';
import { defineConfig, loadEnv, type ProxyOptions } from 'vite';

// In dev, run the BFF locally (default http://localhost:8082, `make bff-run`)
// and route all same-origin BFF traffic to it, mirroring the production
// topology where the BFF itself serves the SPA. The BFF serves the runtime
// config scripts too (in both dev and prod now — there is no separate
// dev-only source for them anymore), so those two paths are proxied as well.
const bffProxy = (): Record<string, ProxyOptions> => {
  const target = process.env.BFF_DEV_TARGET || 'http://localhost:8082';
  const options: ProxyOptions = { target, changeOrigin: true };
  return {
    '/api': options,
    '/proxy': options,
    '/api-platform.env.config.js': options,
    '/api-platform.common.config.js': options,
  };
};

export default ({ mode }: { mode: string }) => {
  process.env = { ...process.env, ...loadEnv(mode, process.cwd()) };
  const basePath = process.env.VITE_APP_BASE_PATH || '/';

  return defineConfig({
    base: basePath,
    plugins: [
      react({
        babel: {
          plugins: [["formatjs", { ast: true }]],
        },
      }),
      basicSsl(),
    ],
    resolve: {
    alias: {
      '@': '/src', 
    },
  },
    build: {
      outDir: 'build',
      sourcemap: false,
    },
    server: {
      host: 'localhost',
      hmr: mode === 'test' ? false : undefined,
      proxy: bffProxy(),
      port: 3000,
    },
    test: {
      environment: 'jsdom',
      setupFiles: ['./src/test/setup.ts'],
      globals: true,
      // Baseline mode for client tests: GraphQL unless a test opts into
      // platform mode via vi.stubEnv('VITE_PLATFORM_API_BASE_URL', ...).
      env: {
        VITE_USE_MOCK_API: 'false',
      },
      // Inline Oxygen UI + its MUI deps so Vite transforms them (and their CSS
      // imports, e.g. @mui/x-data-grid/esm/index.css) instead of letting Node
      // try to import `.css` files directly, which it can't.
      server: {
        deps: {
          inline: [/@wso2\/oxygen-ui/, /@mui\//],
        },
      },
      coverage: {
        provider: 'v8',
        reporter: ['text', 'text-summary', 'html', 'lcov'],
        reportsDirectory: './coverage',
        all: true,
        include: ['src/**/*.{ts,tsx}'],
        exclude: [
          'src/**/*.test.{ts,tsx}',
          'src/test/**',
          'src/api/mocks/**',
          'src/**/*.d.ts',
          'src/main.tsx',
          'src/App.tsx',
          'src/config/loadRuntimeConfigScripts.ts',
          'src/api/generated/**',
          'src/**/index.ts',
          'src/types/**',
        ],
        // Floor set just below the current baseline so the build fails on
        // regression but not on today's coverage. Ratchet these up as page
        // tests land. Re-baselined after the BFF auth migration deleted
        // ~700 lines of (tested) Asgardeo/Thunder/local-file adapter code —
        // AuthProvider.tsx itself is now at ~92%/100% (statements/functions);
        // the drop is LoginPage.tsx and ProductActivation.tsx, both untested
        // before this migration too (no regression, just a smaller
        // denominator). Floors sit a few points below the observed run — v8
        // coverage attribution varies run-to-run because the client-mode
        // tests use vi.resetModules(), so keep a margin so CI doesn't flake.
        thresholds: {
          statements: 56,
          branches: 73,
          functions: 57,
          lines: 56,
        },
      },
    },
  });
};
