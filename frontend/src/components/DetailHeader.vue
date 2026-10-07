<script setup lang="ts">
import AppIcon from './AppIcon.vue'
import type { IconName } from './icons'

// Thẻ đầu trang chi tiết: biểu tượng của khu vực (hay hình đại diện), tiêu đề và nhãn, một
// dòng thông tin, hành động bên phải theo thứ tự cố định (liên kết/hành động chính, đổi trạng
// thái, More, Delete cuối cùng)
defineProps<{ title: string; icon?: IconName }>()
</script>

<template>
  <header class="detail-header">
    <slot name="media"><span v-if="icon" class="glyph"><AppIcon :name="icon" /></span></slot>
    <div class="detail-main">
      <div class="detail-line">
        <h1>{{ title }}</h1>
        <slot name="tags" />
      </div>
      <slot />
    </div>
    <div v-if="$slots.actions" class="actions"><slot name="actions" /></div>
  </header>
</template>

<style scoped>
.detail-header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 1rem;
  padding: 1rem 1.1rem;
  margin-bottom: 1rem;
  border: 1px solid var(--app-line);
  border-radius: 12px;
  background: var(--p-content-background);
}
.glyph {
  display: grid;
  place-items: center;
  width: 2.9rem;
  height: 2.9rem;
  border-radius: 12px;
  background: var(--app-brand-soft);
  color: var(--app-brand-strong);
  font-size: 1.5rem;
  flex: none;
}
.detail-main {
  flex: 1;
  min-width: 12rem;
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  color: var(--p-text-muted-color);
}
.detail-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  color: var(--p-text-color);
}
</style>
