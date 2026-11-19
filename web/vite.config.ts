import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            { name: 'vendor', test: /\/node_modules\/(react|react-dom|react-router|react-router-dom|scheduler)\// },
            { name: 'ui', test: /\/node_modules\/(lucide-react|clsx|tailwind-merge)\// },
            { name: 'three', test: /\/node_modules\/three\// },
            { name: 'chart', test: /\/node_modules\/(chart.js|react-chartjs-2)\// },
          ],
        },
      },
    },
    chunkSizeWarningLimit: 600,
  },
  server: {
    host: '0.0.0.0',
    port: 5174,
    proxy: {
      '/api': {
        target: 'http://localhost:8084',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8084',
        ws: true,
      },
    },
  },
})
