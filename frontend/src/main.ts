import { VueQueryPlugin } from '@tanstack/vue-query'
import { createPinia } from 'pinia'
import 'primeicons/primeicons.css'
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
