import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'path'

const chunkGroups = [
  { name: 'vendor-react', packages: ['react', 'react-dom', 'react-router-dom'] },
  { name: 'vendor-antd', packages: ['antd', '@ant-design/icons'] },
  { name: 'vendor-query', packages: ['@tanstack/react-query'] },
  { name: 'vendor-charts', packages: ['recharts'] },
]

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': resolve(__dirname, './src'),
      '@shared': resolve(__dirname, './src/shared'),
    },
  },
  build: {
    chunkSizeWarningLimit: 1200,
    rollupOptions: {
      input: {
        main: resolve(__dirname, 'index.html'),
        admin: resolve(__dirname, 'admin.html'),
      },
      output: {
        manualChunks(id) {
          const normalizedId = id.replaceAll('\\', '/')
          return chunkGroups.find((group) =>
            group.packages.some((pkg) => normalizedId.includes(`/node_modules/${pkg}/`)),
          )?.name
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/admin/v1': { target: 'http://localhost:8080', changeOrigin: true },
      '/v1': { target: 'http://localhost:8080', changeOrigin: true },
      '/health': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
