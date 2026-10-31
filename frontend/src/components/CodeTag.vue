<script setup lang="ts">
// Mã tự sinh (tag tài sản DSK-0022, mã phiếu REQ-0004, mã loại DESK): một kiểu ở mọi nơi, chữ mono
// trên viên xám nhạt; trong liên kết thì lấy màu liên kết
defineProps<{ code: string }>()
</script>

<template>
  <span class="code-tag">{{ code }}</span>
</template>

<style scoped>
.code-tag {
  display: inline-block;
  padding: 0.05rem 0.45rem;
  border-radius: 6px;
  background: var(--app-fill);
  color: var(--p-text-muted-color);
  font: 600 0.78rem var(--app-mono);
  letter-spacing: 0.01em;
  white-space: nowrap;
}
a .code-tag,
a.code-tag {
  color: var(--app-link);
}
</style>
