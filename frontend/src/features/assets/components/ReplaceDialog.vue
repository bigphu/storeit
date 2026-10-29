<script setup lang="ts">
import AutoComplete, { type AutoCompleteCompleteEvent } from 'primevue/autocomplete'
import DatePicker from 'primevue/datepicker'
import Textarea from 'primevue/textarea'
import { computed, ref, watch } from 'vue'
import FormDialog from '@/components/FormDialog.vue'
import { announce } from '@/lib/actions'
import type { AssetDetail, AssetListItem } from '@/lib/api/types'
import { fromDateString, toDateString } from '@/lib/dates'
import { useDirty, useFormErrors } from '@/lib/forms'
import { searchAssets, useRecordReplacement } from '../api'
import { replacementFormError, todayLocal } from '../replacement'
import { useAssetActions } from '../useAssetActions'

// Thay tài sản này bằng một tài sản khác (US-13): chọn tài sản thay, ngày, lý do. Tài sản này
// bị retire; thông báo thành công có Undo. Lỗi của form (vd ngày tương lai) hiện trong hộp
const props = defineProps<{ asset: AssetDetail }>()
const visible = defineModel<boolean>('visible', { required: true })

const errors = useFormErrors()
const record = useRecordReplacement()
const actions = useAssetActions()

const picked = ref<AssetListItem | null>(null)
const replacedOn = ref('')
const reason = ref('')
const today = ref(todayLocal(new Date()))
const suggestions = ref<AssetListItem[]>([])
const form = useDirty(() => ({ p: picked.value?.id ?? '', d: replacedOn.value, r: reason.value.trim() }))

watch(visible, (open) => {
  if (!open) return
  today.value = todayLocal(new Date())
  picked.value = null
  replacedOn.value = today.value
  reason.value = ''
  errors.clear()
  form.reset()
})

const problem = computed(() =>
  replacementFormError({ replacementId: picked.value?.id, replacedOn: replacedOn.value, reason: reason.value }, today.value),
)

// gợi ý: tìm theo tag hay tên, bỏ chính tài sản này (tài sản đã retire không có trong kết quả)
async function search(e: AutoCompleteCompleteEvent) {
  const items = await searchAssets(e.query)
  suggestions.value = items.filter((a) => a.id !== props.asset.id)
}
const optionLabel = (a: AssetListItem) => `${a.tag} · ${a.name}`

async function submit() {
  if (problem.value || !picked.value) return
  errors.clear()
  try {
    const saved = await record.mutateAsync({
      id: props.asset.id,
      replacementId: picked.value.id,
      replacedOn: replacedOn.value,
      reason: reason.value.trim(),
      version: props.asset.version,
    })
    announce(saved, actions.replacementUndo(props.asset, picked.value.tag))
    visible.value = false
  } catch (err) {
    errors.set(err)
  }
}
</script>

<template>
  <FormDialog
    v-model:visible="visible"
    icon="box"
    :title="`Replace ${asset.tag}`"
    action="Replace asset"
    :busy="record.isPending.value"
    :error="errors.general.value"
    :dirty="form.dirty.value"
    :disabled="!!problem"
    @submit="submit"
  >
    <p class="note">{{ asset.tag }} will be retired.</p>
    <div class="field">
      <label for="replacement-asset">Replacement asset</label>
      <AutoComplete
        v-model="picked"
        input-id="replacement-asset"
        :suggestions="suggestions"
        :option-label="optionLabel"
        placeholder="Search tag or name"
        force-selection
        fluid
        @complete="search"
      >
        <template #option="{ option }: { option: AssetListItem }">
          <span class="opt"><span class="mono">{{ option.tag }}</span> · {{ option.name }} <small>· {{ option.asset_type_name }}</small></span>
        </template>
      </AutoComplete>
      <small v-if="errors.fields.value.replacement_asset_id" class="field-error">{{ errors.fields.value.replacement_asset_id }}</small>
    </div>
    <div class="field">
      <label for="replaced-on">Date</label>
      <DatePicker
        :model-value="fromDateString(replacedOn)"
        input-id="replaced-on"
        date-format="yy-mm-dd"
        :max-date="fromDateString(today) ?? undefined"
        fluid
        @update:model-value="(d) => (replacedOn = toDateString(d as Date | null) ?? '')"
      />
      <small v-if="errors.fields.value.replaced_on" class="field-error">{{ errors.fields.value.replaced_on }}</small>
    </div>
    <div class="field">
      <label for="replace-reason">Reason</label>
      <Textarea id="replace-reason" v-model="reason" rows="3" maxlength="500" auto-resize fluid />
      <small v-if="errors.fields.value.reason" class="field-error">{{ errors.fields.value.reason }}</small>
    </div>
    <small v-if="problem && form.dirty.value" class="hint">{{ problem }}</small>
  </FormDialog>
</template>

<style scoped>
.note {
  margin: 0 0 0.75rem;
  color: var(--p-text-muted-color);
}
.opt small {
  color: var(--p-text-muted-color);
}
.hint {
  color: var(--p-text-muted-color);
}
</style>
