import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    lib: {
      entry: 'src/index.js',
      name: 'WebSocketViewer',
      formats: ['es', 'umd'],
      fileName: (format) => `websocket-viewer.${format}.js`
    },
    rollupOptions: {
      external: ['react', 'react-dom', 'react-router-dom'],
      output: {
        globals: {
          react: 'React',
          'react-dom': 'ReactDOM',
          'react-router-dom': 'ReactRouterDOM'
        }
      }
    }
  },
  define: {
    'process.env.NODE_ENV': JSON.stringify('production'),
    process: '{}', // if needed, but usually NODE_ENV is enough
  }
})
