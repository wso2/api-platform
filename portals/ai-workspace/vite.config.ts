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

const readyLogPlugin: PluginOption = {
  name: 'ready-log',
  configureServer(server) {
    server.httpServer?.once('listening', () => {
      console.log(
        '\n\n' +
        '========================================================================\n' +
        '\n' +
        '\n' +
        '                      AI Workspace Started\n' +
        '\n' +
        '\n' +
        '========================================================================\n' +
        '\n'
      )
    })
  },
}

const plugins: PluginOption[] = [
  react() as unknown as PluginOption,
  basicSsl() as unknown as PluginOption,
  readyLogPlugin,
]

export default defineConfig({
  plugins,
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
      '/api-proxy': {
        target: 'https://localhost:9243',
        changeOrigin: true,
        secure: false,          // accept self-signed cert on the platform API
        rewrite: (p) => p.replace(/^\/api-proxy/, ''),
      },
    },
  }
})
