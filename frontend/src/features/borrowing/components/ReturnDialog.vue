<script setup lang="ts">
import DatePicker from 'primevue/datepicker'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import { computed, ref, watch } from 'vue'
import FormDialog from '@/components/FormDialog.vue'
import { fromDateString, toDateString } from '@/lib/dates'
import { isApiError } from '@/lib/errors'
import { useDirty, useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { useStatuses } from '@/features/statuses/api'
import { useBusinessDay, useReturnAsset } from '../api'
import { returnConditionOptions, returnFormError, shortDay } from '../borrowing'

// Ghi nhận trả (US-11, người quản lý): ngày trả (mặc định hôm nay, không trước ngày bắt đầu), tình
// trạng (tốt thì về Available, hỏng thì chọn status khác), ghi chú. Người mượn nhận thư biên nhận
const props = defineProps<{ asset: { id: string; tag: string; version: number; startedOn: string } | null }>()
const visible = defineModel<boolean>('visible', { required: true })

const ret = useReturnAsset()
const errors = useFormErrors()
const today = useBusinessDay(visible)
const { data: statuses } = useStatuses()
const conditions = computed(() => returnConditionOptions(statuses.value ?? []))
const returnedOn = ref('')
const statusId = ref<string | null>(null)
const note = ref('')
const form = useDirty(() => ({ d: returnedOn.value, s: statusId.value, n: note.value.trim() }))

watch(
  visible,
  (open) => {
    if (!open) return
    returnedOn.value = today.value
    statusId.value = null
    note.value = ''
    errors.clear()
    form.reset()
  },
  { immediate: true },
)
// ngày của server đến sau lúc mở (trước đó là ngày của máy): form chưa sửa thì lấy ngày server
watch(today, (t) => {
  if (!visible.value || form.dirty.value || returnedOn.value === t) return
  returnedOn.value = t
  form.reset()
})

const problem = computed(() => {
  if (!props.asset) return null
  const p = returnFormError(returnedOn.value, props.asset.startedOn, today.value)
  return p ?? (note.value.trim().length > 500 ? 'Keep the note under 500 characters.' : null)
})

async function submit() {
  const a = props.asset
  if (!a || problem.value) return
  errors.clear()
  try {
    await ret.mutateAsync({ id: a.id, returnedOn: returnedOn.value, statusId: statusId.value ?? undefined, note: note.value.trim(), version: a.version })
    notify.success(`${a.tag} returned.`)
    visible.value = false
  } catch (err) {
    if (isApiError(err, '/errors/no-active-loan')) errors.general.value = 'This loan was already closed.'
    else errors.set(err)
  }
}
</script>

<template>
  <FormDialog
    v-model:visible="visible"
    icon="box"
    :title="asset ? `Record return of ${asset.tag}` : 'Record return'"
    action="Record return"
    :busy="ret.isPending.value"
    :error="errors.general.value"
    :dirty="form.dirty.value"
    :disabled="!!problem"
    @submit="submit"
  >
    <p v-if="asset" class="note">On loan since {{ shortDay(asset.startedOn) }}. The borrower gets an email receipt.</p>
    <div class="field">
      <label for="return-on">Returned on</label>
      <DatePicker
        :model-value="fromDateString(returnedOn)"
        input-id="return-on"
        date-format="yy-mm-dd"
        :min-date="fromDateString(asset?.startedOn) ?? undefined"
        :max-date="fromDateString(today) ?? undefined"
        fluid
        @update:model-value="(d) => (returnedOn = toDateString(d as Date | null) ?? '')"
      />
      <small v-if="errors.fields.value.returned_on" class="field-error">{{ errors.fields.value.returned_on }}</small>
      <small v-else-if="problem" class="field-error">{{ problem }}</small>
    </div>
    <div class="field">
      <label for="return-condition">Condition</label>
      <Select v-model="statusId" input-id="return-condition" :options="conditions" option-label="label" option-value="value" fluid />
      <small v-if="errors.fields.value.status_id" class="field-error">{{ errors.fields.value.status_id }}</small>
    </div>
    <div class="field">
      <label for="return-note">Note <span class="muted">(optional)</span></label>
      <Textarea id="return-note" v-model="note" rows="2" maxlength="500" auto-resize fluid />
      <small v-if="errors.fields.value.note" class="field-error">{{ errors.fields.value.note }}</small>
    </div>
  </FormDialog>
</template>

<style scoped>
.note {
  margin: 0 0 0.75rem;
  color: var(--p-text-muted-color);
}
.muted {
  color: var(--p-text-muted-color);
}
</style>
