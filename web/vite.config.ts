import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backendUrl = 'http://192.168.178.111:8080'
export default defineConfig({
    plugins: [react()],
    server: {
        port: 3100,
        proxy: {
            '/api': backendUrl,
            '/tasks': backendUrl,
            '/.well-known': backendUrl,
        }
    }
})
