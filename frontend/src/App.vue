<script setup lang="ts">
import ConfirmDialog from 'primevue/confirmdialog'
import Toast, { type ToastMessageOptions } from 'primevue/toast'
import { useToast } from 'primevue/usetoast'
import { onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import NoticeCard from '@/components/NoticeCard.vue'
import { useSession } from '@/lib/auth/session'
import { isUndoShortcut, latestUndo, type Notice, onNotice } from '@/lib/notify'
import { usePreferences } from '@/lib/preferences'

// Thông báo: thẻ riêng (NoticeCard) trong Toast của PrimeVue; tối đa 3, mới nhất ở dưới.
// Toast không tự đóng (không có life): NoticeCard tự đếm giờ rồi gọi closeCallback
const MAX_NOTICES = 3
type NoticeMessage = ToastMessageOptions & { notice: Notice }
const toast = useToast()
const shown: NoticeMessage[] = []
const off = onNotice((n) => {
  const m: NoticeMessage = { severity: n.severity, notice: n }
  shown.push(m)
  while (shown.length > MAX_NOTICES) toast.remove(shown.shift()!)
  toast.add(m)
})
// Toast gắn id vào message khi thêm (kiểu không khai báo id); so theo id vì Toast trả lại
// bản proxy
const idOf = (m: ToastMessageOptions) => (m as { id?: unknown }).id
function forget(e: { message: ToastMessageOptions }) {
  const i = shown.findIndex((m) => idOf(m) === idOf(e.message))
  if (i >= 0) shown.splice(i, 1)
}
// Ctrl/⌘ Z: Undo của thông báo mới nhất còn hiện (không khi đang gõ)
function onKey(e: KeyboardEvent) {
  if (!isUndoShortcut(e)) return
  const undo = latestUndo()
  if (!undo) return
  e.preventDefault()
  undo()
}
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => {
  off()
  window.removeEventListener('keydown', onKey)
})

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
  <Toast position="bottom-right" :pt="{ root: { class: 'app-notices' } }" @close="forget">
    <template #container="{ message, closeCallback }">
      <NoticeCard :notice="(message as NoticeMessage).notice" @close="closeCallback" />
    </template>
  </Toast>
  <ConfirmDialog />
</template>

<style>
/* Toast của PrimeVue chỉ giữ chỗ và vị trí; thẻ do NoticeCard vẽ */
.app-notices {
  width: auto;
  max-width: calc(100vw - 32px);
}
.app-notices .p-toast-message {
  background: none;
  border: 0;
  box-shadow: none;
  backdrop-filter: none;
  margin: 0 0 0.5rem;
}
</style>
