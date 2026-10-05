<script setup lang="ts" generic="T extends string">
import SelectButton from 'primevue/selectbutton'

// Nút chọn một trong vài giá trị (lọc trạng thái, đổi kiểu hiển thị): SelectButton của
// PrimeVue với số đếm và icon. iconOnly: chỉ hiện icon, nhãn thành tooltip.
export interface SegmentOption<V extends string> {
  value: V
  label: string
  count?: number
  icon?: string
}

// label: tên của nhóm cho trình đọc màn hình
defineProps<{ options: SegmentOption<T>[]; label: string; iconOnly?: boolean }>()
const model = defineModel<T>({ required: true })
</script>

<template>
  <SelectButton v-model="model" :options="options" option-value="value" :allow-empty="false" :aria-label="label">
    <template #option="{ option }">
      <i v-if="option.icon" v-tooltip.top="iconOnly ? option.label : undefined" :class="option.icon" :aria-label="iconOnly ? option.label : undefined" />
      <span v-if="!iconOnly">{{ option.label }}</span>
      <span v-if="option.count !== undefined" class="segment-count">{{ option.count }}</span>
    </template>
  </SelectButton>
</template>

<style scoped>
.segment-count {
  font: 0.75rem var(--app-mono);
  color: var(--p-text-muted-color);
}
</style>
