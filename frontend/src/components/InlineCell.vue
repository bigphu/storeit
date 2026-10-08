<script setup lang="ts">
import DatePicker from 'primevue/datepicker'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import { nextTick, ref } from 'vue'
import { fromDateString, toDateString } from '@/lib/dates'

// Ô sửa tại chỗ trong danh sách: rê chuột lên dòng (hay thẻ) thì hiện bút chì nhỏ cạnh giá
// trị; bấm bút chì (hay F2 khi ô đang focus) thành ô nhập. Enter hay bấm ra ngoài lưu, Esc
// huỷ. Bấm vào chính giá trị vẫn mở trang chi tiết như bấm dòng (sự kiện đi tiếp lên dòng)
const props = withDefaults(
  defineProps<{ value: string; editable: boolean; kind?: 'text' | 'select' | 'date'; options?: { label: string; value: string }[]; label?: string }>(),
  { kind: 'text' },
)
const emit = defineEmits<{ save: [value: string] }>()

const root = ref<HTMLElement>()
const editing = ref(false)
const text = ref('')
let done = false

async function start() {
  if (!props.editable || editing.value) return
  text.value = props.value
  done = false
  editing.value = true
  await nextTick()
  const input = root.value?.querySelector<HTMLInputElement>('input')
  input?.focus()
  input?.select?.()
}
function finish(keep: boolean) {
  if (done) return
  done = true
  editing.value = false
  const v = text.value.trim()
  if (keep && v && v !== props.value) emit('save', v)
}
</script>

<template>
  <span ref="root" class="inline-cell" :class="{ editing }" @keydown.f2.prevent="start">
    <template v-if="editing">
      <Select
        v-if="kind === 'select'"
        v-model="text"
        :options="options"
        option-label="label"
        option-value="value"
        size="small"
        @change="finish(true)"
        @hide="finish(false)"
        @keydown.esc.stop="finish(false)"
        @click.stop
      />
      <DatePicker
        v-else-if="kind === 'date'"
        :model-value="fromDateString(text) ?? undefined"
        size="small"
        date-format="dd/mm/yy"
        @update:model-value="(d) => { text = toDateString(d as Date | null) ?? ''; finish(true) }"
        @hide="finish(false)"
        @keydown.esc.stop="finish(false)"
        @click.stop
      />
      <InputText
        v-else
        v-model="text"
        size="small"
        fluid
        @keydown.enter.prevent="finish(true)"
        @keydown.esc.stop="finish(false)"
        @blur="finish(true)"
        @click.stop
      />
    </template>
    <template v-else>
      <span class="ic-value"><slot /></span>
      <button
        v-if="editable"
        type="button"
        class="ic-edit"
        :aria-label="label ? `Edit ${label}` : 'Edit'"
        :title="label ? `Edit ${label}` : 'Edit'"
        @click.stop="start"
      >
        <i class="pi pi-pencil" aria-hidden="true" />
      </button>
    </template>
  </span>
</template>

<style scoped>
/* chiếm hết bề ngang của ô: bút chì luôn ở cuối, vị trí đoán trước được */
.inline-cell {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  width: 100%;
  min-width: 0;
}
.inline-cell.editing {
  display: flex;
  width: 100%;
}
/* một dòng: bảng tự giãn cột theo chữ; ô bị bó (bảng table-layout: fixed) thì cắt bằng "…" */
.ic-value {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
/* bút chì: ẩn cho đến khi rê chuột lên dòng / thẻ chứa nó, hay focus vào trong */
.ic-edit {
  flex: none;
  margin-left: auto;
  display: inline-grid;
  place-items: center;
  width: 1.5rem;
  height: 1.5rem;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--p-text-muted-color);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.12s ease;
}
.ic-edit i {
  font-size: 0.75rem;
}
.ic-edit:hover {
  background: var(--app-hover);
  color: var(--p-text-color);
}
.ic-edit:focus-visible {
  opacity: 1;
  outline: 2px solid var(--app-accent);
  outline-offset: 1px;
}
:global(tr:hover) .ic-edit,
:global(tr:focus-within) .ic-edit,
:global(.entity-card:hover) .ic-edit,
:global(.entity-card:focus-within) .ic-edit,
.inline-cell:hover .ic-edit {
  opacity: 1;
}
/* màn cảm ứng không rê chuột được: luôn hiện */
@media (hover: none) {
  .ic-edit {
    opacity: 1;
  }
}
</style>
