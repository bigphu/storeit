<script setup lang="ts">
import Column from 'primevue/column'
import DataTable, { type DataTablePageEvent, type DataTableSortEvent } from 'primevue/datatable'
import Tag from 'primevue/tag'
import { computed, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import PersonCell from '@/components/PersonCell.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import type { Loan } from '@/lib/api/types'
import { formatDate } from '@/lib/dates'
import { openLocation } from '@/lib/navigation'
import { PAGE_SIZES, usePageSize } from '@/lib/preferences'
import { useKeepScrollOnPage, useListTable } from '@/lib/tableRows'
import { queryInt, queryString, useUrlState } from '@/lib/urlState'
import { fromTableSort, toTableSort } from '@/features/assets/listQuery'
import { type LoanParams, useLoans } from '../api'
import { overdueLabel } from '../borrowing'

// Tài sản đang cho mượn (người quản lý), mặc định quá hạn lên trước; Since và Return by sắp xếp
// được. Bấm dòng mở tài sản.
type Sort = NonNullable<LoanParams['sort']>
const SORTS: Sort[] = ['started_on', '-started_on', 'return_by', '-return_by']
const { size: pageSize, set: setPageSize } = usePageSize('loans')
const { state, update } = useUrlState(
  (q) => {
    const sort = queryString(q.sort) as Sort | undefined
    return { sort: sort && SORTS.includes(sort) ? sort : undefined, page: queryInt(q.page, 1) }
  },
  (s) => ({ sort: s.sort, page: s.page > 1 ? String(s.page) : undefined }),
)
const params = computed<LoanParams>(() => ({ sort: state.value.sort, page: state.value.page, page_size: pageSize.value }))
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
const table = useListTable({ open: (l: Loan, e?: MouseEvent) => openLocation(router, `/assets/${l.asset.id}`, e) })
</script>

<template>
  <section>
    <PageHeader title="On loan" subtitle="Assets lent out now, overdue first." />
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
      <Column header="Borrower">
        <template #body="{ data: l }: { data: Loan }"><PersonCell :name="l.borrower.name" :email="l.borrower.email" /></template>
      </Column>
      <Column header="Since" sort-field="started_on" sortable>
        <template #body="{ data: l }: { data: Loan }">{{ formatDate(l.started_on) }}</template>
      </Column>
      <Column header="Return by" sort-field="return_by" sortable>
        <template #body="{ data: l }: { data: Loan }">{{ formatDate(l.return_by) }}</template>
      </Column>
      <Column header="Status">
        <template #body="{ data: l }: { data: Loan }">
          <Tag v-if="l.overdue_days > 0" :value="overdueLabel(l.overdue_days)" severity="danger" />
          <Tag v-else value="On loan" severity="info" />
        </template>
      </Column>
      <Column header="Ticket">
        <template #body="{ data: l }: { data: Loan }">
          <RouterLink v-if="l.ticket_id" :to="`/borrowing/tickets/${l.ticket_id}`">#{{ l.ticket_number }}</RouterLink>
          <span v-else class="muted">Lent directly</span>
        </template>
      </Column>
      <template #empty>
        <TableSkeleton v-if="isLoading" :rows="8" />
        <EmptyState v-else icon="pi pi-clock" text="Nothing is on loan right now." />
      </template>
    </DataTable>
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
