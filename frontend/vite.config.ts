import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { FontaineTransform } from 'fontaine'
import type { Plugin } from 'vite'
import { defineConfig } from 'vitest/config'

// Preload font mà lần vẽ đầu tiên đã cần (chữ thường, tiêu đề), để trình duyệt tải ngay
// cùng CSS thay vì đợi dựng xong trang mới phát hiện. Chỉ khi build: tên file có hash
function preloadFonts(patterns: RegExp[]): Plugin {
  return {
    name: 'storeit-preload-fonts',
    apply: 'build',
    transformIndexHtml: {
      order: 'post',
      handler(_html, ctx) {
        return Object.keys(ctx.bundle ?? {})
          .filter((file) => patterns.some((p) => p.test(file)))
          .map((file) => ({
            tag: 'link',
            attrs: { rel: 'preload', href: `/${file}`, as: 'font', type: 'font/woff2', crossorigin: '' },
            injectTo: 'head' as const,
          }))
      },
    },
  }
}

// Dev chạy cổng 3000 (link trong email trỏ về đây) và chuyển /api sang backend:
// cùng origin nên cookie refresh (SameSite=Strict, Path=/api/v1/auth) hoạt động,
// backend không cần CORS.
export default defineConfig({
  plugins: [
    vue(),
    // Font dự phòng có số đo (size-adjust, ascent…) khớp font web: khi font web tải xong và
    // thay vào, chữ không đổi kích thước nên trang không xô lệch (CLS). Tên "<font> fallback"
    // được thêm vào các biến --app-body/--app-display/--app-mono trong base.css
    FontaineTransform.vite({
      fallbacks: {
        'Be Vietnam Pro': ['Segoe UI', 'Helvetica Neue', 'Arial'],
        'Bricolage Grotesque Variable': ['Segoe UI', 'Helvetica Neue', 'Arial'],
        'JetBrains Mono': ['Consolas', 'Menlo', 'Courier New'],
      },
      resolvePath: (id) => new URL(`./public${id}`, import.meta.url),
      // font biểu tượng không cần chữ dự phòng
      skipFontFaceGeneration: (name) => name.startsWith('primeicons'),
    }),
    preloadFonts([/be-vietnam-pro-latin-400-normal-[\w-]+\.woff2$/, /bricolage-grotesque-latin-wght-normal-[\w-]+\.woff2$/]),
  ],
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
    // Tên miền ngoài localhost/IP (vd mở qua reverse proxy, tunnel), ngăn bằng dấu phẩy
    allowedHosts: process.env.VITE_ALLOWED_HOSTS?.split(',').map((h) => h.trim()).filter(Boolean),
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})
