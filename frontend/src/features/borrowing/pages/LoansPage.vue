<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable, { type DataTablePageEvent, type DataTableSortEvent } from 'primevue/datatable'
import DatePicker from 'primevue/datepicker'
import IconField from 'primevue/iconfield'
import InputIcon from 'primevue/inputicon'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Tag from 'primevue/tag'
import { computed, ref, watch } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import PersonCell from '@/components/PersonCell.vue'
import RowMenuButton from '@/components/RowMenuButton.vue'
import RowMenus from '@/components/RowMenus.vue'
import SegmentedFilter, { type SegmentOption } from '@/components/SegmentedFilter.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import type { Loan } from '@/lib/api/types'
import { formatDate, fromDateString, toDateString } from '@/lib/dates'
import { openLocation } from '@/lib/navigation'
import { PAGE_SIZES, usePageSize } from '@/lib/preferences'
import { useKeepScrollOnPage, useListTable, useRowMenu } from '@/lib/tableRows'
import { queryInt, queryString, useUrlState } from '@/lib/urlState'
import { fromTableSort, toTableSort } from '@/features/assets/listQuery'
import { useQueryClient } from '@tanstack/vue-query'
import { assetKeys, useAsset } from '@/features/assets/api'
import { type LoanParams, useBusinessDay, useLoans } from '../api'
import { loanStateTag, type Period, periodRange, requestCode } from '../borrowing'
import ReturnDialog from '../components/ReturnDialog.vue'

// Khoản mượn (US-10..12): Out now (mặc định, quá hạn lên trước), Returned hay All; lọc theo kỳ
// (khoản ở ngoài lúc nào đó trong kỳ) và từ khoá (tag/tên tài sản, người mượn). Bấm dòng mở tài
// sản. mine: khoản của chính mình (My loans), chỉ xem.
const props = defineProps<{ mine?: boolean }>()
type Sort = NonNullable<LoanParams['sort']>
type Show = 'active' | 'returned' | 'all'
const SORTS: Sort[] = ['started_on', '-started_on', 'return_by', '-return_by', 'returned_on', '-returned_on']
const SHOWS: Show[] = ['active', 'returned', 'all']
const PERIODS: Period[] = ['any', 'this_month', 'last_month', 'last_30', 'custom']
const { size: pageSize, set: setPageSize } = usePageSize(props.mine ? 'my-loans' : 'loans')
const { state, update } = useUrlState(
  (q) => {
    const sort = queryString(q.sort) as Sort | undefined
    const show = queryString(q.show) as Show | undefined
    const period = queryString(q.period) as Period | undefined
    return {
      show: show && SHOWS.includes(show) ? show : ('active' as Show),
      period: period && PERIODS.includes(period) ? period : ('any' as Period),
      from: queryString(q.from) ?? '',
      to: queryString(q.to) ?? '',
      q: queryString(q.q) ?? '',
      sort: sort && SORTS.includes(sort) ? sort : undefined,
      page: queryInt(q.page, 1),
    }
  },
  (s) => ({
    show: s.show === 'active' ? undefined : s.show,
    period: s.period === 'any' ? undefined : s.period,
    from: s.period === 'custom' && s.from ? s.from : undefined,
    to: s.period === 'custom' && s.to ? s.to : undefined,
    q: s.q || undefined,
    sort: s.sort,
    page: s.page > 1 ? String(s.page) : undefined,
  }),
)

const today = useBusinessDay(true)
// kỳ đang lọc: lựa chọn sẵn tính từ hôm nay của ứng dụng; Custom lấy from/to trên URL
const range = computed<{ from?: string; to?: string }>(() => {
  const p = state.value.period
  if (p === 'any') return {}
  if (p === 'custom') return { from: state.value.from || undefined, to: state.value.to || undefined }
  return periodRange(p, today.value)
})
const params = computed<LoanParams>(() => ({
  mine: props.mine || undefined,
  state: state.value.show,
  ...range.value,
  q: state.value.q || undefined,
  sort: state.value.sort,
  page: state.value.page,
  page_size: pageSize.value,
}))

const showOptions: SegmentOption<Show>[] = [
  { label: 'Out now', value: 'active' },
  { label: 'Returned', value: 'returned' },
  { label: 'All', value: 'all' },
]
const show = computed({ get: () => state.value.show, set: (v: Show) => update({ show: v, page: 1 }) })
const periodOptions: { label: string; value: Period }[] = [
  { label: 'Any time', value: 'any' },
  { label: 'This month', value: 'this_month' },
  { label: 'Last month', value: 'last_month' },
  { label: 'Last 30 days', value: 'last_30' },
  { label: 'Custom range', value: 'custom' },
]
const customRange = computed(() => [fromDateString(state.value.from), fromDateString(state.value.to)].filter((d): d is Date => !!d))
// chọn khoảng: chỉ lọc khi đã có đủ hai đầu
function onRange(v: unknown) {
  const [from, to] = Array.isArray(v) ? (v as (Date | null)[]) : []
  if (from && to) update({ from: toDateString(from) ?? '', to: toDateString(to) ?? '', page: 1 })
}

const search = ref(state.value.q)
let timer: ReturnType<typeof setTimeout> | undefined
watch(search, (q) => {
  clearTimeout(timer)
  timer = setTimeout(() => update({ q, page: 1 }), 300)
})

const emptyText = computed(() => {
  if (state.value.q || state.value.period !== 'any') return 'No loans match these filters.'
  if (state.value.show === 'active') return props.mine ? 'You have nothing on loan.' : 'Nothing is on loan right now.'
  return props.mine ? "You haven't returned anything yet." : 'No loans yet.'
})

const tableSort = computed(() => toTableSort(state.value.sort))
function onSort(e: DataTableSortEvent) {
  const field = typeof e.sortField === 'string' ? e.sortField : undefined
  update({ sort: fromTableSort(field, e.sortOrder) as Sort | undefined, page: 1 })
}
const { data, isFetching, isLoading } = useLoans(params)
const tableRef = ref<{ $el: HTMLElement }>()
const keepScroll = useKeepScrollOnPage(() => tableRef.value?.$el, data)

function onPage(e: DataTablePageEvent) {
  if (e.rows !== pageSize.value) {
    setPageSize(e.rows)
    update({ page: 1 })
    return
  }
  keepScroll.beforePageChange()
  update({ page: e.page + 1 })
}
const router = useRouter()
const openAsset = (l: Loan, e?: MouseEvent, newTab?: boolean) => openLocation(router, `/assets/${l.asset.id}`, e, newTab)

// Ghi nhận trả: dòng khoản mượn không có version tài sản, nên đọc tài sản trước khi mở hộp
const returning = ref<Loan | null>(null)
const returnOpen = ref(false)
const qc = useQueryClient()
const { data: returningAsset, isFetching: assetLoading } = useAsset(() => returning.value?.asset.id)
// chờ bản đọc mới: version trong cache có thể đã cũ (người khác vừa sửa tài sản)
const returnTarget = computed(() =>
  returning.value && !assetLoading.value && returningAsset.value?.id === returning.value.asset.id
    ? { id: returningAsset.value.id, tag: returningAsset.value.tag, version: returningAsset.value.version, startedOn: returning.value.started_on }
    : null,
)
function askReturn(l: Loan) {
  qc.invalidateQueries({ queryKey: assetKeys.one(l.asset.id) })
  returning.value = l
  returnOpen.value = true
}

const rowMenu = useRowMenu<Loan>((l) => [
  { label: 'Open asset', icon: 'pi pi-arrow-right', command: () => openAsset(l) },
  { label: 'Open in new tab', icon: 'pi pi-external-link', command: () => openAsset(l, undefined, true) },
  ...(props.mine || l.returned_on ? [] : [{ separator: true }, { label: 'Record return…', icon: 'pi pi-reply', command: () => askReturn(l) }]),
])
const table = useListTable({ open: openAsset, showMenu: rowMenu.showContext })
</script>

<template>
  <section>
    <PageHeader v-if="mine" title="My loans" subtitle="What you have now, when it's due back, and what you returned." />
    <PageHeader v-else title="Loans" subtitle="What is out now, overdue first, and every loan before it." />
    <div class="toolbar">
      <SegmentedFilter v-model="show" :options="showOptions" label="Show" />
      <Select
        :model-value="state.period"
        :options="periodOptions"
        option-label="label"
        option-value="value"
        aria-label="Period"
        @update:model-value="(v: Period) => update({ period: v, page: 1 })"
      />
      <DatePicker
        v-if="state.period === 'custom'"
        :model-value="customRange"
        selection-mode="range"
        date-format="yy-mm-dd"
        placeholder="Pick a range"
        :manual-input="false"
        show-icon
        @update:model-value="onRange"
      />
      <IconField v-if="!mine">
        <InputIcon class="pi pi-search" />
        <InputText v-model="search" placeholder="Search asset or person" aria-label="Search loans" />
      </IconField>
    </div>
    <RowMenus :menu="rowMenu" />
    <DataTable
      ref="tableRef"
      :value="data?.items ?? []"
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
      <Column header="Asset">
        <template #body="{ data: l }: { data: Loan }">
          <RouterLink :to="`/assets/${l.asset.id}`" class="mono">{{ l.asset.tag }}</RouterLink>
          <span class="cell-sub">{{ l.asset.name }}</span>
        </template>
      </Column>
      <Column header="Type" field="asset_type_name" />
      <Column v-if="!mine" header="Borrower">
        <template #body="{ data: l }: { data: Loan }"><PersonCell :name="l.borrower.name" :email="l.borrower.email" /></template>
      </Column>
      <Column header="Borrowed on" sort-field="started_on" sortable>
        <template #body="{ data: l }: { data: Loan }">{{ formatDate(l.started_on) }}</template>
      </Column>
      <Column header="Return by" sort-field="return_by" sortable>
        <template #body="{ data: l }: { data: Loan }">{{ formatDate(l.return_by) }}</template>
      </Column>
      <Column v-if="state.show !== 'active'" header="Returned on" sort-field="returned_on" sortable>
        <template #body="{ data: l }: { data: Loan }">{{ l.returned_on ? formatDate(l.returned_on) : '—' }}</template>
      </Column>
      <Column header="Status">
        <template #body="{ data: l }: { data: Loan }">
          <Tag :value="loanStateTag(l).label" :severity="loanStateTag(l).tone" />
        </template>
      </Column>
      <Column header="Request">
        <template #body="{ data: l }: { data: Loan }">
          <RouterLink v-if="l.ticket_id" :to="`/borrowing/tickets/${l.ticket_id}`">{{ requestCode(l.ticket_number) }}</RouterLink>
          <span v-else class="muted">Lent directly</span>
        </template>
      </Column>
      <Column v-if="!mine" header="">
        <template #body="{ data: l }: { data: Loan }">
          <Button v-if="!l.returned_on" label="Record return" size="small" text @click.stop="askReturn(l)" />
        </template>
      </Column>
      <Column frozen align-frozen="right" header-class="row-menu-col" body-class="row-menu-col">
        <template #body="{ data: l, index }: { data: Loan; index: number }">
          <RowMenuButton :active="table.active.isActive(index)" @open="(e) => rowMenu.toggle(l, e)" />
        </template>
      </Column>
      <template #empty>
        <TableSkeleton v-if="isLoading" :rows="8" />
        <EmptyState v-else icon="pi pi-clock" :text="emptyText" />
      </template>
    </DataTable>
    <ReturnDialog v-if="!mine" v-model:visible="returnOpen" :asset="returnTarget" />
  </section>
</template>

<style scoped>
.mono {
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-weight: 600;
}
.cell-sub {
  display: block;
  margin-top: 0.2rem;
  font-size: 0.78rem;
  color: var(--p-text-muted-color);
}
.muted {
  color: var(--p-text-muted-color);
}
</style>
