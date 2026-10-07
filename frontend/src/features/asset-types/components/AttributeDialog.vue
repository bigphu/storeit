<script setup lang="ts">
import Checkbox from 'primevue/checkbox'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import { computed, ref, watch } from 'vue'
import FormDialog from '@/components/FormDialog.vue'
import type { Attribute, DataType } from '@/lib/api/types'
import { useDirty, useFormErrors } from '@/lib/forms'
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
// thuộc tính mới đứng cuối; đổi thứ tự bằng kéo thả trong bảng
const position = ref(0)
// option của thuộc tính select mới: mỗi dòng một nhãn
const optionsText = ref('')

const errors = useFormErrors()
const add = useAddAttribute()
const update = useUpdateAttribute()
const isNew = computed(() => props.attribute === null)
const form = useDirty(() => ({ k: key.value, l: label.value, t: dataType.value, u: unit.value, r: required.value, o: optionsText.value }))

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
  form.reset()
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
  <FormDialog
    v-model:visible="visible"
    icon="tag"
    :title="isNew ? 'Add attribute' : 'Edit attribute'"
    :action="isNew ? 'Add attribute' : 'Save attribute'"
    :busy="add.isPending.value || update.isPending.value"
    :error="errors.general.value"
    :dirty="form.dirty.value"
    @submit="submit"
  >
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
  </FormDialog>
</template>
