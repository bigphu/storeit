<script setup lang="ts">
// Đầu trang danh sách: tiêu đề, một dòng mô tả, nút hành động chính bên phải
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <div class="page-header">
    <div class="page-title">
      <h1>{{ title }}</h1>
      <p v-if="subtitle" class="page-subtitle">{{ subtitle }}</p>
    </div>
    <div v-if="$slots.default" class="actions"><slot /></div>
  </div>
</template>

<style scoped>
.page-title {
  min-width: 0;
}
.page-subtitle {
  margin: 0.2rem 0 0;
  color: var(--p-text-muted-color);
}
</style>
