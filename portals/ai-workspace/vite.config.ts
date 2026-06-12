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
