<script setup lang="ts">
// Ô "New …" cuối CardGrid: viền đứt, cao bằng các thẻ cùng hàng (lưới kéo giãn)
defineProps<{ label: string }>()
defineEmits<{ click: [] }>()
</script>

<template>
  <button type="button" class="add-card" @click="$emit('click')">
    <i class="pi pi-plus" aria-hidden="true" />
    <span>{{ label }}</span>
  </button>
</template>

<style scoped>
.add-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  min-height: 8rem;
  padding: 1rem;
  border: 1px dashed var(--app-line);
  border-radius: 12px;
  background: transparent;
  color: var(--p-text-muted-color);
  font: inherit;
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    color 0.15s ease;
}
.add-card:hover,
.add-card:focus-visible {
  border-color: var(--app-accent);
  color: var(--p-text-color);
}
</style>
