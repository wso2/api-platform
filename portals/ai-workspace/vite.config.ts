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

import { defineConfig } from 'vite'
import type { PluginOption } from 'vite'
import react from '@vitejs/plugin-react'
import basicSsl from '@vitejs/plugin-basic-ssl'
import path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const repoRoot = path.resolve(__dirname, '../../..')
const rushTemp = path.resolve(repoRoot, 'common/temp')
const aiTemp = path.resolve(rushTemp, 'ai-workspace')
const aiNodeModules = path.resolve(aiTemp, 'node_modules')
const aiPnpm = path.resolve(aiNodeModules, '.pnpm')

const BANNER_WIDTH = 72

const centerInBanner = (s: string): string =>
  s.length >= BANNER_WIDTH ? s : ' '.repeat(Math.floor((BANNER_WIDTH - s.length) / 2)) + s

const readyLogPlugin: PluginOption = {
  name: 'ready-log',
  configureServer(server) {
    server.httpServer?.once('listening', () => {
      const address = server.httpServer?.address()
      // A wildcard or unknown listen host is not clickable — show localhost instead.
      const port = typeof address === 'object' && address ? address.port : server.config.server.port
      const scheme = server.config.server.https ? 'https' : 'http'
      const rule = '='.repeat(BANNER_WIDTH)
      console.log(
        '\n\n' +
        rule + '\n' +
        '\n' +
        centerInBanner('AI Workspace Started') + '\n' +
        centerInBanner(`Visit ${scheme}://localhost:${port}`) + '\n' +
        '\n' +
        rule + '\n' +
        '\n'
      )
    })
  },
}

// Browser-safe environment variables exposed to client code via import.meta.env.
// This mirrors the BFF's runtime allowlist (bff/internal/config/runtime_config.go
// browserSafeKeys): the same APIP_AIW_ names work at build time and at runtime, but
// only these — a blanket 'APIP_AIW_' prefix would also inline secrets that share the
// namespace (e.g. APIP_AIW_OIDC_CLIENT_SECRET) into the bundle if set at build time.
const browserSafeEnvVars = [
  'APIP_AIW_DOMAIN',
  'APIP_AIW_AUTH_MODE',
  'APIP_AIW_DEFAULT_ORG_REGION',
  'APIP_AIW_CONTROLPLANE_HOST',
  'APIP_AIW_PLATFORM_GATEWAY_VERSIONS',
  'APIP_AIW_CSRF_HEADER',
  'APIP_AIW_DEBUG',
  'APIP_AIW_OIDC_SCOPE',
  'APIP_AIW_OIDC_CLAIM_MAPPINGS_',   // all claim-name mappings, no secrets share this
  'APIP_AIW_DEV_PORTAL_BASE_URL',
  'APIP_AIW_API_POLICY_HUB',
  'APIP_AIW_POLICY_HUB_WEB_URL',
  'APIP_AIW_MOESIF_WEB_URL',
  'APIP_AIW_MOESIF_APP_API_KEY',     // Moesif publishable Application Id
  'APIP_AIW_PLATFORM_API_BASE_URL',
  'APIP_AIW_PORTAL_API_BASE_URL',
]

const plugins: PluginOption[] = [
  react() as unknown as PluginOption,
  basicSsl() as unknown as PluginOption,
  readyLogPlugin,
]

export default defineConfig({
  plugins,
  // Expose only the allowlisted APIP_AIW_ variables to client code via
  // import.meta.env, instead of Vite's default VITE_ prefix. The whole platform
  // namespaces its configuration this way (APIP_AIW_ here, APIP_CP_ for the Platform
  // API, APIP_DP_ for the Developer Portal), and the BFF serves the same names in
  // window.__RUNTIME_CONFIG__, so one key spelling works at build time and at runtime.
  envPrefix: browserSafeEnvVars,
  resolve: {
    dedupe: ['react', 'react-dom', 'react/jsx-runtime', 'react/jsx-dev-runtime'],
  },
  server: {
    port: 5380,
    fs: {
      allow: [
        path.resolve(__dirname),
        repoRoot,
        rushTemp,
        aiTemp,
        aiNodeModules,
        aiPnpm
      ]
    },
    proxy: {
      // In dev, run the BFF locally (default https://localhost:8081) and route
      // all same-origin BFF traffic to it, mirroring the production topology.
      // `make bff-run` starts it against configs/config.toml, whose {{ env }} tokens read
      // the APIP_AIW_* variables (PLATFORM_API_URL, LISTEN_ADDR, ...).
      '/api': {
        target: process.env.BFF_DEV_TARGET || 'https://localhost:8081',
        changeOrigin: true,
        secure: false,          // accept the BFF self-signed cert in dev
      },
      '/runtime-config.js': {
        target: process.env.BFF_DEV_TARGET || 'https://localhost:8081',
        changeOrigin: true,
        secure: false,
      },
    },
  }
})
