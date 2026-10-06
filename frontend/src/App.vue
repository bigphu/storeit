<script setup lang="ts">
import Button from 'primevue/button'
import ConfirmDialog from 'primevue/confirmdialog'
import Toast, { type ToastMessageOptions } from 'primevue/toast'
import { useToast } from 'primevue/usetoast'
import { onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useSession } from '@/lib/auth/session'
import { usePreferences } from '@/lib/preferences'
import { type NoticeAction, onNotice } from '@/lib/notify'

// Thông báo có thể kèm một nút (vd "What’s inside"); nút chạy rồi đóng thông báo
type NoticeMessage = ToastMessageOptions & { action?: NoticeAction }
const toast = useToast()
const off = onNotice((n) => toast.add({ severity: n.severity, summary: n.summary, life: n.action ? 8000 : 5000, action: n.action } as NoticeMessage))
const ICONS: Record<string, string> = { success: 'pi pi-check-circle', info: 'pi pi-info-circle', warn: 'pi pi-exclamation-triangle', error: 'pi pi-times-circle' }
function runAction(m: NoticeMessage) {
  m.action?.run()
  toast.remove(m)
}
onUnmounted(off)

// Phiên hết hạn (refresh hỏng hoặc tab khác đăng xuất): về trang đăng nhập
const router = useRouter()
// nạp tuỳ chọn ngay từ đầu để theme áp cả trang đăng nhập
usePreferences()
useSession().setOnExpired(() => {
  const current = router.currentRoute.value
  if (!current.matched.some((r) => r.meta.public)) {
    router.push({ name: 'login', query: { redirect: current.fullPath } })
  }
})
</script>

<template>
  <RouterView />
  <Toast>
    <template #message="{ message }">
      <i :class="['notice-icon', ICONS[message.severity ?? 'info']]" aria-hidden="true" />
      <div class="notice-text">
        <span class="p-toast-summary">{{ message.summary }}</span>
        <Button v-if="(message as NoticeMessage).action" :label="(message as NoticeMessage).action!.label" text size="small" class="notice-action" @click="runAction(message as NoticeMessage)" />
      </div>
    </template>
  </Toast>
  <ConfirmDialog />
</template>

<style scoped>
.notice-icon {
  font-size: 1.15rem;
  margin-top: 0.1rem;
}
.notice-text {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.2rem;
  flex: 1;
  min-width: 0;
}
.notice-action {
  padding: 0;
  font-weight: 600;
}
</style>
