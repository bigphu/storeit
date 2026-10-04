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
import { type AttrFilterRow, operatorsFor } from '../listQuery'

// "+ Filter": thêm một điều kiện "thuộc tính / toán tử / giá trị" (các điều kiện kết hợp AND)
const props = defineProps<{ attributes: Attribute[] }>()
const emit = defineEmits<{ add: [filter: AttrFilterRow] }>()

const pop = ref<InstanceType<typeof Popover>>()
const row = ref<AttrFilterRow>({ key: '', op: 'eq', value: '' })
const attr = computed(() => props.attributes.find((a) => a.key === row.value.key))

function reset() {
  const a = props.attributes[0]
  row.value = { key: a?.key ?? '', op: a ? operatorsFor(a.data_type)[0] : 'eq', value: '' }
}

// đổi thuộc tính thì toán tử và giá trị cũ có thể không còn hợp
function changeKey(key: string) {
  const a = props.attributes.find((x) => x.key === key)
  row.value = { key, op: a ? operatorsFor(a.data_type)[0] : 'eq', value: '' }
}
function changeOp(op: string) {
  if (row.value.op === 'in' || op === 'in') row.value.value = ''
  row.value.op = op
}

const operators = computed(() => operatorsFor(attr.value?.data_type ?? 'text').map((o) => ({ label: OP_LABEL[o], value: o })))
const options = computed(() => (attr.value?.options ?? []).filter((o) => !o.removed).map((o) => ({ label: o.label, value: o.id })))
const boolOptions = [
  { label: 'Yes', value: 'true' },
  { label: 'No', value: 'false' },
]

function add() {
  if (!row.value.key || row.value.value === '') return
  emit('add', { ...row.value })
  pop.value?.hide()
}

defineExpose({
  toggle(e: Event) {
    reset()
    pop.value?.toggle(e)
  },
})
</script>

<template>
  <Popover ref="pop">
    <form class="filter-form" @submit.prevent="add">
      <label for="filter-attr">Attribute</label>
      <Select
        :model-value="row.key"
        input-id="filter-attr"
        :options="attributes"
        option-label="label"
        option-value="key"
        @update:model-value="changeKey"
      />
      <label for="filter-op">Condition</label>
      <Select
        :model-value="row.op"
        input-id="filter-op"
        :options="operators"
        option-label="label"
        option-value="value"
        @update:model-value="changeOp"
      />
      <label for="filter-value">Value</label>
      <template v-if="attr">
        <InputNumber
          v-if="attr.data_type === 'number'"
          :model-value="row.value === '' ? null : Number(row.value)"
          input-id="filter-value"
          :max-fraction-digits="6"
          :use-grouping="false"
          :suffix="attr.unit ? ` ${attr.unit}` : undefined"
          @update:model-value="(v: number | null) => (row.value = v === null ? '' : String(v))"
        />
        <DatePicker
          v-else-if="attr.data_type === 'date'"
          :model-value="fromDateString(row.value)"
          input-id="filter-value"
          date-format="yy-mm-dd"
          @update:model-value="(d) => (row.value = toDateString(d as Date | null) ?? '')"
        />
        <Select
          v-else-if="attr.data_type === 'boolean'"
          v-model="row.value"
          input-id="filter-value"
          :options="boolOptions"
          option-label="label"
          option-value="value"
        />
        <MultiSelect
          v-else-if="attr.data_type === 'select' && row.op === 'in'"
          :model-value="row.value ? row.value.split(',') : []"
          input-id="filter-value"
          :options="options"
          option-label="label"
          option-value="value"
          @update:model-value="(v: string[]) => (row.value = v.join(','))"
        />
        <Select
          v-else-if="attr.data_type === 'select'"
          v-model="row.value"
          input-id="filter-value"
          :options="options"
          option-label="label"
          option-value="value"
        />
        <InputText v-else id="filter-value" v-model="row.value" />
      </template>
      <div class="actions">
        <Button type="submit" label="Add filter" size="small" :disabled="row.value === ''" />
        <Button label="Cancel" size="small" text severity="secondary" @click="pop?.hide()" />
      </div>
    </form>
  </Popover>
</template>

<style scoped>
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
