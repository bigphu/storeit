<script setup lang="ts">
// Thẻ đầu trang chi tiết (tài khoản, vai trò): hình đại diện, tiêu đề và nhãn, chi tiết,
// hành động bên phải
defineProps<{ title: string }>()
</script>

<template>
  <header class="detail-header">
    <slot name="media" />
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
