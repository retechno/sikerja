import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

// Di dev, /api diteruskan ke backend sehingga browser dan API satu origin (cookie SameSite=Lax berjalan tanpa CORS).
const apiTarget = process.env.VITE_API_PROXY || 'http://localhost:8080'

export default defineConfig({
  plugins: [vue()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: { host: true, port: 5173, proxy: { '/api': { target: apiTarget, changeOrigin: false } } },
  test: {
    environment: 'jsdom',
    globals: true,
    include: ['src/**/*.test.ts'],
    css: false,
  },
})
