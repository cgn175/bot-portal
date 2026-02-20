import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3100,
    proxy: {
      '/api': 'http://localhost:8080',
      '/tasks': 'http://localhost:8080',
      '/.well-known': 'http://localhost:8080',
    }
  }
})
