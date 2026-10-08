<script setup lang="ts">
// Phím tắt cạnh nhãn nút ("New asset  N"). Nút tự đặt aria-label có phím ("New asset (N)")
// nên phần này ẩn với trình đọc màn hình. Màn cảm ứng không có bàn phím: ẩn luôn.
defineProps<{ keys: string }>()
</script>

<template>
  <kbd class="key-hint" aria-hidden="true">{{ keys }}</kbd>
</template>

<style scoped>
.key-hint {
  margin-left: 0.1rem;
}
@media (hover: none) {
  .key-hint {
    display: none;
  }
}
</style>
