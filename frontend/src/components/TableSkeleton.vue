<script setup lang="ts">
import Skeleton from 'primevue/skeleton'

// Dòng giả trong lúc trang đầu đang tải (đặt trong #empty của DataTable)
withDefaults(defineProps<{ rows?: number }>(), { rows: 3 })
</script>

<template>
  <div class="table-skeleton" aria-busy="true" aria-label="Loading">
    <Skeleton v-for="i in rows" :key="i" height="0.9rem" />
  </div>
</template>

<style scoped>
.table-skeleton {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  padding: 0.6rem 0;
}
</style>
