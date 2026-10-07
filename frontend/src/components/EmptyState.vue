<script setup lang="ts">
import Button from 'primevue/button'

// Danh sách trống: biểu tượng, một câu, tuỳ chọn một hành động
defineProps<{ icon: string; text: string; action?: string }>()
defineEmits<{ action: [] }>()
</script>

<template>
  <div class="empty-state">
    <i :class="icon" aria-hidden="true" />
    <p>{{ text }}</p>
    <Button v-if="action" :label="action" size="small" outlined @click="$emit('action')" />
  </div>
</template>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 2rem 1rem;
  text-align: center;
  color: var(--p-text-muted-color);
}
.empty-state i {
  font-size: 1.6rem;
}
.empty-state p {
  margin: 0;
  max-width: 40ch;
}
</style>
