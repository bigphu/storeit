<script setup lang="ts">
import Button from 'primevue/button'
import type { RouteLocationRaw } from 'vue-router'

// Nút chỉ có icon cho hành động nhanh (dòng bảng, đầu thẻ): tooltip và aria-label là label.
// to: là liên kết thật (Ctrl/chuột giữa mở tab mới). disabled kèm reason: tooltip nói lý do
// (nút tắt không nhận rê chuột nên tooltip nằm ở span bọc ngoài).
const props = defineProps<{
  icon: string
  label: string
  danger?: boolean
  disabled?: boolean
  reason?: string
  to?: RouteLocationRaw
}>()
const emit = defineEmits<{ click: [e: MouseEvent] }>()
const severity = props.danger ? 'danger' : undefined
</script>

<template>
  <span v-if="disabled" v-tooltip.top="reason ?? label" class="icon-action-wrap">
    <Button :icon="icon" text rounded size="small" :severity="severity" :aria-label="label" disabled />
  </span>
  <Button v-else-if="to" v-slot="slot" v-tooltip.top="label" :icon="icon" text rounded size="small" :severity="severity" as-child>
    <RouterLink :to="to" :class="slot.class" :aria-label="label"><i :class="icon" aria-hidden="true" /></RouterLink>
  </Button>
  <Button
    v-else
    v-tooltip.top="label"
    :icon="icon"
    text
    rounded
    size="small"
    :severity="severity"
    :aria-label="label"
    @click="(e: MouseEvent) => emit('click', e)"
  />
</template>

<style scoped>
.icon-action-wrap {
  display: inline-flex;
}
</style>
