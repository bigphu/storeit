<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import { computed, ref, watch } from 'vue'
import type { Attribute, DataType } from '@/lib/api/types'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { useAddAttribute, useUpdateAttribute } from '../api'

// attribute null là thêm mới
const props = defineProps<{ typeId: string; attribute: Attribute | null; nextPosition: number }>()
const visible = defineModel<boolean>('visible', { required: true })

const dataTypes: DataType[] = ['text', 'number', 'date', 'boolean', 'select']

const key = ref('')
const label = ref('')
const dataType = ref<DataType>('text')
const unit = ref('')
const required = ref(false)
const position = ref(0)
// option của thuộc tính select mới: mỗi dòng một nhãn
const optionsText = ref('')

const errors = useFormErrors()
const add = useAddAttribute()
const update = useUpdateAttribute()
const isNew = computed(() => props.attribute === null)

watch(visible, (open) => {
  if (!open) return
  const a = props.attribute
  key.value = a?.key ?? ''
  label.value = a?.label ?? ''
  dataType.value = a?.data_type ?? 'text'
  unit.value = a?.unit ?? ''
  required.value = a?.is_required ?? false
  position.value = a?.position ?? props.nextPosition
  optionsText.value = ''
  errors.clear()
})

async function submit() {
  errors.clear()
  try {
    const a = props.attribute
    if (!a) {
      const options = optionsText.value
        .split('\n')
        .map((s) => s.trim())
        .filter(Boolean)
      await add.mutateAsync({
        typeId: props.typeId,
        key: key.value,
        label: label.value,
        data_type: dataType.value,
        unit: dataType.value === 'number' && unit.value ? unit.value : undefined,
        is_required: required.value,
        position: position.value,
        options: dataType.value === 'select' ? options : undefined,
      })
    } else {
      // chỉ gửi kiểu và đơn vị khi đổi: đổi khi đã có giá trị là lỗi 409 attribute-in-use
      await update.mutateAsync({
        typeId: props.typeId,
        attrId: a.id,
        label: label.value,
        is_required: required.value,
        position: position.value,
        data_type: dataType.value !== a.data_type ? dataType.value : undefined,
        unit: unit.value !== (a.unit ?? '') ? unit.value : undefined,
      })
    }
    notify.success('Attribute saved.')
    visible.value = false
  } catch (err) {
    errors.set(err)
  }
}
</script>

<template>
  <Dialog v-model:visible="visible" modal :header="isNew ? 'Add attribute' : 'Edit attribute'" :style="{ width: '32rem' }">
    <form class="form" @submit.prevent="submit">
      <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
      <div class="field">
        <label for="attr-key">Key</label>
        <InputText id="attr-key" v-model="key" :disabled="!isNew" required placeholder="ram_gb" />
        <small v-if="isNew">a-z, 0-9 and _, starting with a letter. Used in the API; can't be changed.</small>
        <small v-if="errors.fields.value.key" class="field-error">{{ errors.fields.value.key }}</small>
      </div>
      <div class="field">
        <label for="attr-label">Label</label>
        <InputText id="attr-label" v-model="label" required />
        <small v-if="errors.fields.value.label" class="field-error">{{ errors.fields.value.label }}</small>
      </div>
      <div class="field">
        <label for="attr-type">Data type</label>
        <Select v-model="dataType" input-id="attr-type" :options="dataTypes" />
        <small v-if="!isNew">Can't change while assets have values for this attribute.</small>
        <small v-if="errors.fields.value.data_type" class="field-error">{{ errors.fields.value.data_type }}</small>
      </div>
      <div v-if="dataType === 'number'" class="field">
        <label for="attr-unit">Unit</label>
        <InputText id="attr-unit" v-model="unit" placeholder="GB, inch, kg…" />
        <small v-if="errors.fields.value.unit" class="field-error">{{ errors.fields.value.unit }}</small>
      </div>
      <div v-if="isNew && dataType === 'select'" class="field">
        <label for="attr-options">Options (one per line)</label>
        <Textarea id="attr-options" v-model="optionsText" rows="4" />
      </div>
      <div class="actions">
        <Checkbox v-model="required" input-id="attr-required" binary />
        <label for="attr-required">Required</label>
      </div>
      <div class="field">
        <label for="attr-position">Position</label>
        <InputNumber v-model="position" input-id="attr-position" :use-grouping="false" />
      </div>
      <div class="actions">
        <Button type="submit" label="Save" :loading="add.isPending.value || update.isPending.value" />
        <Button label="Cancel" severity="secondary" text @click="visible = false" />
      </div>
    </form>
  </Dialog>
</template>
