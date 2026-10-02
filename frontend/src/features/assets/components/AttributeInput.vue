<script setup lang="ts">
import DatePicker from 'primevue/datepicker'
import InputNumber from 'primevue/inputnumber'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import { computed } from 'vue'
import type { Attribute } from '@/lib/api/types'
import type { FormValue } from '../values'

// Một ô nhập cho giá trị thuộc tính, theo kiểu dữ liệu (xem values.ts cho dạng giá trị)
const props = defineProps<{ attribute: Attribute; error?: string }>()
const model = defineModel<FormValue>({ required: true })

const id = computed(() => `attr-${props.attribute.key}`)

// boolean có ba trạng thái: chưa đặt, có, không
const boolOptions = [
  { label: '—', value: null },
  { label: 'Yes', value: 'true' },
  { label: 'No', value: 'false' },
]

// Option đang hoạt động, cộng option đã gỡ nếu tài sản đang giữ nó
const selectOptions = computed(() =>
  props.attribute.options
    .filter((o) => !o.removed || o.id === model.value)
    .sort((a, b) => a.position - b.position)
    .map((o) => ({ label: o.removed ? `${o.label} (removed)` : o.label, value: o.id })),
)
</script>

<template>
  <div class="field">
    <label :for="id">
      {{ attribute.label }}<span v-if="attribute.is_required"> *</span>
    </label>
    <Textarea v-if="attribute.data_type === 'text'" :id="id" v-model="model as string" rows="1" auto-resize />
    <InputNumber
      v-else-if="attribute.data_type === 'number'"
      v-model="model as number | null"
      :input-id="id"
      :max-fraction-digits="6"
      :use-grouping="false"
      :suffix="attribute.unit ? ` ${attribute.unit}` : undefined"
    />
    <DatePicker
      v-else-if="attribute.data_type === 'date'"
      v-model="model as Date | null"
      :input-id="id"
      date-format="yy-mm-dd"
      show-icon
      show-button-bar
    />
    <Select
      v-else-if="attribute.data_type === 'boolean'"
      v-model="model"
      :input-id="id"
      :options="boolOptions"
      option-label="label"
      option-value="value"
    />
    <Select
      v-else
      v-model="model"
      :input-id="id"
      :options="selectOptions"
      option-label="label"
      option-value="value"
      show-clear
      placeholder="—"
    />
    <small v-if="error" class="field-error">{{ error }}</small>
  </div>
</template>
