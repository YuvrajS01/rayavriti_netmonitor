import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  cacheDir: '../node_modules/.vite/client',
  build: {
    outDir: '../server/dist/public',
    emptyOutDir: true,
    sourcemap: true,
    rollupOptions: {
      output: {
        manualChunks: (id: string) => {
          if (id.includes('react-router-dom') || id.includes('react-dom') || id.includes('/react/')) return 'vendor';
          if (id.includes('@reduxjs/toolkit') || id.includes('react-redux')) return 'redux';
          if (id.includes('recharts')) return 'charts';
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:3000',
        changeOrigin: true,
      },
      '/socket.io': {
        target: 'http://localhost:3000',
        ws: true,
      },
    },
  },
})

