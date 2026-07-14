import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const electron = process.env.ELECTRON === '1'

export default defineConfig({
  plugins: [vue()],
  base: electron ? './' : '/',
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': 'http://localhost:8080',
      '/uploads': 'http://localhost:8080',
    },
  },
})
