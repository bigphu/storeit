<script setup lang="ts">
import { computed } from 'vue'
import { rowActionsWidth } from '@/lib/tableRows'

// Ô nút cuối dòng của bảng danh sách. count: số nút nhiều nhất cột này có thể hiện; ô luôn giữ
// đủ bề rộng và chiều cao cho chừng đó nút, nên rê chuột (nút mới được dựng) không làm cột
// giãn hay hàng cao lên. active: chỉ dựng nút cho hàng đang dùng (useListTable().active
// .isActive(index)); bỏ trống thì luôn dựng. Nút bên trong dùng IconAction
const props = withDefaults(defineProps<{ count: number; active?: boolean }>(), { active: true })
const width = computed(() => rowActionsWidth(props.count))
</script>

<template>
  <div class="row-actions" :style="{ minWidth: width }">
    <slot v-if="active" />
  </div>
</template>
