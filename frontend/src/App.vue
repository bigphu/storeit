<script setup lang="ts">
import Button from 'primevue/button'
import ConfirmDialog from 'primevue/confirmdialog'
import Toast, { type ToastMessageOptions } from 'primevue/toast'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import { onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import AppIcon from '@/components/AppIcon.vue'
import NoticeCard from '@/components/NoticeCard.vue'
import { useSession } from '@/lib/auth/session'
import { bindConfirm, type ConfirmOptions } from '@/lib/confirm'
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

// Hộp xác nhận: confirmAction (lib/confirm.ts) gửi tới ConfirmDialog qua đây
bindConfirm(useConfirm().require)
const view = (m: unknown) => (m as { view: ConfirmOptions }).view

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
  <ConfirmDialog :draggable="false" :pt="{ root: { class: 'app-confirm' } }">
    <template #container="{ message, acceptCallback, rejectCallback }">
      <div :class="['confirm', view(message).danger ? 'danger' : view(message).warn ? 'warn' : 'info']">
        <span class="bubble"><AppIcon :name="view(message).icon" /></span>
        <h2 class="confirm-title">{{ view(message).title }}</h2>
        <p class="confirm-body">{{ view(message).body }}</p>
        <ul v-if="view(message).impact?.length" class="confirm-impact">
          <li v-for="line in view(message).impact" :key="line">{{ line }}</li>
        </ul>
        <div class="confirm-actions">
          <!-- đỏ và cam: focus sẵn ở Cancel, Enter không lỡ tay làm -->
          <Button label="Cancel" severity="secondary" outlined :autofocus="view(message).danger || view(message).warn" @click="rejectCallback" />
          <Button :label="view(message).action" class="go" :autofocus="!view(message).danger && !view(message).warn" @click="acceptCallback" />
        </div>
      </div>
    </template>
  </ConfirmDialog>
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

/* Hộp xác nhận: căn giữa; bong bóng tròn (biểu tượng đặc trắng) nhô nửa lên mép trên, có
   vòng sáng cùng màu; tiêu đề và nút hành động theo loại (đỏ: bỏ; xanh dương: không hoàn
   tác được) */
.app-confirm.p-dialog {
  width: min(26rem, calc(100vw - 2rem));
  margin-top: 2.4rem;
  overflow: visible;
}
.confirm {
  --tone: var(--app-info);
  --tone-strong: var(--app-info-strong);
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.6rem;
  padding: 3.2rem 1.4rem 1.3rem;
  text-align: center;
}
.confirm.danger {
  --tone: var(--app-danger);
  --tone-strong: var(--app-danger-strong);
}
.confirm.warn {
  --tone: var(--app-warn);
  --tone-strong: var(--app-warn-strong);
}
.confirm .bubble {
  position: absolute;
  top: -2.4rem;
  left: 50%;
  transform: translateX(-50%);
  display: grid;
  place-items: center;
  width: 4.8rem;
  height: 4.8rem;
  border-radius: 50%;
  background: var(--tone);
  box-shadow:
    0 0 0 5px color-mix(in srgb, var(--tone) 40%, #ffffff),
    0 8px 20px color-mix(in srgb, var(--tone) 35%, transparent);
  color: var(--app-on-color);
  font-size: 2.2rem;
}
.confirm-title {
  font-size: 1.2rem;
  color: var(--tone-strong);
}
.confirm-body {
  margin: 0;
}
.confirm-impact {
  align-self: stretch;
  margin: 0;
  padding: 0.6rem 0.75rem;
  list-style: none;
  border-radius: 8px;
  background: var(--app-soft);
  font-size: 0.86rem;
}
.confirm-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.5rem;
  align-self: stretch;
  margin-top: 0.4rem;
}
.confirm .go {
  background: var(--tone-strong);
  border-color: var(--tone-strong);
  color: var(--app-on-color);
}
</style>
