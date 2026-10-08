import { VueQueryPlugin } from '@tanstack/vue-query'
import { createPinia } from 'pinia'
import 'primeicons/primeicons.css'
// Font đóng gói theo app (không tải từ Google): chữ, tiêu đề, mã
import '@fontsource/be-vietnam-pro/400.css'
import '@fontsource/be-vietnam-pro/500.css'
import '@fontsource/be-vietnam-pro/600.css'
import '@fontsource/be-vietnam-pro/700.css'
import '@fontsource-variable/bricolage-grotesque'
import '@fontsource/jetbrains-mono/400.css'
import '@fontsource/jetbrains-mono/500.css'
import '@fontsource/jetbrains-mono/600.css'
import PrimeVue from 'primevue/config'
import ConfirmationService from 'primevue/confirmationservice'
import ToastService from 'primevue/toastservice'
import Tooltip from 'primevue/tooltip'
import { createApp } from 'vue'
import App from './App.vue'
import './app/base.css'
import { router } from './app/router'
import { StoreItPreset } from './app/theme'
import { queryClient } from './lib/query'

// CSS chính tải không chặn (vite.config.ts: nonBlockingCss); đợi nó xong rồi mới mount để app
// không hiện khi chưa có style. Trong lúc đợi, khung tĩnh của index.html vẫn hiện; router đã
// bắt đầu tải code và dữ liệu của trang từ lúc .use(router)
function appCssReady(): Promise<void> {
  const link = document.getElementById('app-css') as HTMLLinkElement | null
  if (!link || link.sheet) return Promise.resolve()
  return new Promise((resolve) => {
    link.addEventListener('load', () => resolve(), { once: true })
    link.addEventListener('error', () => resolve(), { once: true })
  })
}

const app = createApp(App)
  .use(createPinia())
  .use(router)
  .use(VueQueryPlugin, { queryClient })
  .use(PrimeVue, { theme: { preset: StoreItPreset, options: { darkModeSelector: '.app-dark' } } })
  .use(ToastService)
  .use(ConfirmationService)
  // v-tooltip: nhãn cho các nút chỉ có biểu tượng (hành động trên dòng của bảng)
  .directive('tooltip', Tooltip)
void appCssReady().then(() => app.mount('#app'))
