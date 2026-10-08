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

createApp(App)
  .use(createPinia())
  .use(router)
  .use(VueQueryPlugin, { queryClient })
  .use(PrimeVue, { theme: { preset: StoreItPreset, options: { darkModeSelector: '.app-dark' } } })
  .use(ToastService)
  .use(ConfirmationService)
  // v-tooltip: nhãn cho các nút chỉ có biểu tượng (hành động trên dòng của bảng)
  .directive('tooltip', Tooltip)
  .mount('#app')
