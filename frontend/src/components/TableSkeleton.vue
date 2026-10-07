<script setup lang="ts">
import Skeleton from 'primevue/skeleton'

// Dòng giả trong lúc trang đầu đang tải (đặt trong #empty của DataTable). Mỗi dòng giả cao
// bằng một dòng thật: danh sách phân trang truyền đủ dòng cho một màn hình (rows) để thanh
// phân trang và phần bên dưới không bị đẩy xuống khi dữ liệu về (CLS)
withDefaults(defineProps<{ rows?: number }>(), { rows: 3 })
</script>

<template>
  <div class="table-skeleton" aria-busy="true" aria-label="Loading">
    <div v-for="i in rows" :key="i" class="sk-row"><Skeleton height="0.9rem" /></div>
  </div>
</template>

<style scoped>
.sk-row {
  display: flex;
  align-items: center;
  height: 2.9rem;
}
</style>
