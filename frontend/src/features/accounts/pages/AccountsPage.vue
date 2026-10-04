<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable, { type DataTablePageEvent } from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Tag from 'primevue/tag'
import { computed, ref, watch } from 'vue'
import type { Account } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { formatDateTime } from '@/lib/dates'
import { PAGE_SIZES, usePageSize } from '@/lib/preferences'
import { queryInt, queryString, useUrlState } from '@/lib/urlState'
import { useAccounts } from '../api'
import CreateAccountDialog from '../components/CreateAccountDialog.vue'
import { statusSeverity } from '../status'

const session = useSession()
const { size: pageSize, set: setPageSize } = usePageSize('accounts')

type Active = 'all' | 'active' | 'disabled'
const { state, update } = useUrlState(
  (q) => ({
    q: queryString(q.q) ?? '',
    active: (['active', 'disabled'].includes(String(q.active)) ? q.active : 'all') as Active,
    page: queryInt(q.page, 1),
  }),
  (s) => ({
    q: s.q || undefined,
    active: s.active === 'all' ? undefined : s.active,
    page: s.page > 1 ? String(s.page) : undefined,
  }),
)

const params = computed(() => ({
  q: state.value.q || undefined,
  active: state.value.active === 'all' ? undefined : state.value.active === 'active',
  page: state.value.page,
  page_size: pageSize.value,
}))
const { data, isFetching } = useAccounts(params)

// Ô tìm kiếm: đợi gõ xong rồi mới đổi URL
const search = ref(state.value.q)
let timer: ReturnType<typeof setTimeout> | undefined
watch(search, (q) => {
  clearTimeout(timer)
  timer = setTimeout(() => update({ q, page: 1 }), 300)
})

const activeOptions = [
  { label: 'All', value: 'all' },
  { label: 'Active', value: 'active' },
  { label: 'Disabled', value: 'disabled' },
]

function onPage(e: DataTablePageEvent) {
  // đổi số dòng thì về trang 1
  if (e.rows !== pageSize.value) {
    setPageSize(e.rows)
    update({ page: 1 })
    return
  }
  update({ page: e.page + 1 })
}

const creating = ref(false)
</script>

<template>
  <section>
    <div class="page-header">
      <h1>Accounts</h1>
      <Button v-if="session.can(Perm.AccountManage)" label="New account" icon="pi pi-plus" @click="creating = true" />
    </div>
    <div class="toolbar">
      <InputText v-model="search" placeholder="Search name or email" />
      <Select
        :model-value="state.active"
        :options="activeOptions"
        option-label="label"
        option-value="value"
        @update:model-value="(v: Active) => update({ active: v, page: 1 })"
      />
    </div>
    <DataTable
      :value="data?.items ?? []"
      lazy
      paginator
      :rows="pageSize"
      :rows-per-page-options="PAGE_SIZES"
      :first="(state.page - 1) * pageSize"
      :total-records="data?.total ?? 0"
      :loading="isFetching"
      data-key="id"
      @page="onPage"
    >
      <Column header="Name">
        <template #body="{ data: a }: { data: Account }">
          <RouterLink :to="`/accounts/${a.id}`">{{ a.name }}</RouterLink>
        </template>
      </Column>
      <Column field="email" header="Email" />
      <Column header="Status">
        <template #body="{ data: a }: { data: Account }">
          <Tag :value="a.status" :severity="statusSeverity(a.status)" />
        </template>
      </Column>
      <Column header="Created">
        <template #body="{ data: a }: { data: Account }">{{ formatDateTime(a.created_at) }}</template>
      </Column>
      <template #empty>No accounts found.</template>
    </DataTable>
    <CreateAccountDialog v-model:visible="creating" />
  </section>
</template>
