<script setup lang="ts">
import Button from 'primevue/button'
import Chip from 'primevue/chip'
import DatePicker from 'primevue/datepicker'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import { useQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import FormDialog from '@/components/FormDialog.vue'
import { inventoryApi } from '@/lib/api/client'
import type { BorrowTicket } from '@/lib/api/types'
import { fromDateString, toDateString } from '@/lib/dates'
import { unwrap } from '@/lib/errors'
import { useDirty, useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { assetKeys } from '@/features/assets/api'
import AttributeFilterPopover from '@/features/assets/components/AttributeFilterPopover.vue'
import { attrFilterLabel } from '@/features/assets/filterChips'
import type { AttrFilterRow } from '@/features/assets/listQuery'
import { useAssetType, useAssetTypes } from '@/features/asset-types/api'
import { useBusinessDay, useCreateTicket } from '../api'
import { requestCode, ticketFormError } from '../borrowing'
import { keyId } from '@/lib/queryKeyId'

// Phiếu mượn mới: loại tài sản cho mượn được, yêu cầu thuộc tính chọn bằng bộ lọc của danh
// sách (đếm ngay số tài sản khớp và đang rảnh), ngày cần, hạn trả (tuỳ chọn), ghi chú.
// presetTypeId / suggested: mở từ "Borrow this…" trên một tài sản
const props = defineProps<{ presetTypeId?: string; suggested?: { id: string; tag: string } }>()
const visible = defineModel<boolean>('visible', { required: true })
const emit = defineEmits<{ created: [ticket: BorrowTicket] }>()

const { data: allTypes } = useAssetTypes()
const typeOptions = computed(() => (allTypes.value ?? []).filter((t) => t.can_borrow))

const typeId = ref<string>()
const reqs = ref<AttrFilterRow[]>([])
const neededFrom = ref('')
const returnBy = ref('')
const note = ref('')
const today = useBusinessDay(visible)
const errors = useFormErrors()
const create = useCreateTicket()
const form = useDirty(() => ({ t: typeId.value, r: reqs.value, n: neededFrom.value, b: returnBy.value, note: note.value.trim() }))

watch(
  visible,
  (open) => {
    if (!open) return
    typeId.value = props.presetTypeId
    reqs.value = []
    neededFrom.value = today.value
    returnBy.value = ''
    note.value = ''
    errors.clear()
    form.reset()
  },
  { immediate: true },
)
// ngày của server đến sau lúc mở và muộn hơn ngày của máy: dời "cần từ" lên, form vẫn coi là chưa sửa
watch(today, (t) => {
  if (!visible.value || !neededFrom.value || neededFrom.value >= t) return
  const clean = !form.dirty.value
  neededFrom.value = t
  if (clean) form.reset()
})

const { data: type } = useAssetType(typeId)
const attrs = computed(() => (type.value?.attributes ?? []).filter((a) => !a.removed))
// đổi loại thì yêu cầu cũ không còn nghĩa
watch(typeId, (id, old) => {
  if (old && id !== old) reqs.value = []
})

const reqStrings = computed(() => reqs.value.map((f) => `${f.key}:${f.op}:${f.value}`))
const matches = useQuery({
  queryKey: computed(() => [...assetKeys.all, 'borrowable-count', typeId.value ?? '', reqStrings.value] as const),
  // loại và yêu cầu lấy từ khoá: hộp vừa đóng thì typeId đã rỗng (lib/queryKeyId.ts)
  queryFn: ({ queryKey }) =>
    unwrap(
      inventoryApi.GET('/assets', {
        params: { query: { type_id: keyId(queryKey, 2), attr: queryKey[3].length ? [...queryKey[3]] : undefined, borrowable: true, page: 1, page_size: 1 } },
      }),
    ),
  enabled: computed(() => visible.value && !!typeId.value),
})
const matchText = computed(() => {
  const n = matches.data.value?.total
  if (n === undefined || !type.value) return ''
  return `${n} ${type.value.name} ${n === 1 ? 'matches and is' : 'match and are'} free now`
})

const pop = ref<InstanceType<typeof AttributeFilterPopover>>()
function addReq(f: AttrFilterRow) {
  reqs.value = [...reqs.value, f]
}
function removeReq(i: number) {
  reqs.value = reqs.value.filter((_, j) => j !== i)
}

const problem = computed(() =>
  ticketFormError({ typeId: typeId.value, neededFrom: neededFrom.value, returnBy: returnBy.value || undefined, note: note.value }, today.value),
)
// lỗi của một yêu cầu từ API: "requirements[i]"
const reqErrors = computed(() =>
  Object.entries(errors.fields.value)
    .filter(([k]) => k.startsWith('requirements'))
    .map(([, v]) => v),
)

async function submit() {
  if (problem.value || !typeId.value) return
  errors.clear()
  try {
    const t = await create.mutateAsync({
      asset_type_id: typeId.value,
      requirements: reqStrings.value,
      suggested_asset_id: props.suggested?.id,
      needed_from: neededFrom.value,
      return_by: returnBy.value || undefined,
      note: note.value.trim() || undefined,
    })
    notify.success(`${requestCode(t.number)} sent.`)
    visible.value = false
    emit('created', t)
  } catch (err) {
    errors.set(err)
  }
}
</script>

<template>
  <FormDialog
    v-model:visible="visible"
    icon="box"
    title="Ask to borrow"
    action="Send ticket"
    :busy="create.isPending.value"
    :error="errors.general.value"
    :dirty="form.dirty.value"
    :disabled="!!problem"
    @submit="submit"
  >
    <p v-if="suggested" class="note">You asked for {{ suggested.tag }}; the inventory team may pick another that fits.</p>
    <div class="field">
      <label for="ticket-type">What do you need?</label>
      <Select
        v-model="typeId"
        input-id="ticket-type"
        :options="typeOptions"
        option-label="name"
        option-value="id"
        placeholder="Pick a kind of asset"
        :disabled="!!suggested"
        fluid
      />
      <small v-if="errors.fields.value.asset_type_id" class="field-error">{{ errors.fields.value.asset_type_id }}</small>
      <small v-else-if="!typeOptions.length" class="hint">Nothing can be borrowed yet. Ask the inventory team.</small>
    </div>
    <div v-if="typeId" class="field">
      <span class="label">Specs <span class="muted">(optional)</span></span>
      <div class="chips">
        <Chip v-for="(f, i) in reqs" :key="i" :label="attrFilterLabel(f, attrs)" removable @remove="removeReq(i)" />
        <Button
          v-if="attrs.length && reqs.length < 10"
          label="Add spec"
          icon="pi pi-plus"
          size="small"
          text
          @click="(e: Event) => pop?.toggle(e)"
        />
        <span v-if="!attrs.length" class="muted">This type has no attributes to ask for.</span>
      </div>
      <small v-for="(m, i) in reqErrors" :key="i" class="field-error">{{ m }}</small>
      <small v-if="matchText" class="hint">{{ matchText }}</small>
      <AttributeFilterPopover ref="pop" :attributes="attrs" :builtins="false" lead="Ask for assets that match." action="Add spec" @add="addReq" />
    </div>
    <div class="dates">
      <div class="field">
        <label for="ticket-from">Needed from</label>
        <DatePicker
          :model-value="fromDateString(neededFrom)"
          input-id="ticket-from"
          date-format="yy-mm-dd"
          :min-date="fromDateString(today) ?? undefined"
          fluid
          @update:model-value="(d) => (neededFrom = toDateString(d as Date | null) ?? '')"
        />
        <small v-if="errors.fields.value.needed_from" class="field-error">{{ errors.fields.value.needed_from }}</small>
      </div>
      <div class="field">
        <label for="ticket-return">Return by <span class="muted">(optional)</span></label>
        <DatePicker
          :model-value="fromDateString(returnBy)"
          input-id="ticket-return"
          date-format="yy-mm-dd"
          :min-date="fromDateString(neededFrom || today) ?? undefined"
          show-clear
          fluid
          @update:model-value="(d) => (returnBy = toDateString(d as Date | null) ?? '')"
        />
        <small v-if="errors.fields.value.return_by" class="field-error">{{ errors.fields.value.return_by }}</small>
      </div>
    </div>
    <div class="field">
      <label for="ticket-note">Note <span class="muted">(optional)</span></label>
      <Textarea id="ticket-note" v-model="note" rows="2" maxlength="500" auto-resize fluid placeholder="What it's for, anything the specs can't say" />
      <small v-if="errors.fields.value.note" class="field-error">{{ errors.fields.value.note }}</small>
    </div>
    <small v-if="problem && form.dirty.value" class="hint">{{ problem }}</small>
  </FormDialog>
</template>

<style scoped>
.note {
  margin: 0 0 0.75rem;
  color: var(--p-text-muted-color);
}
.label {
  display: block;
  margin-bottom: 0.35rem;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  align-items: center;
}
.dates {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.75rem;
}
.muted,
.hint {
  color: var(--p-text-muted-color);
}
</style>
