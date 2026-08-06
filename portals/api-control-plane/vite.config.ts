import {
  copyFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
} from 'node:fs';
import { resolve } from 'node:path';
import basicSsl from '@vitejs/plugin-basic-ssl';
import react from '@vitejs/plugin-react';
import { defineConfig, loadEnv, type Plugin, type ProxyOptions } from 'vite';

const legacyConfigFiles = [
  'api-platform.env.config.js',
  'api-platform.common.config.js',
];
const getLegacyConfigFile = (fileName: string) => {
  // When pointing at a local wso2cloud cluster, use the local override in
  // configs/development/. There is no other source for these files in dev —
  // production instead renders them at container startup (see
  // configs/production/40-config-setup.sh).
  if (process.env.VITE_LOCAL_WSO2CLOUD === 'true') {
    const localFile = resolve(__dirname, 'configs/development', fileName);
    if (existsSync(localFile)) return localFile;
  }
  return undefined;
};

// Dev proxy that forwards same-origin cluster paths to the k3d gateway
// (127.0.0.1:19080), rewriting the Host header so the gateway's HTTPRoutes
// match. Keeps the browser same-origin -> no CORS, no https->http mixed content.
const K3D_GATEWAY = 'http://127.0.0.1:19080';
const CLUSTER_HOSTS = {
  platformApi: 'development-wso2cloud.openchoreoapis.localhost',
  idp: 'platform-idp-development.openchoreoapis.localhost',
};
const proxyWithHost = (host: string): ProxyOptions => ({
  target: K3D_GATEWAY,
  changeOrigin: false,
  secure: false,
  configure: (proxy) => {
    proxy.on('proxyReq', (proxyReq) => proxyReq.setHeader('host', host));
  },
});
const localClusterProxy = () =>
  process.env.VITE_LOCAL_WSO2CLOUD === 'true'
    ? {
        '/oauth2': proxyWithHost(CLUSTER_HOSTS.idp),
        '/.well-known': proxyWithHost(CLUSTER_HOSTS.idp),
        '/platform-api-service-platform-api-endpoint': proxyWithHost(
          CLUSTER_HOSTS.platformApi
        ),
      }
    : undefined;

const normalizeBasePath = (basePath: string) =>
  `/${basePath.replace(/^\/|\/$/g, '')}`;

const getConfigRequestPaths = (basePath: string) => {
  const normalizedBasePath = normalizeBasePath(basePath);
  return new Map(
    legacyConfigFiles.flatMap((fileName) => [
      [`/${fileName}`, fileName],
      [`${normalizedBasePath}/${fileName}`, fileName],
    ])
  );
};

const legacyRuntimeConfigPlugin = (basePath: string): Plugin => ({
  name: 'oxygen-legacy-runtime-config',
  configureServer(server) {
    const configRequestPaths = getConfigRequestPaths(basePath);

    server.middlewares.use((request, response, next) => {
      const requestPath = request.url?.split('?')[0];
      const fileName = requestPath ? configRequestPaths.get(requestPath) : '';
      if (!fileName) {
        next();
        return;
      }

      const legacyConfigFile = getLegacyConfigFile(fileName);

      if (!legacyConfigFile || !existsSync(legacyConfigFile)) {
        next();
        return;
      }

      response.setHeader('Content-Type', 'application/javascript');
      response.end(readFileSync(legacyConfigFile, 'utf8'));
    });
  },
  closeBundle() {
    const normalizedBasePath = normalizeBasePath(basePath);
    for (const fileName of legacyConfigFiles) {
      const source = getLegacyConfigFile(fileName);
      if (!source || !existsSync(source)) continue;

      const rootTarget = resolve(__dirname, 'build', fileName);
      const baseTarget = resolve(
        __dirname,
        'build',
        normalizedBasePath.slice(1),
        fileName
      );

      mkdirSync(resolve(rootTarget, '..'), { recursive: true });
      mkdirSync(resolve(baseTarget, '..'), { recursive: true });
      copyFileSync(source, rootTarget);
      copyFileSync(source, baseTarget);
    }
  },
});

export default ({ mode }: { mode: string }) => {
  process.env = { ...process.env, ...loadEnv(mode, process.cwd()) };
  const basePath = process.env.VITE_APP_BASE_PATH || '/';

  return defineConfig({
    base: basePath,
    plugins: [legacyRuntimeConfigPlugin(basePath), react(), basicSsl()],
    build: {
      outDir: 'build',
      sourcemap: false,
    },
    server: {
      host: 'localhost',
      hmr: mode === 'test' ? false : undefined,
      proxy: localClusterProxy(),
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
