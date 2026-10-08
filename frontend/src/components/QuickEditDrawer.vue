<script setup lang="ts">
import Button from 'primevue/button'
import { nextTick, onActivated, onDeactivated, onMounted, onUnmounted, watch } from 'vue'
import { isPageClickOutside } from '@/lib/clickAway'
import { closeGuard } from '@/lib/confirm'
import { afterSaveNext, quickEditKey } from '@/lib/quickDrawer'
import AppIcon from './AppIcon.vue'
import KeyHint from './KeyHint.vue'
import SideDrawer from './SideDrawer.vue'
import type { IconName } from './icons'

// Sửa nhanh từ danh sách: ngăn kéo bên phải (chuột ít phải di), cùng form với Overview của
// trang chi tiết. J/K (khi không đang gõ) sang dòng trước/sau; Ctrl/⌘ S lưu; Ctrl/⌘ Enter lưu rồi sang dòng sau; còn thay đổi
// chưa lưu thì hỏi trước khi đóng (cha tự hỏi khi chuyển dòng)
// actionsLabel: nhãn của khối nút hành động (slot #actions), ví dụ "Account"
const props = defineProps<{
  title: string
  icon: IconName
  dirty: boolean
  busy?: boolean
  canPrev: boolean
  canNext: boolean
  actionsLabel?: string
}>()
const visible = defineModel<boolean>('visible', { required: true })
// closed: ngăn kéo đã trượt ra xong (cha bỏ mục đang sửa lúc này, xem lib/quickDrawer.ts)
const emit = defineEmits<{ save: []; prev: []; next: []; openPage: []; closed: [] }>()

const mayClose = closeGuard()
// đang lưu (cả lúc đang hỏi xác nhận trước khi lưu) thì không đóng: Esc trên hộp xác nhận
// cũng tới ngăn kéo
async function requestClose() {
  if (props.busy) return
  if (await mayClose(props.dirty)) visible.value = false
}
// Ctrl Enter đang chờ lần lưu chạy xong để sang dòng sau
let saveNextPending = false
let sawBusy = false
watch(
  () => props.busy,
  async (b) => {
    if (!saveNextPending) return
    if (b) {
      sawBusy = true
      return
    }
    await nextTick() // cha cập nhật dirty sau khi lưu
    const r = afterSaveNext(sawBusy, { busy: !!props.busy, dirty: props.dirty, canNext: props.canNext })
    if (r === 'wait') return
    saveNextPending = false
    sawBusy = false
    if (r === 'next') emit('next')
  },
)
// sang dòng khác bằng cách nào cũng được thì thôi chờ
watch(
  () => props.title,
  () => {
    saveNextPending = false
    sawBusy = false
  },
)

function onKey(e: KeyboardEvent) {
  if (!visible.value) return
  const target = e.target as HTMLElement | null
  const action = quickEditKey(e, {
    dirty: props.dirty,
    busy: !!props.busy,
    canPrev: props.canPrev,
    canNext: props.canNext,
    typing: !!target?.closest?.('input, textarea, select, [contenteditable], .p-select-overlay, .p-datepicker-panel'),
    // Esc trên hộp xác nhận (mở từ nút trong ngăn kéo) chỉ đóng hộp xác nhận
    inDialog: !!target?.closest?.('.p-dialog'),
  })
  if ((e.ctrlKey || e.metaKey) && (e.key.toLowerCase() === 's' || e.key === 'Enter')) e.preventDefault()
  switch (action) {
    case 'close':
      void requestClose()
      break
    case 'save':
      emit('save')
      break
    case 'saveNext':
      saveNextPending = true
      sawBusy = false
      emit('save')
      break
    case 'prev':
      e.preventDefault()
      emit('prev')
      break
    case 'next':
      e.preventDefault()
      emit('next')
      break
  }
}
// Bấm ra ngoài: tự nghe thay cho dismissable của Drawer, vì Drawer coi cả việc chọn trong
// danh sách thả xuống (dựng ngoài ngăn kéo) là bấm ra ngoài
function onClickAway(e: MouseEvent) {
  if (visible.value && isPageClickOutside(e.target)) void requestClose()
}
// Tab bị ẩn (KeepAlive) thì thôi nghe phím và chuột, khỏi xử lý thay tab đang mở
function listen() {
  window.addEventListener('keydown', onKey)
  document.addEventListener('click', onClickAway, true)
}
function unlisten() {
  window.removeEventListener('keydown', onKey)
  document.removeEventListener('click', onClickAway, true)
}
onMounted(listen)
onActivated(listen)
onDeactivated(unlisten)
onUnmounted(unlisten)
</script>

<template>
  <SideDrawer
    :visible="visible"
    position="right"
    :modal="false"
    :dismissable="false"
    :close-on-escape="false"
    :show-close-icon="false"
    root-class="quick-edit"
    header-class="qe-header"
    content-class="qe-body"
    footer-class="qe-footer"
    @update:visible="(v: boolean) => !v && requestClose()"
    @closed="emit('closed')"
  >
    <template #header>
      <span class="qe-icon"><AppIcon :name="icon" /></span>
      <div class="qe-title">
        <h2>Quick edit</h2>
        <span class="qe-name">{{ title }}</span>
      </div>
      <Button icon="pi pi-times" rounded severity="secondary" class="qe-close" aria-label="Close" @click="requestClose" />
    </template>
    <div v-if="$slots.meta" class="qe-meta"><slot name="meta" /></div>
    <slot />
    <!-- hành động trên mục (như nút cuối dòng), chạy ngay, không qua Save -->
    <section v-if="$slots.actions" class="qe-actions">
      <span class="qe-actions-label">{{ actionsLabel ?? 'Actions' }}</span>
      <div class="qe-actions-buttons"><slot name="actions" /></div>
    </section>
    <p class="qe-hint">J / K moves to the previous or next row. Ctrl Enter saves and moves on.</p>
    <template #footer>
      <Button label="Open full page" icon="pi pi-arrow-right" icon-pos="right" text @click="emit('openPage')" />
      <span class="qe-grow" />
      <Button :disabled="!dirty || busy" aria-label="Save (Ctrl S)" @click="emit('save')">
        <i v-if="busy" class="pi pi-spin pi-spinner" aria-hidden="true" />
        <span>Save</span>
        <KeyHint keys="Ctrl S" />
      </Button>
    </template>
  </SideDrawer>
</template>

<style>
/* không scoped: Drawer dựng ngoài cây component */
.quick-edit .qe-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 1rem 1.1rem;
  background: var(--app-soft);
}
.quick-edit .qe-icon {
  color: var(--app-brand);
  font-size: 1.6rem;
}
.quick-edit .qe-title {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.quick-edit .qe-title h2 {
  font-family: var(--app-display);
  font-size: 1.2rem;
  font-weight: 800;
}
.quick-edit .qe-name {
  font-size: 0.85rem;
  color: var(--p-text-muted-color);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.quick-edit .qe-close {
  background: var(--p-content-background);
  color: var(--p-text-color);
  border: 0;
  box-shadow: 0 2px 8px rgb(15 23 42 / 0.15);
}
.quick-edit .qe-body {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.1rem;
}
.quick-edit .qe-actions {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}
.quick-edit .qe-actions-label {
  font-size: 0.85rem;
  color: var(--p-text-muted-color);
}
.quick-edit .qe-actions-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.quick-edit .qe-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
  font-size: 0.85rem;
  color: var(--p-text-muted-color);
}
.quick-edit .qe-hint {
  margin: 0;
  font-size: 0.8rem;
  color: var(--p-text-muted-color);
}
.quick-edit .qe-footer {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 1.1rem;
  border-top: 1px solid var(--app-line);
}
.quick-edit .qe-grow {
  flex: 1;
}
</style>
