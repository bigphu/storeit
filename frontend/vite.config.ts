import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

// Dev chạy cổng 3000 (link trong email trỏ về đây) và chuyển /api sang backend:
// cùng origin nên cookie refresh (SameSite=Strict, Path=/api/v1/auth) hoạt động,
// backend không cần CORS.
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  // File build vào /static/, không phải /assets/ mặc định: /assets/... là route của SPA
  // (trang tài sản), nginx phải trả index.html cho chúng
  build: { assetsDir: 'static' },
  server: {
    port: 3000,
    strictPort: true,
    proxy: {
      '/api': { target: process.env.API_URL ?? 'http://localhost:8080', changeOrigin: false },
    },
    // Trong container (compose.yml): bind mount không phát sự kiện file, nên dùng
    // polling; HMR nối về port publish trên host
    watch: process.env.VITE_USE_POLLING === 'true' ? { usePolling: true, interval: 300 } : undefined,
    hmr: process.env.VITE_HMR_CLIENT_PORT ? { clientPort: Number(process.env.VITE_HMR_CLIENT_PORT) } : undefined,
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})
