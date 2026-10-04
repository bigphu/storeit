<script setup lang="ts">
import Button from 'primevue/button'

// Thanh "chưa lưu" dính ở đáy trang khi có thay đổi: Discard, Save. blocked: không lưu được
// (vd luật lock-out), lý do hiện ở chỗ khác trên trang.
withDefaults(defineProps<{ message: string; saveLabel?: string; saving?: boolean; blocked?: boolean }>(), {
  saveLabel: 'Save',
})
defineEmits<{ save: []; discard: [] }>()
</script>

<template>
  <div class="save-bar" role="region" aria-label="Unsaved changes">
    <span class="save-dot" aria-hidden="true" />
    <span class="save-message">{{ message }}</span>
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
