import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { resolve } from 'path'

export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      // Proxy API requests to the Go backend in development
      '/api': {
        target: 'http://localhost:5888',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://localhost:5888',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('chart.js')) return 'charts'
          if (id.includes('node_modules/vue') || id.includes('vue-router') || id.includes('pinia')) {
            return 'vendor'
          }
        },
      },
    },
  },
})
