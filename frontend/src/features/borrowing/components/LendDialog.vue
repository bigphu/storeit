<script setup lang="ts">
import AutoComplete, { type AutoCompleteCompleteEvent } from 'primevue/autocomplete'
import DatePicker from 'primevue/datepicker'
import { computed, ref, watch } from 'vue'
import FormDialog from '@/components/FormDialog.vue'
import type { Person } from '@/lib/api/types'
import { fromDateString, toDateString } from '@/lib/dates'
import { useDirty, useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { todayLocal } from '@/features/assets/replacement'
import { searchBorrowers, useLendAsset } from '../api'

// Cho mượn trực tiếp (người quản lý), không cần phiếu: chọn người (account đang hoạt động), hạn
// trả tuỳ chọn; khoản mượn bắt đầu hôm nay, tài sản thành On loan
const props = defineProps<{ asset: { id: string; tag: string; version: number } | null }>()
const visible = defineModel<boolean>('visible', { required: true })

const lend = useLendAsset()
const errors = useFormErrors()
const person = ref<Person | null>(null)
const returnBy = ref('')
const today = ref(todayLocal(new Date()))
const suggestions = ref<Person[]>([])
const form = useDirty(() => ({ p: person.value?.id ?? '', r: returnBy.value }))

watch(
  visible,
  (open) => {
    if (!open) return
    today.value = todayLocal(new Date())
    person.value = null
    returnBy.value = ''
    errors.clear()
    form.reset()
  },
  { immediate: true },
)

// câu trả lời cũ đến sau câu mới thì bỏ
let seq = 0
async function search(e: AutoCompleteCompleteEvent) {
  const mine = ++seq
  let items: Person[] = []
  try {
    items = await searchBorrowers(e.query.trim())
  } catch {
    // lỗi mạng: không gợi ý, gõ lại để thử
  }
  if (mine === seq) suggestions.value = items
}

const problem = computed(() => (!person.value?.id ? 'Pick who borrows it.' : returnBy.value && returnBy.value < today.value ? "The return date can't be in the past." : null))

async function submit() {
  const a = props.asset
  if (!a || !person.value || problem.value) return
  errors.clear()
  try {
    await lend.mutateAsync({ id: a.id, borrowerId: person.value.id, returnBy: returnBy.value || undefined, version: a.version })
    notify.success(`${a.tag} lent to ${person.value.name}.`)
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
    :title="asset ? `Lend ${asset.tag}` : 'Lend'"
    :action="asset ? `Lend ${asset.tag}` : 'Lend'"
    :busy="lend.isPending.value"
    :error="errors.general.value"
    :dirty="form.dirty.value"
    :disabled="!!problem"
    @submit="submit"
  >
    <p class="note">The loan starts today, without a ticket.</p>
    <div class="field">
      <label for="lend-person">Borrower</label>
      <AutoComplete
        v-model="person"
        input-id="lend-person"
        :suggestions="suggestions"
        option-label="name"
        placeholder="Search name or email"
        dropdown
        force-selection
        fluid
        @complete="search"
      >
        <template #option="{ option }: { option: Person }">
          <span>{{ option.name }} <small class="muted">· {{ option.email }}</small></span>
        </template>
      </AutoComplete>
      <small v-if="errors.fields.value.borrower_id" class="field-error">{{ errors.fields.value.borrower_id }}</small>
    </div>
    <div class="field">
      <label for="lend-return">Return by <span class="muted">(optional)</span></label>
      <DatePicker
        :model-value="fromDateString(returnBy)"
        input-id="lend-return"
        date-format="yy-mm-dd"
        :min-date="fromDateString(today) ?? undefined"
        show-clear
        fluid
        @update:model-value="(d) => (returnBy = toDateString(d as Date | null) ?? '')"
      />
      <small v-if="errors.fields.value.return_by" class="field-error">{{ errors.fields.value.return_by }}</small>
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
