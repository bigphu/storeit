<script setup lang="ts">
import Button from 'primevue/button'
import DatePicker from 'primevue/datepicker'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import MultiSelect from 'primevue/multiselect'
import Popover from 'primevue/popover'
import Select from 'primevue/select'
import { computed, ref } from 'vue'
import type { Attribute } from '@/lib/api/types'
import { fromDateString, toDateString } from '@/lib/dates'
import { OP_LABEL } from '../filterChips'
import { type AttrFilterRow, BUILTIN_FIELDS, type FieldFilterRow, fieldOperators, operatorsFor } from '../listQuery'

// "+ Filter": thêm một điều kiện "trường / toán tử / giá trị" (các điều kiện kết hợp AND).
// Trường có sẵn (ngày mua, ngày tạo, ngày sửa, mô tả) luôn có; thuộc tính khi đang xem một loại.
// Trường có sẵn mang khoá "field:<key>" trong ô chọn để không trùng key thuộc tính.
const props = defineProps<{ attributes: Attribute[] }>()
const emit = defineEmits<{ add: [filter: AttrFilterRow]; addField: [filter: FieldFilterRow] }>()

const pop = ref<InstanceType<typeof Popover>>()
const row = ref<AttrFilterRow>({ key: '', op: 'eq', value: '' })
const attr = computed(() => props.attributes.find((a) => a.key === row.value.key))
const builtin = computed(() => BUILTIN_FIELDS.find((b) => `field:${b.key}` === row.value.key))
const dataType = computed(() => builtin.value?.data_type ?? attr.value?.data_type)

const groups = computed(() => [
  { label: 'Built-in', items: BUILTIN_FIELDS.map((b) => ({ label: b.label, value: `field:${b.key}` })) },
  ...(props.attributes.length ? [{ label: 'Attributes', items: props.attributes.map((a) => ({ label: a.label, value: a.key })) }] : []),
])

function opsFor(key: string): string[] {
  const b = BUILTIN_FIELDS.find((x) => `field:${x.key}` === key)
  if (b) return fieldOperators(b.key)
  const a = props.attributes.find((x) => x.key === key)
  return a ? operatorsFor(a.data_type) : ['eq']
}

function reset() {
  const key = props.attributes[0]?.key ?? `field:${BUILTIN_FIELDS[0].key}`
  row.value = { key, op: opsFor(key)[0], value: '' }
}

// đổi trường thì toán tử và giá trị cũ có thể không còn hợp
function changeKey(key: string) {
  row.value = { key, op: opsFor(key)[0], value: '' }
}
function changeOp(op: string) {
  if (row.value.op === 'in' || op === 'in') row.value.value = ''
  row.value.op = op
}

const operators = computed(() => opsFor(row.value.key).map((o) => ({ label: OP_LABEL[o], value: o })))
const options = computed(() => (attr.value?.options ?? []).filter((o) => !o.removed).map((o) => ({ label: o.label, value: o.id })))
const boolOptions = [
  { label: 'Yes', value: 'true' },
  { label: 'No', value: 'false' },
]

function add() {
  if (!row.value.key || row.value.value === '') return
  if (builtin.value) emit('addField', { key: builtin.value.key, op: row.value.op, value: row.value.value })
  else emit('add', { ...row.value })
  pop.value?.hide()
}

defineExpose({
  // target: phần tử neo (phím F mở từ nút Filter)
  toggle(e: Event, target?: HTMLElement) {
    reset()
    pop.value?.toggle(e, target)
  },
})
</script>

<template>
  <Popover ref="pop">
    <form class="filter-form" @submit.prevent="add">
      <p class="pop-lead">Show only assets that match.</p>
      <label for="filter-attr">Field</label>
      <Select
        :model-value="row.key"
        input-id="filter-attr"
        :options="groups"
        option-label="label"
        option-value="value"
        option-group-label="label"
        option-group-children="items"
        append-to="self"
        @update:model-value="changeKey"
      />
      <label for="filter-op">Condition</label>
      <Select
        :model-value="row.op"
        input-id="filter-op"
        :options="operators"
        option-label="label"
        option-value="value"
        append-to="self"
        @update:model-value="changeOp"
      />
      <label for="filter-value">Value</label>
      <template v-if="attr || builtin">
        <DatePicker
          v-if="dataType === 'date'"
          :model-value="fromDateString(row.value)"
          input-id="filter-value"
          date-format="yy-mm-dd"
          append-to="self"
          @update:model-value="(d) => (row.value = toDateString(d as Date | null) ?? '')"
        />
        <InputNumber
          v-else-if="attr?.data_type === 'number'"
          :model-value="row.value === '' ? null : Number(row.value)"
          input-id="filter-value"
          :max-fraction-digits="6"
          :use-grouping="false"
          :suffix="attr?.unit ? ` ${attr.unit}` : undefined"
          @update:model-value="(v: number | null) => (row.value = v === null ? '' : String(v))"
        />
        <Select
          v-else-if="attr?.data_type === 'boolean'"
          v-model="row.value"
          input-id="filter-value"
          :options="boolOptions"
          option-label="label"
          option-value="value"
          append-to="self"
        />
        <MultiSelect
          v-else-if="attr?.data_type === 'select' && row.op === 'in'"
          :model-value="row.value ? row.value.split(',') : []"
          input-id="filter-value"
          :options="options"
          option-label="label"
          option-value="value"
          append-to="self"
          @update:model-value="(v: string[]) => (row.value = v.join(','))"
        />
        <Select
          v-else-if="attr?.data_type === 'select'"
          v-model="row.value"
          input-id="filter-value"
          :options="options"
          option-label="label"
          option-value="value"
          append-to="self"
        />
        <InputText v-else id="filter-value" v-model="row.value" />
      </template>
      <div class="actions">
        <Button label="Cancel" size="small" text severity="secondary" @click="pop?.hide()" />
        <Button type="submit" label="Add filter" size="small" :disabled="row.value === ''" />
      </div>
    </form>
  </Popover>
</template>

<style scoped>
.pop-lead {
  margin: 0;
  font-size: 0.85rem;
  color: var(--p-text-muted-color);
}
.filter-form {
  display: grid;
  gap: 0.35rem;
  width: min(80vw, 18rem);
}
.filter-form label {
  font-size: 0.85rem;
  color: var(--p-text-muted-color);
  margin-top: 0.25rem;
}
.filter-form .actions {
  margin-top: 0.5rem;
}
</style>
