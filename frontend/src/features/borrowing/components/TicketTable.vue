<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable, { type DataTablePageEvent, type DataTableSortEvent } from 'primevue/datatable'
import IconField from 'primevue/iconfield'
import InputIcon from 'primevue/inputicon'
import InputText from 'primevue/inputtext'
import Tag from 'primevue/tag'
import type { MenuItem } from 'primevue/menuitem'
import { computed, ref, watch } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import EmptyState from '@/components/EmptyState.vue'
import PersonCell from '@/components/PersonCell.vue'
import RowMenuButton from '@/components/RowMenuButton.vue'
import RowMenus from '@/components/RowMenus.vue'
import SegmentedFilter, { type SegmentOption } from '@/components/SegmentedFilter.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import type { BorrowTicket, TicketState } from '@/lib/api/types'
import { formatDate } from '@/lib/dates'
import { openLocation } from '@/lib/navigation'
import { PAGE_SIZES, usePageSize } from '@/lib/preferences'
import { useKeepScrollOnPage, useListTable, useRowMenu } from '@/lib/tableRows'
import { queryInt, queryString, useUrlState } from '@/lib/urlState'
import { fromTableSort, toTableSort } from '@/features/assets/listQuery'
import { type TicketParams, useTickets, useTypeAttributes } from '../api'
import { requestCode, requirementLabels, TICKET_STATE_LABEL, TICKET_STATE_TONE } from '../borrowing'
import { useTicketActions } from '../useTicketActions'
import CodeTag from '@/components/CodeTag.vue'
import ApproveDialog from './ApproveDialog.vue'
import RejectDialog from './RejectDialog.vue'

// Bảng phiếu mượn dùng chung: My requests (mine: chỉ phiếu của mình) và Requests của người quản
// lý (cột người gửi, tìm kiếm, mặc định chỉ phiếu đang chờ). Bấm dòng mở trang phiếu.
const props = defineProps<{ mine: boolean }>()

type Filter = 'all' | 'waiting' | 'approved'
const FILTERS: Filter[] = ['all', 'waiting', 'approved']
const defaultFilter: Filter = props.mine ? 'all' : 'waiting'
// cột sắp xếp được (API sort); trống là thứ tự mặc định của API
type Sort = NonNullable<TicketParams['sort']>
const SORTS: Sort[] = ['number', '-number', 'needed_from', '-needed_from']
const { size: pageSize, set: setPageSize } = usePageSize(props.mine ? 'my-tickets' : 'tickets')
const { state, update } = useUrlState(
  (q) => {
    const f = String(q.show ?? defaultFilter)
    const sort = queryString(q.sort) as Sort | undefined
    return {
      show: (FILTERS.includes(f as Filter) ? f : defaultFilter) as Filter,
      q: queryString(q.q) ?? '',
      sort: sort && SORTS.includes(sort) ? sort : undefined,
      page: queryInt(q.page, 1),
    }
  },
  (s) => ({ show: s.show === defaultFilter ? undefined : s.show, q: s.q || undefined, sort: s.sort, page: s.page > 1 ? String(s.page) : undefined }),
)

const params = computed<TicketParams>(() => ({
  mine: props.mine || undefined,
  state: state.value.show === 'all' ? undefined : [state.value.show as TicketState],
  q: state.value.q || undefined,
  sort: state.value.sort,
  page: state.value.page,
  page_size: pageSize.value,
}))
const { data, isFetching, isLoading } = useTickets(params)
const rows = computed(() => data.value?.items ?? [])
const typeAttrs = useTypeAttributes(() => rows.value.map((t) => t.asset_type.id))
const specs = (t: BorrowTicket) => requirementLabels(t.requirements, typeAttrs.value[t.asset_type.id] ?? [])

const tableRef = ref<{ $el: HTMLElement }>()
const keepScroll = useKeepScrollOnPage(() => tableRef.value?.$el, data)

const show = computed({ get: () => state.value.show, set: (v: Filter) => update({ show: v, page: 1 }) })
const filterOptions = computed<SegmentOption<Filter>[]>(() =>
  props.mine
    ? [
        { label: 'All', value: 'all' },
        { label: 'Waiting', value: 'waiting' },
        { label: 'Ready for pickup', value: 'approved' },
      ]
    : [
        { label: 'Waiting', value: 'waiting' },
        { label: 'Ready for pickup', value: 'approved' },
        { label: 'All', value: 'all' },
      ],
)

const search = ref(state.value.q)
let timer: ReturnType<typeof setTimeout> | undefined
watch(search, (q) => {
  clearTimeout(timer)
  timer = setTimeout(() => update({ q, page: 1 }), 300)
})

function onPage(e: DataTablePageEvent) {
  if (e.rows !== pageSize.value) {
    setPageSize(e.rows)
    update({ page: 1 })
    return
  }
  keepScroll.beforePageChange()
  update({ page: e.page + 1 })
}

const tableSort = computed(() => toTableSort(state.value.sort))
function onSort(e: DataTableSortEvent) {
  const field = typeof e.sortField === 'string' ? e.sortField : undefined
  update({ sort: fromTableSort(field, e.sortOrder) as Sort | undefined, page: 1 })
}

const actions = useTicketActions()
const router = useRouter()
function openTicket(t: BorrowTicket, e?: MouseEvent, newTab?: boolean) {
  openLocation(router, `/borrowing/tickets/${t.id}`, e, newTab)
}
const rowMenu = useRowMenu<BorrowTicket>((t) => {
  const items: MenuItem[] = [
    { label: 'Open', icon: 'pi pi-arrow-right', command: () => openTicket(t) },
    { label: 'Open in new tab', icon: 'pi pi-external-link', command: () => openTicket(t, undefined, true) },
  ]
  const more = actions.menuItems(t)
  if (more.length) items.push({ separator: true }, ...more)
  return items
})
const table = useListTable({ open: openTicket, showMenu: rowMenu.showContext })
const emptyText = computed(() =>
  state.value.q ? 'No requests match this search.' : props.mine ? 'You have no requests here yet.' : 'No requests here.',
)
</script>

<template>
  <div class="toolbar">
    <SegmentedFilter v-model="show" :options="filterOptions" label="Show" />
    <IconField v-if="!mine">
      <InputIcon class="pi pi-search" />
      <InputText v-model="search" placeholder="Search person or type" aria-label="Search requests" />
    </IconField>
    <slot name="toolbar" />
  </div>
  <RowMenus :menu="rowMenu" />
  <DataTable
    ref="tableRef"
    :value="rows"
    lazy
    :paginator="!isLoading"
    :rows="pageSize"
    :rows-per-page-options="PAGE_SIZES"
    paginator-template="FirstPageLink PrevPageLink PageLinks NextPageLink LastPageLink CurrentPageReport RowsPerPageDropdown"
    current-page-report-template="Showing {first}–{last} of {totalRecords}"
    :first="(state.page - 1) * pageSize"
    :total-records="data?.total ?? 0"
    :loading="isFetching"
    data-key="id"
    scrollable
    :sort-field="tableSort.field"
    :sort-order="tableSort.order"
    removable-sort
    v-bind="table.bind"
    @page="onPage"
    @sort="onSort"
  >
    <Column header="Request" sort-field="number" sortable>
      <template #body="{ data: t }: { data: BorrowTicket }">
        <RouterLink :to="`/borrowing/tickets/${t.id}`"><CodeTag :code="requestCode(t.number)" /></RouterLink>
      </template>
    </Column>
    <Column v-if="!mine" header="Requested by">
      <template #body="{ data: t }: { data: BorrowTicket }">
        <PersonCell :name="t.requester.name" :email="t.requester.email" />
      </template>
    </Column>
    <Column header="Type">
      <template #body="{ data: t }: { data: BorrowTicket }">{{ t.asset_type.name }}</template>
    </Column>
    <Column header="Specs">
      <template #body="{ data: t }: { data: BorrowTicket }">
        <span v-if="t.requirements.length">{{ specs(t).join(' · ') }}</span>
        <span v-else class="muted">—</span>
      </template>
    </Column>
    <Column header="Needed from" sort-field="needed_from" sortable>
      <template #body="{ data: t }: { data: BorrowTicket }">{{ formatDate(t.needed_from) }}</template>
    </Column>
    <Column header="Return by">
      <template #body="{ data: t }: { data: BorrowTicket }">{{ formatDate(t.return_by) }}</template>
    </Column>
    <Column header="Status">
      <template #body="{ data: t }: { data: BorrowTicket }">
        <Tag :value="TICKET_STATE_LABEL[t.state]" :severity="TICKET_STATE_TONE[t.state]" />
      </template>
    </Column>
    <Column header="Asset">
      <template #body="{ data: t }: { data: BorrowTicket }">
        <RouterLink v-if="t.asset" :to="`/assets/${t.asset.id}`"><CodeTag :code="t.asset.tag" /></RouterLink>
        <span v-else class="muted">—</span>
      </template>
    </Column>
    <Column v-if="actions.canManage.value" header="">
      <template #body="{ data: t }: { data: BorrowTicket }">
        <div class="row-actions">
          <Button v-if="actions.can.approve(t)" label="Approve" size="small" text @click.stop="actions.askApprove(t)" />
          <Button v-if="actions.can.reject(t)" label="Reject" size="small" text severity="secondary" @click.stop="actions.askReject(t)" />
          <Button v-if="actions.can.handOver(t)" label="Hand over" size="small" text @click.stop="actions.doHandOver(t)" />
        </div>
      </template>
    </Column>
    <Column frozen align-frozen="right" header-class="row-menu-col" body-class="row-menu-col">
      <template #body="{ data: t, index }: { data: BorrowTicket; index: number }">
        <RowMenuButton :active="table.active.isActive(index)" @open="(e) => rowMenu.toggle(t, e)" />
      </template>
    </Column>
    <template #empty>
      <TableSkeleton v-if="isLoading" :rows="10" />
      <EmptyState v-else icon="pi pi-ticket" :text="emptyText" />
    </template>
  </DataTable>
  <ApproveDialog v-model:visible="actions.approveOpen.value" :ticket="actions.approveTarget.value" />
  <RejectDialog v-model:visible="actions.rejectOpen.value" :ticket="actions.rejectTarget.value" :cancel="actions.rejectCancel.value" />
</template>

<style scoped>
.muted {
  color: var(--p-text-muted-color);
}
.row-actions {
  display: flex;
  gap: 0.15rem;
  white-space: nowrap;
}
</style>
