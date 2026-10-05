<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import Message from 'primevue/message'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import { computed, ref, watch } from 'vue'
import { notify } from '@/lib/notify'
import { useStatuses } from '@/features/statuses/api'
import { useBulkRetire, useBulkStatus } from '../api'
import { type BulkSummary, summarizeBulk } from '../bulk'

// Đổi status hay retire các tài sản đã chọn. Sau khi chạy: tài sản không làm được
// được liệt kê kèm lý do (người khác vừa sửa, đã retire...)
const props = defineProps<{ mode: 'status' | 'retire'; rows: { id: string; tag: string; version: number }[] }>()
const visible = defineModel<boolean>('visible', { required: true })
const emit = defineEmits<{ done: [] }>()

const retire = useBulkRetire()
const setStatus = useBulkStatus()
const busy = computed(() => retire.isPending.value || setStatus.isPending.value)

// status chọn được: không thuộc kind retired (dùng Retire), không lưu trữ
const { data: statuses } = useStatuses()
const statusOptions = computed(() => (statuses.value ?? []).filter((s) => s.kind !== 'retired'))

const statusId = ref<string | null>(null)
const reason = ref('')
const summary = ref<BulkSummary | null>(null)

watch(visible, (open) => {
  if (open) {
    statusId.value = null
    reason.value = ''
    summary.value = null
  }
})

const count = computed(() => `${props.rows.length} ${props.rows.length === 1 ? 'asset' : 'assets'}`)
const header = computed(() =>
  summary.value ? 'Some assets were not changed' : props.mode === 'retire' ? `Retire ${count.value}?` : `Change status of ${count.value}`,
)

async function run() {
  const items = props.rows.map((r) => ({ id: r.id, version: r.version }))
  try {
    const result =
      props.mode === 'retire'
        ? await retire.mutateAsync({ items, reason: reason.value })
        : await setStatus.mutateAsync({ items, statusId: statusId.value! })
    const s = summarizeBulk(result, props.rows, props.mode === 'retire' ? 'retired' : 'updated')
    emit('done')
    if (s.failures.length) summary.value = s
    else {
      notify.success(s.message)
      visible.value = false
    }
  } catch {
    // lỗi của cả yêu cầu đã hiện bằng toast mặc định
  }
}
</script>

<template>
  <Dialog v-model:visible="visible" modal :header="header" :style="{ width: 'min(92vw, 32rem)' }">
    <div v-if="summary" class="form">
      <Message severity="warn">{{ summary.message }} Reload the list and try those again.</Message>
      <ul class="failures">
        <li v-for="f in summary.failures" :key="f.tag">
          <b>{{ f.tag }}</b>: {{ f.reason }}
        </li>
      </ul>
      <div class="actions"><Button label="Close" @click="visible = false" /></div>
    </div>

    <form v-else-if="mode === 'retire'" class="form" @submit.prevent="run">
      <p>Retired assets leave the list unless “Include retired” is on, and can't be edited until restored.</p>
      <div class="field">
        <label for="bulk-reason">Reason (optional, the same for all)</label>
        <Textarea id="bulk-reason" v-model="reason" rows="3" maxlength="500" />
      </div>
      <div class="actions">
        <Button type="submit" :label="`Retire ${count}`" severity="danger" :loading="busy" />
        <Button label="Cancel" severity="secondary" text @click="visible = false" />
      </div>
    </form>

    <form v-else class="form" @submit.prevent="run">
      <div class="field">
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
      <div class="actions">
        <Button type="submit" :label="`Update ${count}`" :disabled="!statusId" :loading="busy" />
        <Button label="Cancel" severity="secondary" text @click="visible = false" />
      </div>
    </form>
  </Dialog>
</template>

<style scoped>
.failures {
  margin: 0;
  padding-left: 1.25rem;
  max-height: 40vh;
  overflow: auto;
}
</style>
