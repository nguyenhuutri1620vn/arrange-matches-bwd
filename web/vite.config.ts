import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// In dev the Go server runs on :8080 and Vite proxies the API to it.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: { '/api': 'http://localhost:8080' },
  },
})
