<script setup lang="ts">
import AppIcon from '@/components/AppIcon.vue'
import type { IconName } from '@/components/icons'

// Kết quả trên trang công khai ("Check your email", "This link no longer works"): bong bóng
// biểu tượng màu, tiêu đề, một câu giải thích (slot), các nút bước tiếp (slot #actions)
defineProps<{ icon: IconName; tone: 'info' | 'warn' | 'brand'; title: string }>()
</script>

<template>
  <div class="result" :class="tone" role="status">
    <span class="bubble"><AppIcon :name="icon" /></span>
    <h2>{{ title }}</h2>
    <p><slot /></p>
    <div class="result-actions"><slot name="actions" /></div>
  </div>
</template>

<style scoped>
.result {
  --tone: var(--app-info);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  text-align: center;
}
.result.warn {
  --tone: var(--app-warn);
}
.result.brand {
  --tone: var(--app-brand);
}
.bubble {
  display: grid;
  place-items: center;
  width: 3.4rem;
  height: 3.4rem;
  border-radius: 50%;
  background: var(--tone);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--tone) 30%, transparent);
  color: var(--app-on-color);
  font-size: 1.5rem;
  margin-bottom: 0.25rem;
}
h2 {
  font: 800 1.35rem var(--app-display);
  margin: 0;
}
p {
  margin: 0;
  color: var(--p-text-muted-color);
}
.result-actions {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  align-self: stretch;
  margin-top: 0.5rem;
}
</style>
