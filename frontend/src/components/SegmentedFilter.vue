<script setup lang="ts" generic="T extends string">
import SelectButton from 'primevue/selectbutton'

// Điều khiển phân đoạn duy nhất của app (kiểu iOS): lọc trạng thái, đổi kiểu hiển thị và tab chia
// mục của trang chi tiết đều dùng nó. SelectButton của PrimeVue với số đếm, icon, chấm "chưa lưu".
// iconOnly: chỉ hiện icon, nhãn thành tooltip.
export interface SegmentOption<V extends string> {
  value: V
  label: string
  count?: number
  icon?: string
  // mục còn thay đổi chưa lưu (tab của trang chi tiết)
  dirty?: boolean
  disabled?: boolean
}

// label: tên của nhóm cho trình đọc màn hình
defineProps<{ options: SegmentOption<T>[]; label: string; iconOnly?: boolean }>()
const model = defineModel<T>({ required: true })
</script>

<template>
  <SelectButton v-model="model" :options="options" option-value="value" option-disabled="disabled" :allow-empty="false" :aria-label="label">
    <template #option="{ option }">
      <i v-if="option.icon" v-tooltip.top="iconOnly ? option.label : undefined" :class="option.icon" :aria-label="iconOnly ? option.label : undefined" />
      <!-- data-text: giữ sẵn bề rộng của chữ đậm để lựa chọn không đổi cỡ khi bật / tắt -->
      <span v-if="!iconOnly" class="segment-label" :data-text="option.label">{{ option.label }}</span>
      <span v-if="option.count !== undefined" class="segment-count">{{ option.count }}</span>
      <span v-if="option.dirty" class="segment-dirty" aria-label="Unsaved changes" />
    </template>
  </SelectButton>
</template>

<style scoped>
.segment-label {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
}
.segment-label::after {
  content: attr(data-text);
  height: 0;
  overflow: hidden;
  visibility: hidden;
  font-weight: 600;
  user-select: none;
  pointer-events: none;
}
.segment-count {
  font: 0.75rem var(--app-mono);
  color: var(--p-text-muted-color);
}
.segment-dirty {
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 50%;
  background: var(--app-warn);
  flex: none;
}
</style>
