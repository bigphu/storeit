<script setup lang="ts">
import Button from 'primevue/button'
import Drawer from 'primevue/drawer'
import { onMounted, onUnmounted } from 'vue'
import { closeGuard } from '@/lib/confirm'
import AppIcon from './AppIcon.vue'
import type { IconName } from './icons'

// Sửa nhanh từ danh sách: ngăn kéo bên phải (chuột ít phải di), cùng form với Overview của
// trang chi tiết. ↑/↓ (khi không đang gõ) sang dòng trước/sau; Ctrl/⌘ S lưu; còn thay đổi
// chưa lưu thì hỏi trước khi đóng (cha tự hỏi khi chuyển dòng)
const props = defineProps<{ title: string; icon: IconName; dirty: boolean; busy?: boolean; canPrev: boolean; canNext: boolean }>()
const visible = defineModel<boolean>('visible', { required: true })
const emit = defineEmits<{ save: []; prev: []; next: []; openPage: [] }>()

const mayClose = closeGuard()
async function requestClose() {
  if (await mayClose(props.dirty)) visible.value = false
}
function onKey(e: KeyboardEvent) {
  if (!visible.value) return
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
    e.preventDefault()
    if (props.dirty && !props.busy) emit('save')
    return
  }
  const typing = (e.target as HTMLElement | null)?.closest?.('input, textarea, select, [contenteditable], .p-select-overlay, .p-datepicker-panel')
  if (typing) return
  if (e.key === 'ArrowDown' && props.canNext) {
    e.preventDefault()
    emit('next')
  }
  if (e.key === 'ArrowUp' && props.canPrev) {
    e.preventDefault()
    emit('prev')
  }
}
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <Drawer
    :visible="visible"
    position="right"
    :modal="false"
    :show-close-icon="false"
    :pt="{
      root: { class: 'quick-edit', style: 'width: min(26rem, 100vw)' },
      header: { class: 'qe-header' },
      content: { class: 'qe-body' },
      footer: { class: 'qe-footer' },
    }"
    @update:visible="(v: boolean) => !v && requestClose()"
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
    <p class="qe-hint">↑/↓ moves to the previous or next row.</p>
    <template #footer>
      <Button label="Open full page" icon="pi pi-arrow-right" icon-pos="right" text @click="emit('openPage')" />
      <span class="qe-grow" />
      <Button label="Save" :loading="busy" :disabled="!dirty" @click="emit('save')" />
    </template>
  </Drawer>
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
