<script setup lang="ts">
import IconAction from './IconAction.vue'
import RowActions from './RowActions.vue'

// Nút ☰ cuối dòng: mở menu của dòng (useRowMenu().toggle). Ô luôn giữ chỗ một nút; nút chỉ
// dựng cho hàng đang dùng (active), để rê chuột không làm bảng xô lệch. Đặt trong cột
// <Column frozen align-frozen="right" class="row-menu-col">, để luôn ở mép phải của bảng
defineProps<{ active: boolean; label?: string }>()
const emit = defineEmits<{ open: [e: MouseEvent] }>()
</script>

<template>
  <RowActions :count="1" :active="active">
    <IconAction icon="pi pi-bars" :label="label ?? 'Row actions'" aria-haspopup="menu" @click="(e: MouseEvent) => emit('open', e)" />
  </RowActions>
</template>
