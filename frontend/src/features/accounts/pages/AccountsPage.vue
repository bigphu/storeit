<script setup lang="ts">
import Button from 'primevue/button'
import Chip from 'primevue/chip'
import Checkbox from 'primevue/checkbox'
import Column from 'primevue/column'
import DataTable, { type DataTablePageEvent } from 'primevue/datatable'
import IconField from 'primevue/iconfield'
import InputIcon from 'primevue/inputicon'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Tag from 'primevue/tag'
import type { MenuItem } from 'primevue/menuitem'
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import EmptyState from '@/components/EmptyState.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import RowMenuButton from '@/components/RowMenuButton.vue'
import RowMenus from '@/components/RowMenus.vue'
import PageHeader from '@/components/PageHeader.vue'
import InlineCell from '@/components/InlineCell.vue'
import OverviewFields, { type FieldDef } from '@/components/OverviewFields.vue'
import PersonCell from '@/components/PersonCell.vue'
import YouTag from '@/components/YouTag.vue'
import QuickEditDrawer from '@/components/QuickEditDrawer.vue'
import SegmentedFilter, { type SegmentOption } from '@/components/SegmentedFilter.vue'
import { useRoles } from '@/features/roles/api'
import type { AccountListItem } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { mayClose } from '@/lib/confirm'
import { useQuickDrawer } from '@/lib/quickDrawer'
import { formatDay } from '@/lib/dates'
import { changeCount, changesOf, clearTab, emptyDraft, isDirty, listOf, setList, pruneList } from '@/lib/detailDraft'
import { openLocation } from '@/lib/navigation'
import { inviteNote } from '@/lib/people'
import { PAGE_SIZES, usePageSize } from '@/lib/preferences'
import { useListTable, useRowMenu } from '@/lib/tableRows'
import { queryInt, queryString, useUrlState } from '@/lib/urlState'
import { type AccountStatus, useAccounts } from '../api'
import CreateAccountDialog from '../components/CreateAccountDialog.vue'
import { useAccountOverviewSave, useAccountRolesSave } from '../overviewSave'
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
const { data, isFetching, isLoading } = useAccounts(params)

// Ô tìm kiếm: đợi gõ xong rồi mới đổi URL
const search = ref(state.value.q)
let timer: ReturnType<typeof setTimeout> | undefined
watch(search, (q) => {
  clearTimeout(timer)
  timer = setTimeout(() => update({ q, page: 1 }), 300)
})

// Nút lọc trạng thái có số đếm (cùng tìm kiếm và role)
const status = computed({ get: () => state.value.status, set: (v: Filter) => update({ status: v, page: 1 }) })
const statusOptions = computed<SegmentOption<Filter>[]>(() => {
  const c = data.value?.status_counts
  const n = (s: AccountStatus) => c?.[s] ?? 0
  return [
    { label: 'All', value: 'all', count: c ? n('active') + n('invited') + n('disabled') : undefined },
    { label: 'Active', value: 'active', count: c ? n('active') : undefined },
    { label: 'Invited', value: 'invited', count: c ? n('invited') : undefined },
    { label: 'Disabled', value: 'disabled', count: c ? n('disabled') : undefined },
  ]
})
const roleOptions = computed(() => [{ name: 'All roles', id: 'all' }, ...(roles.data.value ?? [])])

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
// menu của dòng: chuột phải và nút ☰ cuối dòng
const rowMenu = useRowMenu<AccountListItem>((a) => {
  const items: MenuItem[] = [
    { label: 'Open', icon: 'pi pi-arrow-right', command: () => openAccount(a) },
    { label: 'Open in new tab', icon: 'pi pi-external-link', command: () => openAccount(a, undefined, true) },
  ]
  if (!canManage.value) return items
  items.push({ separator: true })
  items.push({ label: 'Quick edit', icon: 'pi pi-sliders-h', command: () => openQuick(a) })
  if (a.status === 'invited') items.push({ label: 'Resend invitation', icon: 'pi pi-envelope', command: () => actions.resendInvitation(a) })
  if (a.status === 'active') items.push({ label: 'Send reset link', icon: 'pi pi-key', command: () => actions.sendReset(a) })
  if (a.status === 'disabled') items.push({ label: 'Enable', icon: 'pi pi-check-circle', command: () => actions.enable(a) })
  else items.push({ label: 'Disable', icon: 'pi pi-ban', disabled: actions.isSelf(a), command: () => actions.disable(a) })
  return items
})
// bảng: bấm dòng mở tài khoản, menu chuột phải, nút cuối dòng cho hàng đang dùng
const table = useListTable({ open: openAccount, showMenu: rowMenu.showContext })

// Sửa nhanh: bút chì cạnh tên để đổi tại chỗ; nút thanh trượt (pi-sliders-h) mở ngăn kéo (tên, role)
const saveAccount = useAccountOverviewSave()
const saveRoles = useAccountRolesSave()
const rename = (a: AccountListItem, name: string) => saveAccount(a, { name })
const rows = computed(() => data.value?.items ?? [])
const quick = ref<AccountListItem | null>(null)
const quickDraft = reactive(emptyDraft())
const quickSaving = ref(false)
const quickFields: FieldDef[] = [
  { key: 'name', label: 'Display name', maxlength: 200 },
  { key: 'email', label: 'Email', lock: 'The sign-in address. Invite a new account to use another one.' },
]
const quickSaved = computed(() => ({ name: quick.value?.name ?? '', email: quick.value?.email ?? '' }))
const quickRoles = computed(() => quick.value?.roles.map((r) => r.id) ?? [])
watch(quickRoles, (s) => pruneList(quickDraft, 'roles', s))
const allRoleIds = computed(() => (roles.data.value ?? []).map((r) => r.id))
const quickCurrent = computed(() => listOf(quickDraft, 'roles', quickRoles.value, allRoleIds.value))
const grantable = (permissions: string[]) => permissions.every((p) => session.can(p))
function pickRole(id: string, on: boolean) {
  const next = on ? [...quickCurrent.value, id] : quickCurrent.value.filter((x) => x !== id)
  setList(quickDraft, 'roles', quickRoles.value, next)
}
function clearQuick() {
  clearTab(quickDraft, 'overview')
  clearTab(quickDraft, 'roles')
}
// mở/đóng ngăn kéo: mục giữ lại đến khi trượt ra xong rồi mới xoá cùng bản nháp
const quickDrawer = useQuickDrawer(quick, clearQuick)
const quickOpen = quickDrawer.visible
function openQuick(a: AccountListItem) {
  quickDrawer.open(a)
}
const quickIndex = computed(() => (quick.value ? rows.value.findIndex((x) => x.id === quick.value!.id) : -1))
async function moveQuick(step: number) {
  const next = rows.value[quickIndex.value + step]
  if (!next || !(await mayClose(isDirty(quickDraft)))) return
  openQuick(next)
}
async function saveQuick() {
  const a = quick.value
  if (!a) return
  quickSaving.value = true
  try {
    if (changeCount(quickDraft, 'overview') && (await saveAccount(a, changesOf(quickDraft, 'overview')))) clearTab(quickDraft, 'overview')
    if (changeCount(quickDraft, 'roles') && (await saveRoles(a, quickRoles.value, quickCurrent.value))) clearTab(quickDraft, 'roles')
  } finally {
    quickSaving.value = false
  }
}
watch(rows, (list) => {
  if (quick.value) quick.value = list.find((x) => x.id === quick.value!.id) ?? null
})
</script>

<template>
  <section>
    <PageHeader title="Accounts" subtitle="People who can sign in to StoreIt and the roles they hold.">
      <Button v-if="canManage" label="Invite account" icon="pi pi-envelope" @click="creating = true" />
    </PageHeader>
    <div class="toolbar">
      <SegmentedFilter v-model="status" :options="statusOptions" label="Status" />
      <IconField>
        <InputIcon class="pi pi-search" />
        <InputText v-model="search" placeholder="Search name or email" aria-label="Search accounts" />
      </IconField>
      <Select
        :model-value="state.role || 'all'"
        :options="roleOptions"
        option-label="name"
        option-value="id"
        aria-label="Role"
        @update:model-value="(v: string) => update({ role: v === 'all' ? '' : v, page: 1 })"
      />
    </div>
    <RowMenus :menu="rowMenu" />
    <DataTable
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
      v-bind="table.bind"
      @page="onPage"
    >
      <Column header="Person">
        <template #body="{ data: a }: { data: AccountListItem }">
          <PersonCell :name="a.name" :email="a.email" :to="`/accounts/${a.id}`" :muted="a.status === 'disabled'">
            <!-- tên và nhãn "You" trong ô sửa tại chỗ: bút chì ở cuối cột -->
            <template #name>
              <InlineCell :value="a.name" label="name" :editable="canManage" @save="(v) => rename(a, v)">
                <span class="person-name">{{ a.name }}</span><YouTag v-if="actions.isSelf(a)" />
              </InlineCell>
            </template>
          </PersonCell>
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
      <!-- nút ☰ luôn ở mép phải của bảng, kể cả khi bảng cuộn ngang -->
      <Column frozen align-frozen="right" header-class="row-menu-col" body-class="row-menu-col">
        <template #body="{ data: a, index }: { data: AccountListItem; index: number }">
          <RowMenuButton :active="table.active.isActive(index)" @open="(e) => rowMenu.toggle(a, e)" />
        </template>
      </Column>
      <template #empty>
        <TableSkeleton v-if="isLoading" :rows="12" />
        <EmptyState v-else icon="pi pi-users" text="No accounts match these filters." />
      </template>
    </DataTable>
    <CreateAccountDialog v-model:visible="creating" @created="(a) => router.push(`/accounts/${a.id}`)" />
    <QuickEditDrawer
      v-if="quick"
      v-model:visible="quickOpen"
      @closed="quickDrawer.closed()"
      :title="quick.name"
      icon="user-plus"
      :dirty="isDirty(quickDraft)"
      :busy="quickSaving"
      :can-prev="quickIndex > 0"
      :can-next="quickIndex >= 0 && quickIndex < rows.length - 1"
      actions-label="Account"
      @save="saveQuick"
      @prev="moveQuick(-1)"
      @next="moveQuick(1)"
      @open-page="router.push(`/accounts/${quick.id}`)"
    >
      <OverviewFields :fields="quickFields" :saved="quickSaved" :draft="quickDraft" stacked />
      <fieldset class="quick-roles">
        <legend>Roles</legend>
        <label v-for="r in roles.data.value ?? []" :key="r.id" class="quick-role" :class="{ off: !grantable(r.permissions) }">
          <Checkbox
            :model-value="quickCurrent.includes(r.id)"
            binary
            :input-id="`qr-${r.id}`"
            :disabled="!grantable(r.permissions)"
            @update:model-value="(v: boolean) => pickRole(r.id, v)"
          />
          <span>{{ r.name }}</span>
        </label>
      </fieldset>
      <!-- hành động trên tài khoản, như nút cuối dòng; đều hỏi trước -->
      <template #actions>
        <Button
          v-if="quick.status === 'invited'"
          label="Resend invitation"
          icon="pi pi-envelope"
          severity="secondary"
          outlined
          size="small"
          @click="actions.resendInvitation(quick)"
        />
        <Button
          v-if="quick.status === 'active'"
          label="Send reset link"
          icon="pi pi-key"
          severity="secondary"
          outlined
          size="small"
          @click="actions.sendReset(quick)"
        />
        <Button
          v-if="quick.status === 'disabled'"
          label="Enable"
          icon="pi pi-check-circle"
          severity="secondary"
          outlined
          size="small"
          @click="actions.enable(quick)"
        />
        <span v-else v-tooltip.top="actions.isSelf(quick) ? 'You can’t disable yourself' : undefined">
          <Button
            label="Disable"
            icon="pi pi-ban"
            severity="secondary"
            outlined
            size="small"
            :disabled="actions.isSelf(quick)"
            @click="actions.disable(quick)"
          />
        </span>
      </template>
    </QuickEditDrawer>
  </section>
</template>

<style scoped>
.person-name {
  font-weight: 600;
  color: var(--p-text-color);
}
.quick-roles {
  border: 0;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}
.quick-roles legend {
  font-size: 0.85rem;
  color: var(--p-text-muted-color);
  margin-bottom: 0.3rem;
}
.quick-role {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}
.quick-role.off {
  opacity: 0.55;
}
/* role trên một dòng như mọi ô của bảng; nhiều role thì bảng cuộn ngang */
.chips {
  display: flex;
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
