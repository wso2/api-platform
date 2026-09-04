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
import fs from 'node:fs'
import path from 'path'
import { fileURLToPath } from 'url'

import { BASE_PATH } from './src/paths'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const repoRoot = path.resolve(__dirname, '../../..')
const cloudPluginsRoot = path.resolve(__dirname, '../cloud-plugins')
const insightsLocalEntry = path.resolve(
  cloudPluginsRoot,
  'apip-cloud-ui-insights/src/index.ts'
)
// Monorepo: alias to the sibling plugin source. Docker SaaS images only copy
// portals/ai-workspace into /web and install Insights into node_modules — the
// sibling path is absent there, so skip the alias and let Node resolve the package.
const insightsAlias = fs.existsSync(insightsLocalEntry)
  ? { '@wso2-enterprise/apip-cloud-ui-insights': insightsLocalEntry }
  : {}
const rushTemp = path.resolve(repoRoot, 'common/temp')
const aiTemp = path.resolve(rushTemp, 'ai-workspace')
const aiNodeModules = path.resolve(aiTemp, 'node_modules')
const aiPnpm = path.resolve(aiNodeModules, '.pnpm')

// URL path prefix the app is served under, with the trailing slash Vite's `base` wants.
// Taken from the SPA's own BASE_PATH constant (src/paths.ts) so the prefix baked into
// the bundle and the prefix the app code composes URLs from are one value, not two that
// could drift. It has to be baked in at all because index.html references its assets by
// absolute path: a bundle built for one prefix and served under another 404s on every
// asset, so this is a fixed contract rather than a deployment knob. It must also stay in
// lockstep with the BFF's paths.Base (bff/internal/paths/paths.go), which mounts every
// server route — SPA, auth endpoints, runtime-config.js, the proxy — under the same prefix.
const basePath = `${BASE_PATH}/`

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
        // The base path is part of the address to open — the origin root only
        // redirects there in the BFF, not in this dev server.
        centerInBanner(`Visit ${scheme}://localhost:${port}${basePath}`) + '\n' +
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
// namespace (e.g. APIP_AIW_AUTH_OIDC_CLIENT_SECRET) into the bundle if set at build time.
const browserSafeEnvVars = [
  'APIP_AIW_AUTH_MODE',
  'APIP_AIW_DEFAULT_ORG_REGION',
  'APIP_AIW_GATEWAY_CONTROLPLANE_HOST',
  'APIP_AIW_GATEWAY_PLATFORM_GATEWAY_VERSIONS',
  'APIP_AIW_LOGGING_BROWSER_DEBUG',
  'APIP_AIW_AUTH_OIDC_SCOPE',
  'APIP_AIW_AUTH_CLAIM_MAPPINGS_',   // all claim-name mappings, no secrets share this
  'APIP_AIW_DEV_PORTAL_BASE_URL',
  'APIP_AIW_API_POLICY_HUB',
  'APIP_AIW_POLICY_HUB_WEB_URL',
  'APIP_AIW_MOESIF_WEB_URL',
  'APIP_AIW_MOESIF_APP_API_KEY',     // Moesif publishable Application Id
]

// The BFF serves window.__RUNTIME_CONFIG__ from <base>/runtime-config.js, generated per
// request. Because no such file exists on disk, Vite's index.html URL rewriting handles
// a hand-written src for it inconsistently — in a build it leaves the path untouched
// (missing the base path), in dev it prepends the base to an already-prefixed path
// (doubling it). Injecting the tag from a post-enforced hook sidesteps that rewriting
// entirely, so the URL is spelled once, here, and is identical in both modes.
const runtimeConfigScriptPlugin: PluginOption = {
  name: 'runtime-config-script',
  enforce: 'post',
  transformIndexHtml() {
    return [{
      tag: 'script',
      attrs: {
        src: `${basePath}runtime-config.js`,
        onerror: "console.warn('Runtime config not available, using build-time or default values')",
      },
      injectTo: 'head-prepend',
    }]
  },
}

const plugins: PluginOption[] = [
  react() as unknown as PluginOption,
  basicSsl() as unknown as PluginOption,
  readyLogPlugin,
  runtimeConfigScriptPlugin,
]

export default defineConfig({
  plugins,
  base: basePath,
  // Expose only the allowlisted APIP_AIW_ variables to client code via
  // import.meta.env, instead of Vite's default VITE_ prefix. The whole platform
  // namespaces its configuration this way (APIP_AIW_ here, APIP_CP_ for the Platform
  // API, APIP_AP_ for the API Portal), and the BFF serves the same names in
  // window.__RUNTIME_CONFIG__, so one key spelling works at build time and at runtime.
  envPrefix: browserSafeEnvVars,
  resolve: {
    dedupe: ['react', 'react-dom', 'react/jsx-runtime', 'react/jsx-dev-runtime'],
    alias: {
      ...insightsAlias,
    },
  },
  server: {
    port: 9643,
    fs: {
      allow: [
        path.resolve(__dirname),
        ...(fs.existsSync(cloudPluginsRoot) ? [cloudPluginsRoot] : []),
        repoRoot,
        rushTemp,
        aiTemp,
        aiNodeModules,
        aiPnpm
      ]
    },
    // In dev, run the BFF locally (default https://localhost:8081) and route all
    // same-origin BFF traffic to it, mirroring the production topology. `make bff-run`
    // starts it against configs/config.toml, whose {{ env }} tokens read the APIP_AIW_*
    // variables (CONTROL_PLANE_URL, SERVER_PORT, ...). The BFF serves these routes under
    // the same prefix, so each is forwarded prefix and all, with no rewriting either side.
    proxy: Object.fromEntries(
      ['api', 'proxy', 'runtime-config.js'].map((route) => [
        `${basePath}${route}`,
        {
          target: process.env.BFF_DEV_TARGET || 'https://localhost:8081',
          changeOrigin: true,
          secure: false,        // accept the BFF self-signed cert in dev
        },
      ]),
    ),
  }
})
