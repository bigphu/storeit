<script setup lang="ts">
import Avatar from 'primevue/avatar'
import Button from 'primevue/button'
import Chip from 'primevue/chip'
import Column from 'primevue/column'
import ContextMenu from 'primevue/contextmenu'
import DataTable, { type DataTablePageEvent } from 'primevue/datatable'
import IconField from 'primevue/iconfield'
import InputIcon from 'primevue/inputicon'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import SelectButton from 'primevue/selectbutton'
import Tag from 'primevue/tag'
import type { MenuItem } from 'primevue/menuitem'
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useRoles } from '@/features/roles/api'
import type { AccountListItem } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { formatDay } from '@/lib/dates'
import { openLocation } from '@/lib/navigation'
import { initials, inviteNote } from '@/lib/people'
import { PAGE_SIZES, usePageSize } from '@/lib/preferences'
import { onRowClick, useRowMenu } from '@/lib/tableRows'
import { queryInt, queryString, useUrlState } from '@/lib/urlState'
import { type AccountStatus, useAccounts } from '../api'
import CreateAccountDialog from '../components/CreateAccountDialog.vue'
import { statusSeverity } from '../status'
import { useAccountActions } from '../useAccountActions'

const session = useSession()
const canManage = computed(() => session.can(Perm.AccountManage))
const { size: pageSize, set: setPageSize } = usePageSize('accounts')
const roles = useRoles()

type Filter = 'all' | AccountStatus
const STATUSES: AccountStatus[] = ['active', 'invited', 'disabled']
const { state, update } = useUrlState(
  (q) => {
    // link cũ ?active=false vẫn mở đúng danh sách bị khoá
    const legacy = q.active === 'false' ? 'disabled' : undefined
    const status = String(q.status ?? legacy ?? '')
    return {
      q: queryString(q.q) ?? '',
      status: (STATUSES.includes(status as AccountStatus) ? status : 'all') as Filter,
      role: queryString(q.role) ?? '',
      page: queryInt(q.page, 1),
    }
  },
  (s) => ({
    q: s.q || undefined,
    status: s.status === 'all' ? undefined : s.status,
    role: s.role || undefined,
    page: s.page > 1 ? String(s.page) : undefined,
  }),
)

const params = computed(() => ({
  q: state.value.q || undefined,
  status: state.value.status === 'all' ? undefined : state.value.status,
  role_id: state.value.role || undefined,
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

// Nút lọc trạng thái có số đếm (cùng tìm kiếm và role)
const statusOptions = computed(() => {
  const c = data.value?.status_counts
  const n = (s: AccountStatus) => c?.[s] ?? 0
  return [
    { label: 'All', value: 'all', count: c ? n('active') + n('invited') + n('disabled') : undefined },
    { label: 'Active', value: 'active', count: c ? n('active') : undefined },
    { label: 'Invited', value: 'invited', count: c ? n('invited') : undefined },
    { label: 'Disabled', value: 'disabled', count: c ? n('disabled') : undefined },
  ]
})
const roleOptions = computed(() => [{ name: 'Any role', id: '' }, ...(roles.data.value ?? [])])

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
const actions = useAccountActions()
const label = (s: AccountStatus) => s[0].toUpperCase() + s.slice(1)

// Bấm dòng mở tài khoản (Ctrl/⌘ mở tab mới); chuột phải có menu mở và các hành động
const router = useRouter()
function openAccount(a: AccountListItem, e?: MouseEvent, newTab?: boolean) {
  openLocation(router, `/accounts/${a.id}`, e, newTab)
}
const rowClick = onRowClick(openAccount)
const menu = ref<InstanceType<typeof ContextMenu>>()
const { items: menuItems, show: showMenu, clear: clearMenu } = useRowMenu<AccountListItem>(menu, (a) => {
  const items: MenuItem[] = [
    { label: 'Open', icon: 'pi pi-arrow-right', command: () => openAccount(a) },
    { label: 'Open in new tab', icon: 'pi pi-external-link', command: () => openAccount(a, undefined, true) },
  ]
  if (!canManage.value) return items
  items.push({ separator: true })
  if (a.status === 'invited') items.push({ label: 'Resend invitation', icon: 'pi pi-envelope', command: () => actions.resendInvitation(a) })
  if (a.status === 'active') items.push({ label: 'Send reset link', icon: 'pi pi-key', command: () => actions.sendReset(a) })
  if (a.status === 'disabled') items.push({ label: 'Enable', icon: 'pi pi-check-circle', command: () => actions.enable(a) })
  else items.push({ label: 'Disable', icon: 'pi pi-ban', disabled: actions.isSelf(a), command: () => actions.disable(a) })
  return items
})
</script>

<template>
  <section>
    <div class="page-header">
      <div>
        <h1>Accounts</h1>
        <p class="sub">People who can sign in to StoreIt and the roles they hold.</p>
      </div>
      <Button v-if="canManage" label="Invite account" icon="pi pi-envelope" @click="creating = true" />
    </div>
    <div class="toolbar">
      <SelectButton
        :model-value="state.status"
        :options="statusOptions"
        option-value="value"
        :allow-empty="false"
        aria-label="Status"
        @update:model-value="(v: Filter) => update({ status: v, page: 1 })"
      >
        <template #option="{ option }">
          {{ option.label }} <span v-if="option.count !== undefined" class="seg-count">{{ option.count }}</span>
        </template>
      </SelectButton>
      <IconField>
        <InputIcon class="pi pi-search" />
        <InputText v-model="search" placeholder="Search name or email" aria-label="Search accounts" />
      </IconField>
      <Select
        :model-value="state.role"
        :options="roleOptions"
        option-label="name"
        option-value="id"
        aria-label="Role"
        @update:model-value="(v: string) => update({ role: v, page: 1 })"
      />
    </div>
    <ContextMenu ref="menu" :model="menuItems" @hide="clearMenu" />
    <DataTable
      :value="data?.items ?? []"
      lazy
      paginator
      :rows="pageSize"
      :rows-per-page-options="PAGE_SIZES"
      paginator-template="FirstPageLink PrevPageLink PageLinks NextPageLink LastPageLink CurrentPageReport RowsPerPageDropdown"
      current-page-report-template="Showing {first}–{last} of {totalRecords}"
      :first="(state.page - 1) * pageSize"
      :total-records="data?.total ?? 0"
      :loading="isFetching"
      data-key="id"
      row-hover
      :row-class="() => 'clickable-row'"
      @page="onPage"
      @row-click="rowClick"
      @row-contextmenu="showMenu"
    >
      <Column header="Person">
        <template #body="{ data: a }: { data: AccountListItem }">
          <div class="person">
            <Avatar :label="initials(a.name)" shape="circle" :class="['avatar', { muted: a.status === 'disabled' }]" />
            <div class="who">
              <RouterLink :to="`/accounts/${a.id}`" class="name">{{ a.name }}</RouterLink>
              <Tag v-if="actions.isSelf(a)" value="You" severity="secondary" class="you" />
              <div class="email">{{ a.email }}</div>
            </div>
          </div>
        </template>
      </Column>
      <Column header="Roles">
        <template #body="{ data: a }: { data: AccountListItem }">
          <div class="chips">
            <Chip v-for="r in a.roles" :key="r.id" :label="r.name" />
            <span v-if="!a.roles.length" class="muted">No roles</span>
          </div>
        </template>
      </Column>
      <Column header="Status">
        <template #body="{ data: a }: { data: AccountListItem }">
          <Tag :value="label(a.status)" :severity="statusSeverity(a.status)" />
          <span v-if="a.status === 'invited' && a.invite_expires_at" class="cell-sub">{{ inviteNote(a.invite_expires_at) }}</span>
        </template>
      </Column>
      <Column header="Last sign-in">
        <template #body="{ data: a }: { data: AccountListItem }">
          <span :class="{ muted: !a.last_sign_in_at }">{{ a.last_sign_in_at ? formatDay(a.last_sign_in_at) : 'Never' }}</span>
        </template>
      </Column>
      <Column header="Created">
        <template #body="{ data: a }: { data: AccountListItem }">{{ formatDay(a.created_at) }}</template>
      </Column>
      <Column v-if="canManage" header="" header-style="width: 6rem">
        <template #body="{ data: a }: { data: AccountListItem }">
          <div class="row-actions">
            <Button
              v-if="a.status === 'invited'"
              v-tooltip.top="'Resend invitation'"
              icon="pi pi-envelope"
              size="small"
              text
              rounded
              aria-label="Resend invitation"
              @click="actions.resendInvitation(a)"
            />
            <Button
              v-if="a.status === 'active'"
              v-tooltip.top="'Send reset link'"
              icon="pi pi-key"
              size="small"
              text
              rounded
              aria-label="Send reset link"
              @click="actions.sendReset(a)"
            />
            <Button
              v-if="a.status === 'disabled'"
              v-tooltip.top="'Enable'"
              icon="pi pi-check-circle"
              size="small"
              text
              rounded
              aria-label="Enable"
              @click="actions.enable(a)"
            />
            <span v-else v-tooltip.top="actions.isSelf(a) ? 'You can’t disable yourself' : 'Disable'">
              <Button
                icon="pi pi-ban"
                size="small"
                text
                rounded
                severity="danger"
                aria-label="Disable"
                :disabled="actions.isSelf(a)"
                @click="actions.disable(a)"
              />
            </span>
          </div>
        </template>
      </Column>
      <template #empty>No accounts match these filters.</template>
    </DataTable>
    <CreateAccountDialog v-model:visible="creating" />
  </section>
</template>

<style scoped>
.sub {
  margin: 0.2rem 0 0;
  color: var(--p-text-muted-color);
}
.seg-count {
  font: 0.75rem var(--app-mono);
  color: var(--p-text-muted-color);
}
.person {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  min-width: 0;
}
.avatar {
  flex: none;
  background: var(--p-highlight-background);
  color: var(--p-highlight-color);
  font-weight: 700;
  font-size: 0.8rem;
}
.avatar.muted {
  background: var(--app-soft);
  color: var(--p-text-muted-color);
}
.who {
  min-width: 0;
}
.name {
  font-weight: 600;
  color: var(--p-text-color);
  text-decoration: none;
}
.you {
  margin-left: 0.4rem;
  font-size: 0.7rem;
}
.email {
  font-size: 0.82rem;
  color: var(--p-text-muted-color);
  overflow: hidden;
  text-overflow: ellipsis;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}
.chips :deep(.p-chip) {
  padding: 0.1rem 0.6rem;
  font-size: 0.8rem;
}
.muted {
  color: var(--p-text-muted-color);
}
.cell-sub {
  display: block;
  margin-top: 0.2rem;
  font-size: 0.78rem;
  color: var(--p-text-muted-color);
}
</style>
