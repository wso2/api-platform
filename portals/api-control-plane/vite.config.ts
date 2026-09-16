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
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig, loadEnv, type ProxyOptions } from 'vite';

/** This file's own directory; `__dirname` doesn't exist in an ES module. */
const projectRoot = path.dirname(fileURLToPath(import.meta.url));

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
          plugins: [['formatjs', { ast: true }]],
        },
      }),
      basicSsl(),
    ],
    resolve: {
      alias: {
        // Mirrors the `@/*` path mapping in tsconfig.json. Resolved to an
        // absolute path rather than the root-relative '/src', which would
        // depend on Vite's own root handling.
        '@': path.resolve(projectRoot, 'src'),
      },
    },
    build: {
      outDir: 'build',
      sourcemap: false,
    },
    // Vite's default IIFE worker format can't bundle a worker that itself
    // code-splits into multiple chunks — `monaco-graphql`'s worker (pulled
    // in transitively by `graphiql`'s Vite worker setup) does. ES module
    // workers support code-splitting and are supported by every browser this
    // app already targets, so this covers both that worker and the existing
    // `monaco-editor` ones in `SpecCodeEditor.tsx` uniformly.
    worker: {
      format: 'es',
    },
    optimizeDeps: {
      // `dev`-only: the dependency pre-bundling scan is a raw esbuild pass
      // without Vite's own plugin pipeline, so it can't resolve the `?worker`
      // suffix inside `graphiql`'s bundled worker-setup module and crashes
      // the dev server on startup. Excluding just the two worker-setup
      // specifiers (not all of `graphiql`/`@graphiql/react`) stops the
      // scanner from parsing into that one file, while still letting it
      // crawl the rest of `@graphiql/react` normally — excluding the whole
      // package here previously also stopped Vite from discovering and
      // pre-bundling its CJS `react-compiler-runtime` dependency, which then
      // reached the browser unconverted (no named exports) and broke the
      // console with "does not provide an export named 'c'". `npm run build`
      // already works without any of this, since Rollup (not esbuild) drives
      // that path.
      exclude: ['graphiql/setup-workers/vite', '@graphiql/react/setup-workers/vite'],
      // `react-compiler-runtime` is only reached deep inside `@graphiql/
      // react`'s components, which the dependency scanner never crawls into
      // (that route is lazy-loaded, and the two `exclude` entries above stop
      // the scanner from following into `@graphiql/react` for the worker
      // files) — so without `include`, Vite never optimizes it at all and
      // serves the raw CJS file byte-for-byte, which has no `export`
      // statements. `include` forces it into the optimizer; `needsInterop`
      // is required alongside it because the package's own shipped bundle
      // sets its exports via a dynamic `__export(index_exports, {...})`
      // helper that esbuild's static named-export detection can't see
      // through on its own, which otherwise collapses the optimized output
      // to a single `export default` — breaking every `import { c } from
      // 'react-compiler-runtime'` inside `@graphiql/react`'s own ESM source
      // ("does not provide an export named 'c'"). Together they make Vite
      // actually `require()` it in Node and re-export every real key.
      include: ['react-compiler-runtime'],
      needsInterop: ['react-compiler-runtime'],
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
