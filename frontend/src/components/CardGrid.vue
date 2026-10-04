<script setup lang="ts">
// Lưới thẻ (loại tài sản, vai trò): cột tự co giãn, mọi thẻ trong một hàng cao bằng nhau
withDefaults(defineProps<{ min?: string }>(), { min: '16rem' })
</script>

<template>
  <div class="card-grid" :style="{ '--card-min': min }"><slot /></div>
</template>

<style scoped>
.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(var(--card-min), 100%), 1fr));
  gap: 0.9rem;
}
</style>
