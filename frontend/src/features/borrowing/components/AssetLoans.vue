<script setup lang="ts">
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Tag from 'primevue/tag'
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import EmptyState from '@/components/EmptyState.vue'
import PersonCell from '@/components/PersonCell.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import type { Loan } from '@/lib/api/types'
import { formatDate } from '@/lib/dates'
import { useLoans } from '../api'
import { loanStateTag, requestCode } from '../borrowing'

// Lịch sử mượn của một tài sản (US-12, người quản lý): mọi khoản, mới nhất trước
const props = defineProps<{ assetId: string }>()
const { data, isLoading } = useLoans(computed(() => ({ asset_id: props.assetId, state: 'all' as const, sort: '-started_on' as const, page_size: 50 })))
</script>

<template>
  <DataTable :value="data?.items ?? []" data-key="id" scrollable>
    <Column header="Borrower">
      <template #body="{ data: l }: { data: Loan }"><PersonCell :name="l.borrower.name" :email="l.borrower.email" /></template>
    </Column>
    <Column header="Borrowed on">
      <template #body="{ data: l }: { data: Loan }">{{ formatDate(l.started_on) }}</template>
    </Column>
    <Column header="Return by">
      <template #body="{ data: l }: { data: Loan }">{{ formatDate(l.return_by) }}</template>
    </Column>
    <Column header="Returned on">
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
    <Column header="Note">
      <template #body="{ data: l }: { data: Loan }">
        <span v-if="l.return_note">{{ l.return_note }}</span>
        <span v-else class="muted">—</span>
      </template>
    </Column>
    <template #empty>
      <TableSkeleton v-if="isLoading" :rows="3" />
      <EmptyState v-else icon="pi pi-clock" text="Never lent." />
    </template>
  </DataTable>
</template>

<style scoped>
.muted {
  color: var(--p-text-muted-color);
}
</style>
