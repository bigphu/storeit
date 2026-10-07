<script setup lang="ts">
import Message from 'primevue/message'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import { computed, ref, watch } from 'vue'
import FormDialog from '@/components/FormDialog.vue'
import { announce } from '@/lib/actions'
import { useDirty, useFormErrors } from '@/lib/forms'
import { useStatuses } from '@/features/statuses/api'
import { type BulkItemRef, useBulkRetire, useBulkStatus, useRestoreAsset } from '../api'
import { type BulkSummary, retiredItems, statusGroups, summarizeBulk } from '../bulk'

// Đổi status hay retire các tài sản đã chọn, một Undo cho cả lô. Sau khi chạy: tài sản
// không làm được được liệt kê kèm lý do (người khác vừa sửa, đã retire...)
const props = defineProps<{ mode: 'status' | 'retire'; rows: { id: string; tag: string; version: number; status_id: string }[] }>()
const visible = defineModel<boolean>('visible', { required: true })
const emit = defineEmits<{ done: [] }>()

const retire = useBulkRetire()
const setStatus = useBulkStatus()
const restore = useRestoreAsset()
const busy = computed(() => retire.isPending.value || setStatus.isPending.value)
const errors = useFormErrors()

// status chọn được: không thuộc kind retired (dùng Retire), không lưu trữ
const { data: statuses } = useStatuses()
const statusOptions = computed(() => (statuses.value ?? []).filter((s) => s.kind !== 'retired'))

const statusId = ref<string | null>(null)
const reason = ref('')
const summary = ref<BulkSummary | null>(null)
const form = useDirty(() => ({ s: statusId.value, r: reason.value.trim() }))

watch(visible, (open) => {
  if (open) {
    statusId.value = null
    reason.value = ''
    summary.value = null
    errors.clear()
    form.reset()
  }
})

const count = computed(() => `${props.rows.length} ${props.rows.length === 1 ? 'asset' : 'assets'}`)
const header = computed(() =>
  summary.value ? 'Some assets were not changed' : props.mode === 'retire' ? `Retire ${count.value}` : `Change status of ${count.value}`,
)

async function restoreEach(items: BulkItemRef[]) {
  const results = await Promise.allSettled(items.map((it) => restore.mutateAsync(it)))
  const failed = results.filter((x) => x.status === 'rejected').length
  if (failed) throw new Error(`${failed} of ${items.length} could not be restored.`)
}
async function setEach(groups: { statusId: string; items: BulkItemRef[] }[]) {
  let failed = 0
  for (const g of groups) failed += (await setStatus.mutateAsync({ items: g.items, statusId: g.statusId })).failed.length
  if (failed) throw new Error(`${failed} could not be changed back.`)
}
const assets = (n: number) => `${n} ${n === 1 ? 'asset' : 'assets'}`

async function run() {
  const rows = [...props.rows]
  const items = rows.map((r) => ({ id: r.id, version: r.version }))
  const retiring = props.mode === 'retire'
  const target = statusId.value!
  errors.clear()
  try {
    const result = retiring
      ? await retire.mutateAsync({ items, reason: reason.value })
      : await setStatus.mutateAsync({ items, statusId: target })
    const s = summarizeBulk(result, rows, retiring ? 'retired' : 'updated')
    emit('done')
    const n = result.succeeded.length
    if (n)
      announce(
        result,
        retiring
          ? {
              done: s.message,
              undo: (r) => restoreEach(retiredItems(r, rows)),
              undone: `${assets(n)} restored.`,
              undoFailed: "Couldn't restore every asset. Some are still retired.",
            }
          : {
              done: s.message,
              undo: (r) => setEach(statusGroups(r, rows, target)),
              undone: `${assets(n)} changed back.`,
              undoFailed: "Couldn't change every asset back.",
            },
      )
    if (s.failures.length) summary.value = s
    else visible.value = false
  } catch (err) {
    errors.set(err)
  }
}
</script>

<template>
  <FormDialog
    v-model:visible="visible"
    icon="sliders"
    :title="header"
    :action="summary ? undefined : mode === 'retire' ? `Retire ${count}` : `Update ${count}`"
    :disabled="mode === 'status' && !statusId"
    :busy="busy"
    :error="errors.general.value"
    :dirty="!summary && form.dirty.value"
    @submit="run"
  >
    <template v-if="summary">
      <Message severity="warn">{{ summary.message }} Reload the list and try those again.</Message>
      <ul class="failures">
        <li v-for="f in summary.failures" :key="f.tag">
          <b>{{ f.tag }}</b>: {{ f.reason }}
        </li>
      </ul>
    </template>

    <template v-else-if="mode === 'retire'">
      <p>Retired assets leave the list unless “Include retired” is on, and can't be edited until restored.</p>
      <div class="field">
        <label for="bulk-reason">Reason (optional, the same for all)</label>
        <Textarea id="bulk-reason" v-model="reason" rows="3" maxlength="500" />
      </div>
    </template>

    <div v-else class="field">
      <label for="bulk-status">New status</label>
      <Select
        v-model="statusId"
        input-id="bulk-status"
        :options="statusOptions"
        option-label="name"
        option-value="id"
        placeholder="Choose a status"
      />
      <small>To retire assets, use Retire instead.</small>
    </div>
  </FormDialog>
</template>

<style scoped>
.failures {
  margin: 0;
  padding-left: 1.25rem;
  max-height: 40vh;
  overflow: auto;
}
</style>
