<script setup lang="ts">
import Button from 'primevue/button'
import DatePicker from 'primevue/datepicker'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import MultiSelect from 'primevue/multiselect'
import Select from 'primevue/select'
import { ref, watch } from 'vue'
import type { Attribute } from '@/lib/api/types'
import { fromDateString, toDateString } from '@/lib/dates'
import { type AttrFilterRow, operatorsFor } from '../listQuery'

// Các dòng lọc "thuộc tính / toán tử / giá trị" (AND). Sửa trên bản nháp, bấm Apply mới gửi.
const props = defineProps<{ attributes: Attribute[]; filters: AttrFilterRow[] }>()
const emit = defineEmits<{ apply: [filters: AttrFilterRow[]] }>()

const maxFilters = 10 // giới hạn của API
const rows = ref<AttrFilterRow[]>([])
watch(
  () => props.filters,
  (f) => (rows.value = f.map((r) => ({ ...r }))),
  { immediate: true },
)

const byKey = (key: string) => props.attributes.find((a) => a.key === key)

const opLabels: Record<string, string> = {
  eq: 'is',
  contains: 'contains',
  gt: '>',
  gte: '≥',
  lt: '<',
  lte: '≤',
  in: 'is any of',
}

function addRow() {
  const a = props.attributes[0]
  if (a) rows.value.push({ key: a.key, op: operatorsFor(a.data_type)[0], value: '' })
}

// đổi thuộc tính thì toán tử và giá trị cũ có thể không còn hợp
function changeKey(row: AttrFilterRow, key: string) {
  const a = byKey(key)
  row.key = key
  row.op = a ? operatorsFor(a.data_type)[0] : 'eq'
  row.value = ''
}

function changeOp(row: AttrFilterRow, op: string) {
  if (row.op === 'in' || op === 'in') row.value = ''
  row.op = op
}

const options = (a: Attribute) =>
  a.options.filter((o) => !o.removed).map((o) => ({ label: o.label, value: o.id }))

const boolOptions = [
  { label: 'Yes', value: 'true' },
  { label: 'No', value: 'false' },
]

function apply() {
  emit('apply', rows.value.filter((r) => r.value !== ''))
}
</script>

<template>
  <fieldset class="filters">
    <legend>Attribute filters</legend>
    <div v-for="(row, i) in rows" :key="i" class="actions">
      <Select
        :model-value="row.key"
        :options="attributes"
        option-label="label"
        option-value="key"
        aria-label="Attribute"
        @update:model-value="(k: string) => changeKey(row, k)"
      />
      <Select
        :model-value="row.op"
        :options="operatorsFor(byKey(row.key)?.data_type ?? 'text').map((o) => ({ label: opLabels[o], value: o }))"
        option-label="label"
        option-value="value"
        aria-label="Operator"
        @update:model-value="(o: string) => changeOp(row, o)"
      />
      <template v-if="byKey(row.key)">
        <InputNumber
          v-if="byKey(row.key)!.data_type === 'number'"
          :model-value="row.value === '' ? null : Number(row.value)"
          :max-fraction-digits="6"
          :use-grouping="false"
          :suffix="byKey(row.key)!.unit ? ` ${byKey(row.key)!.unit}` : undefined"
          aria-label="Value"
          @update:model-value="(v: number | null) => (row.value = v === null ? '' : String(v))"
        />
        <DatePicker
          v-else-if="byKey(row.key)!.data_type === 'date'"
          :model-value="fromDateString(row.value)"
          date-format="yy-mm-dd"
          aria-label="Value"
          @update:model-value="(d) => (row.value = toDateString(d as Date | null) ?? '')"
        />
        <Select
          v-else-if="byKey(row.key)!.data_type === 'boolean'"
          v-model="row.value"
          :options="boolOptions"
          option-label="label"
          option-value="value"
          aria-label="Value"
        />
        <MultiSelect
          v-else-if="byKey(row.key)!.data_type === 'select' && row.op === 'in'"
          :model-value="row.value ? row.value.split(',') : []"
          :options="options(byKey(row.key)!)"
          option-label="label"
          option-value="value"
          aria-label="Value"
          @update:model-value="(v: string[]) => (row.value = v.join(','))"
        />
        <Select
          v-else-if="byKey(row.key)!.data_type === 'select'"
          v-model="row.value"
          :options="options(byKey(row.key)!)"
          option-label="label"
          option-value="value"
          aria-label="Value"
        />
        <InputText v-else v-model="row.value" aria-label="Value" />
      </template>
      <Button icon="pi pi-times" text severity="secondary" aria-label="Remove filter" @click="rows.splice(i, 1)" />
    </div>
    <div class="actions">
      <Button
        label="Add filter"
        icon="pi pi-plus"
        size="small"
        text
        :disabled="rows.length >= maxFilters || attributes.length === 0"
        @click="addRow"
      />
      <Button label="Apply" size="small" @click="apply" />
    </div>
  </fieldset>
</template>

<style scoped>
.filters {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 1rem;
}
</style>
