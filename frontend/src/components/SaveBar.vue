<script setup lang="ts">
import Button from 'primevue/button'
import { computed, onActivated, onDeactivated, onMounted, onUnmounted } from 'vue'

// Thanh "chưa lưu" dính ở đáy trang khi có thay đổi: Discard, Save; Ctrl/⌘ S lưu. blocked:
// không lưu được (vd luật lock-out), lý do hiện ở chỗ khác trên trang
const props = withDefaults(defineProps<{ message?: string; count?: number; saveLabel?: string; saving?: boolean; blocked?: boolean }>(), {
  saveLabel: 'Save',
})
const emit = defineEmits<{ save: []; discard: [] }>()
const text = computed(() => props.message ?? `${props.count ?? 0} unsaved ${props.count === 1 ? 'change' : 'changes'}`)

// trang trong KeepAlive vẫn giữ thanh lưu khi tab khác đang mở: chỉ nghe phím khi trang đang hiện
let active = true
function onKey(e: KeyboardEvent) {
  if (!active || !(e.ctrlKey || e.metaKey) || e.key.toLowerCase() !== 's') return
  e.preventDefault()
  if (!props.saving && !props.blocked) emit('save')
}
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
onActivated(() => (active = true))
onDeactivated(() => (active = false))
</script>

<template>
  <div class="save-bar" role="region" aria-label="Unsaved changes">
    <span class="save-dot" aria-hidden="true" />
    <span class="save-message">{{ text }}</span>
    <Button label="Discard" text size="small" class="save-discard" @click="$emit('discard')" />
    <Button :label="saveLabel" size="small" :loading="saving" :disabled="blocked" @click="$emit('save')" />
  </div>
</template>

<style scoped>
.save-bar {
  grid-column: 1 / -1;
  position: sticky;
  bottom: 0.75rem;
  z-index: 2;
  margin-top: 1rem;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.6rem;
  padding: 0.6rem 0.75rem 0.6rem 1rem;
  border-radius: 10px;
  /* màu đảo theo theme: nổi trên nền sáng lẫn tối */
  background: var(--p-text-color);
  color: var(--p-content-background);
  box-shadow: 0 6px 20px rgb(0 0 0 / 0.18);
}
.save-message {
  flex: 1;
  min-width: 8rem;
}
.save-dot {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 50%;
  background: var(--app-accent);
}
.save-discard {
  color: var(--p-content-background);
}
</style>
