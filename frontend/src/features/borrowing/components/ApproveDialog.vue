<script setup lang="ts">
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import RadioButton from 'primevue/radiobutton'
import { useQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import FormDialog from '@/components/FormDialog.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import EmptyState from '@/components/EmptyState.vue'
import { inventoryApi } from '@/lib/api/client'
import type { AssetListItem, AttributeValue, BorrowTicket } from '@/lib/api/types'
import { formatDate } from '@/lib/dates'
import { unwrap } from '@/lib/errors'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { assetKeys } from '@/features/assets/api'
import { useAssetType } from '@/features/asset-types/api'
import { useApproveTicket } from '../api'
import { parseRequirement, requirementLabels, satisfies, shortDay } from '../borrowing'

// Duyệt phiếu: chọn một tài sản đang rảnh của loại được hỏi. Bảng có một cột cho mỗi yêu cầu;
// tài sản thoả mọi yêu cầu lên trước, còn lại mờ đi và gạch ô không thoả. Duyệt là giữ chỗ
// tài sản (Reserved) và người gửi nhận thư "sẵn sàng để nhận".
const props = defineProps<{ ticket: BorrowTicket | null }>()
const visible = defineModel<boolean>('visible', { required: true })

const typeId = computed(() => props.ticket?.asset_type.id)
const { data: type } = useAssetType(typeId)
const assets = useQuery({
  queryKey: computed(() => [...assetKeys.all, 'borrowable', typeId.value ?? ''] as const),
  queryFn: () =>
    unwrap(
      inventoryApi.GET('/assets', {
        params: { query: { type_id: typeId.value, borrowable: true, sort: 'tag', page: 1, page_size: 200 } },
      }),
    ),
  enabled: computed(() => visible.value && !!typeId.value),
})

const attrs = computed(() => type.value?.attributes ?? [])
// yêu cầu còn khớp một thuộc tính của loại (thuộc tính đã gỡ thì bỏ khỏi việc so)
const reqs = computed(() =>
  (props.ticket?.requirements ?? [])
    .map((r) => ({ raw: r, row: parseRequirement(r) }))
    .filter((r) => r.row && attrs.value.some((a) => a.key === r.row!.key))
    .map((r) => ({ ...r.row!, label: requirementLabels([r.raw], attrs.value)[0] })),
)

interface Row extends AssetListItem {
  group: 'match' | 'other'
  fails: boolean[]
  values: (AttributeValue | undefined)[]
}
const rows = computed<Row[]>(() => {
  const out = (assets.data.value?.items ?? []).map((a) => {
    const values = reqs.value.map((r) => a.attributes?.find((v) => v.key === r.key))
    const fails = reqs.value.map((r, i) => !satisfies(r, values[i]))
    return { ...a, values, fails, group: fails.some(Boolean) ? 'other' : 'match' } as Row
  })
  // nhóm "khớp" trước, giữ thứ tự tag trong mỗi nhóm
  return [...out.filter((r) => r.group === 'match'), ...out.filter((r) => r.group === 'other')]
})
const matchCount = computed(() => rows.value.filter((r) => r.group === 'match').length)

const errors = useFormErrors()
const approve = useApproveTicket()
const picked = ref<string>()
watch(
  [visible, rows],
  ([open], [wasOpen]) => {
    if (!open) return
    if (!wasOpen) errors.clear()
    if (picked.value && rows.value.some((r) => r.id === picked.value)) return
    // chọn sẵn tài sản người gửi gợi ý nếu còn rảnh, không thì tài sản khớp đầu tiên
    const suggested = props.ticket?.suggested_asset?.id
    picked.value = rows.value.find((r) => r.id === suggested)?.id ?? rows.value.find((r) => r.group === 'match')?.id
  },
  { immediate: true },
)
watch(visible, (open) => {
  if (!open) picked.value = undefined
})
const pickedRow = computed(() => rows.value.find((r) => r.id === picked.value))

function cellText(v: AttributeValue | undefined): string {
  if (!v || v.value === null || v.value === undefined || v.value === '') return '—'
  if (v.data_type === 'select') return v.option_label ?? String(v.value)
  if (v.data_type === 'boolean') return v.value ? 'Yes' : 'No'
  return v.unit ? `${v.value} ${v.unit}` : String(v.value)
}

async function submit() {
  const t = props.ticket
  const row = pickedRow.value
  if (!t || !row) return
  errors.clear()
  try {
    await approve.mutateAsync({ id: t.id, assetId: row.id, version: t.version })
    notify.success(`${row.tag} reserved for ${t.requester.name}.`)
    visible.value = false
  } catch (err) {
    errors.set(err)
    // tài sản vừa bị người khác lấy: tải lại danh sách rảnh
    void assets.refetch()
  }
}
</script>

<template>
  <FormDialog
    v-model:visible="visible"
    icon="tick"
    size="l"
    :title="ticket ? `Approve ticket #${ticket.number}` : 'Approve ticket'"
    :action="pickedRow ? `Approve and reserve ${pickedRow.tag}` : 'Approve and reserve'"
    :busy="approve.isPending.value"
    :error="errors.general.value"
    :disabled="!pickedRow"
    @submit="submit"
  >
    <template v-if="ticket">
      <p class="summary">
        <strong>{{ ticket.requester.name }}</strong> wants a <strong>{{ ticket.asset_type.name }}</strong>
        from {{ shortDay(ticket.needed_from) }}<template v-if="ticket.return_by"> to {{ shortDay(ticket.return_by) }}</template>.
        <span v-if="ticket.note" class="muted">“{{ ticket.note }}”</span>
      </p>
      <p class="muted lead">
        {{ matchCount }} of {{ rows.length }} available {{ ticket.asset_type.name }} assets match every spec.
        {{ ticket.requester.name }} gets an email that it's ready for pickup.
      </p>
      <DataTable
        :value="rows"
        data-key="id"
        row-group-mode="subheader"
        group-rows-by="group"
        scrollable
        scroll-height="22rem"
        size="small"
        :row-class="(r: Row) => (r.group === 'other' ? 'other' : '')"
        @row-click="(e) => (picked = (e.data as Row).id)"
      >
        <template #groupheader="{ data: r }: { data: Row }">
          <span class="group">{{ r.group === 'match' ? `Matches all specs · ${matchCount}` : `Other available · ${rows.length - matchCount}` }}</span>
        </template>
        <Column header-style="width: 2.5rem">
          <template #body="{ data: r }: { data: Row }">
            <RadioButton v-model="picked" :value="r.id" :input-id="`pick-${r.id}`" name="approve-asset" />
          </template>
        </Column>
        <Column header="Tag">
          <template #body="{ data: r }: { data: Row }"><label :for="`pick-${r.id}`" class="mono">{{ r.tag }}</label></template>
        </Column>
        <Column field="name" header="Name" />
        <Column v-for="(req, i) in reqs" :key="req.key + i" :header="req.label">
          <template #body="{ data: r }: { data: Row }">
            <span :class="{ miss: r.fails[i] }">{{ cellText(r.values[i]) }}</span>
          </template>
        </Column>
        <Column field="status_name" header="Status" />
        <Column header="Purchased">
          <template #body="{ data: r }: { data: Row }">{{ formatDate(r.purchase_date) }}</template>
        </Column>
        <template #empty>
          <TableSkeleton v-if="assets.isLoading.value" :rows="4" />
          <EmptyState v-else icon="pi pi-box" :text="`No ${ticket.asset_type.name} is available right now.`" />
        </template>
      </DataTable>
      <small v-if="errors.fields.value.asset_id" class="field-error">{{ errors.fields.value.asset_id }}</small>
    </template>
  </FormDialog>
</template>

<style scoped>
.summary {
  margin: 0 0 0.35rem;
}
.lead {
  margin: 0 0 0.75rem;
}
.muted {
  color: var(--p-text-muted-color);
}
.group {
  font-weight: 600;
  color: var(--p-text-muted-color);
}
:deep(tr.other) {
  opacity: 0.6;
}
.miss {
  color: var(--p-red-500);
  text-decoration: line-through;
}
.mono {
  font-family: var(--app-font-mono, ui-monospace, monospace);
  cursor: pointer;
}
</style>
