import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 开发期代理 /api → 本地 Go 服务（aiclient serve）
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
