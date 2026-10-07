<script setup lang="ts">
import DatePicker from 'primevue/datepicker'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import { nextTick, onUnmounted, ref } from 'vue'
import { clickOrDouble } from '@/lib/clickOrDouble'
import { fromDateString, toDateString } from '@/lib/dates'

// Ô sửa tại chỗ trong danh sách: nhấp đúp (hay F2 khi ô đang focus) thành ô nhập; Enter hay
// bấm ra ngoài lưu, Esc huỷ. Nhấp một lần vẫn mở trang chi tiết (đợi 220 ms để phân biệt).
// Ô không sửa được: nhấp một lần mở ngay
const props = withDefaults(
  defineProps<{ value: string; editable: boolean; kind?: 'text' | 'select' | 'date'; options?: { label: string; value: string }[] }>(),
  { kind: 'text' },
)
const emit = defineEmits<{ save: [value: string]; open: [e: MouseEvent] }>()

const root = ref<HTMLElement>()
const editing = ref(false)
const text = ref('')
const timer = clickOrDouble()
onUnmounted(() => timer.cancel())

function onClick(e: MouseEvent) {
  if (editing.value) return
  if (!props.editable) {
    emit('open', e)
    return
  }
  timer.click(e, () => emit('open', e))
}
let done = false
async function start() {
  if (!props.editable || editing.value) return
  timer.double()
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
  <span
    ref="root"
    class="inline-cell"
    :class="{ editable, editing }"
    :tabindex="editable ? 0 : undefined"
    :title="editable && !editing ? 'Double-click to edit' : undefined"
    @click.stop="onClick"
    @dblclick.stop="start"
    @keydown.f2.prevent="start"
  >
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
    <slot v-else />
  </span>
</template>

<style scoped>
.inline-cell {
  display: inline-flex;
  max-width: 100%;
  border-radius: 6px;
}
.inline-cell.editing {
  display: flex;
  width: 100%;
}
.inline-cell.editable:not(.editing) {
  cursor: text;
}
.inline-cell.editable:not(.editing):hover {
  box-shadow: 0 0 0 1px var(--app-line);
}
.inline-cell:focus-visible {
  outline: 2px solid var(--app-accent);
  outline-offset: 1px;
}
</style>
