<script setup lang="ts">
import Avatar from 'primevue/avatar'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Chip from 'primevue/chip'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Select from 'primevue/select'
import Tab from 'primevue/tab'
import TabList from 'primevue/tablist'
import Tabs from 'primevue/tabs'
import Tag from 'primevue/tag'
import Textarea from 'primevue/textarea'
import { useConfirm } from 'primevue/useconfirm'
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useTabDirty, useTabTitle } from '@/app/tabs/tabPage'
import AppBreadcrumb from '@/components/AppBreadcrumb.vue'
import { useAccounts, useAssignRoles } from '@/features/accounts/api'
import { statusSeverity } from '@/features/accounts/status'
import type { AccountListItem } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { initials } from '@/lib/people'
import { onRowClick } from '@/lib/tableRows'
import { openLocation } from '@/lib/navigation'
import { useUrlState } from '@/lib/urlState'
import { useDeleteRole, useRole, useSetRolePermissions, useUpdateRole } from '../api'
import { ADMINISTRATOR_ROLE_ID, ALL_PERMS, AREAS, type Area, label, toggle } from '../catalog'

// Trang vai trò: thẻ đầu trang, rồi Permissions (ma trận khu vực × Xem / Quản lý) | People
const props = defineProps<{ id: string }>()

const session = useSession()
const router = useRouter()
const confirm = useConfirm()
const canManage = computed(() => session.can(Perm.RoleManage))
const canSeePeople = computed(() => session.can(Perm.AccountRead))
const canAssign = computed(() => session.can(Perm.AccountManage))

const { data: role } = useRole(() => props.id)
useTabTitle(() => role.value?.name)

type Section = 'permissions' | 'people'
const { state, update } = useUrlState(
  (q) => ({ tab: (q.tab === 'people' ? 'people' : 'permissions') as Section }),
  (s) => ({ tab: s.tab === 'people' ? s.tab : undefined }),
)

// Quyền: bản nháp so với bản đã lưu; bật Quản lý kéo theo Xem (catalog.toggle)
const saved = computed(() => role.value?.permissions ?? [])
const draft = ref<string[] | null>(null)
const current = computed(() => draft.value ?? saved.value)
watch(saved, () => (draft.value = null))
const changed = computed(() => ALL_PERMS.filter((p) => current.value.includes(p) !== saved.value.includes(p)))
useTabDirty(() => changed.value.length > 0)
function set(code: string, on: boolean) {
  draft.value = toggle(current.value, code, on)
}
// Chỉ đổi được quyền mình có (API trả 403 cho phần vượt quyền)
const editable = (code: string) => canManage.value && session.can(code)
const rowChanged = (a: Area) => [a.view, a.manage].some((p) => p && changed.value.includes(p))
// Administrator phải giữ "Manage roles", không thì không ai đổi được role nữa (API trả 409)
const lockout = computed(() => props.id === ADMINISTRATOR_ROLE_ID && !current.value.includes(Perm.RoleManage))
const people = computed(() => role.value?.member_count ?? 0)
const peopleLabel = (n: number) => `${n} ${n === 1 ? 'person' : 'people'}`

const setPerms = useSetRolePermissions()
async function savePermissions() {
  try {
    await setPerms.mutateAsync({ id: props.id, permissions: current.value })
    draft.value = null
    notify.success('Permissions saved.')
  } catch {
    // lỗi đã hiện qua toast của mutation
  }
}

// Sửa tên, mô tả (role hệ thống giữ tên); xoá role tự tạo
const editing = ref(false)
const name = ref('')
const description = ref('')
const errors = useFormErrors()
const updateRole = useUpdateRole()
function openEdit() {
  name.value = role.value?.name ?? ''
  description.value = role.value?.description ?? ''
  errors.clear()
  editing.value = true
}
async function saveDetails() {
  errors.clear()
  try {
    const body = role.value?.is_system ? { description: description.value } : { name: name.value, description: description.value }
    await updateRole.mutateAsync({ id: props.id, ...body })
    editing.value = false
    notify.success('Role saved.')
  } catch (err) {
    errors.set(err)
  }
}
const remove = useDeleteRole()
function askDelete() {
  // role còn người giữ thì API trả 409: nói trước thay vì hỏi rồi báo lỗi
  if (people.value) {
    notify.info(`${role.value?.name} is still held by ${peopleLabel(people.value)}. Remove them from the role first.`)
    return
  }
  confirm.require({
    header: 'Delete role',
    message: `Delete the role ${role.value?.name}? This can't be undone.`,
    acceptLabel: 'Delete',
    rejectLabel: 'Cancel',
    acceptProps: { severity: 'danger' },
    accept: () =>
      remove
        .mutateAsync(props.id)
        .then(() => {
          notify.success('Role deleted.')
          return router.push('/roles')
        })
        .catch(() => {}),
  })
}

// People: tài khoản giữ role (GET /accounts?role_id=), tải khi mở tab
const showPeople = computed(() => state.value.tab === 'people' && canSeePeople.value)
const { data: members, isFetching: membersLoading } = useAccounts(
  () => ({ role_id: props.id, page: 1, page_size: 200 }),
  showPeople,
)
const isSelf = (a: AccountListItem) => a.id === session.me?.account.id
const assign = useAssignRoles()
function askRemove(a: AccountListItem) {
  confirm.require({
    header: 'Remove from role',
    message: `Remove ${a.name} from ${role.value?.name}? They lose what only this role gave them.`,
    acceptLabel: 'Remove',
    rejectLabel: 'Cancel',
    acceptProps: { severity: 'danger' },
    accept: () =>
      assign
        .mutateAsync({ id: a.id, roleIds: a.roles.map((r) => r.id).filter((id) => id !== props.id) })
        .then(() => notify.success(`${a.name} removed from ${role.value?.name}.`))
        .catch(() => {}),
  })
}
const openPerson = onRowClick((a: AccountListItem, e: MouseEvent) => openLocation(router, `/accounts/${a.id}`, e))

// Thêm người: chọn trong các tài khoản chưa có role này
const adding = ref(false)
const addId = ref('')
const { data: everyone } = useAccounts(() => ({ page: 1, page_size: 200 }), adding)
const candidates = computed(() =>
  (everyone.value?.items ?? []).filter((a) => a.status !== 'disabled' && !a.roles.some((r) => r.id === props.id)),
)
async function addPerson() {
  const a = candidates.value.find((x) => x.id === addId.value)
  if (!a) return
  try {
    await assign.mutateAsync({ id: a.id, roleIds: [...a.roles.map((r) => r.id), props.id] })
    notify.success(`${a.name} now has ${role.value?.name}.`)
    adding.value = false
    addId.value = ''
  } catch {
    // lỗi đã hiện qua toast của mutation
  }
}

const crumbs = computed(() => [{ label: 'Roles', to: '/roles' }, { label: role.value?.name ?? '…' }])
</script>

<template>
  <section v-if="role">
    <AppBreadcrumb :items="crumbs" />

    <header class="head-card">
      <span class="mark"><i class="pi pi-shield" /></span>
      <div class="who">
        <div class="line">
          <h1>{{ role.name }}</h1>
          <Tag v-if="role.is_system" value="Built-in" icon="pi pi-lock" severity="secondary" />
          <Tag v-else value="Custom" severity="secondary" />
        </div>
        <div class="muted">{{ role.description || 'No description.' }}</div>
        <div class="meta">{{ role.permissions.length }} of {{ ALL_PERMS.length }} permissions · {{ peopleLabel(people) }}</div>
      </div>
      <div v-if="canManage" class="actions">
        <Button label="Edit details" icon="pi pi-pencil" severity="secondary" outlined @click="openEdit" />
        <Button v-if="!role.is_system" label="Delete" icon="pi pi-trash" severity="danger" outlined @click="askDelete" />
      </div>
    </header>

    <Tabs :value="state.tab" class="section-tabs" @update:value="(v) => update({ tab: v as Section })">
      <TabList>
        <Tab value="permissions">Permissions</Tab>
        <Tab v-if="canSeePeople" value="people">People <span class="tab-count">{{ people }}</span></Tab>
      </TabList>
    </Tabs>

    <template v-if="state.tab === 'permissions'">
      <Message v-if="lockout" severity="warn" :closable="false" class="block-msg">
        Administrator must keep “Manage roles”, or nobody could change roles again.
      </Message>
      <DataTable :value="AREAS" data-key="label" row-group-mode="subheader" group-rows-by="module" :row-class="(a: Area) => (rowChanged(a) ? 'changed' : undefined)">
        <template #groupheader="{ data: a }">
          <span class="eyebrow">{{ a.module }}</span>
        </template>
        <Column header="Area">
          <template #body="{ data: a }: { data: Area }">
            <div class="area">{{ a.label }}</div>
            <div class="hint">{{ a.hint }}</div>
          </template>
        </Column>
        <Column header="View" header-class="center" body-class="center" header-style="width: 9rem">
          <template #body="{ data: a }: { data: Area }">
            <Checkbox
              v-if="a.view"
              v-tooltip.top="label(a.view)"
              :model-value="current.includes(a.view)"
              binary
              :disabled="!editable(a.view)"
              :aria-label="`View ${a.label}`"
              @update:model-value="(v: boolean) => set(a.view!, v)"
            />
            <span v-else class="via">
              <i :class="current.includes(a.viaView!) ? 'pi pi-check' : 'pi pi-minus'" /> via Assets
            </span>
          </template>
        </Column>
        <Column header="Manage" header-class="center" body-class="center" header-style="width: 9rem">
          <template #body="{ data: a }: { data: Area }">
            <Checkbox
              v-tooltip.top="label(a.manage)"
              :model-value="current.includes(a.manage)"
              binary
              :disabled="!editable(a.manage)"
              :aria-label="`Manage ${a.label}`"
              @update:model-value="(v: boolean) => set(a.manage, v)"
            />
          </template>
        </Column>
      </DataTable>
      <p class="hint after">Hover a box for the exact permission. Ticking Manage also ticks View. You can only change permissions you hold.</p>
      <div v-if="changed.length" class="savebar" role="region" aria-label="Unsaved permissions">
        <span class="dot" />
        <span class="grow">
          {{ changed.length }} permission{{ changed.length === 1 ? '' : 's' }} changed · affects {{ peopleLabel(people) }} within 15 minutes
        </span>
        <Button label="Discard" text size="small" class="on-dark" @click="draft = null" />
        <Button label="Save permissions" size="small" :loading="setPerms.isPending.value" :disabled="lockout" @click="savePermissions" />
      </div>
    </template>

    <template v-else-if="canSeePeople">
      <div class="toolbar">
        <span class="muted">{{ peopleLabel(people) }} {{ people === 1 ? 'has' : 'have' }} this role (disabled accounts not counted).</span>
        <Button v-if="canAssign" label="Add people" icon="pi pi-plus" size="small" severity="secondary" outlined class="end" @click="adding = true" />
      </div>
      <DataTable
        :value="members?.items ?? []"
        :loading="membersLoading"
        data-key="id"
        row-hover
        :row-class="() => 'clickable-row'"
        @row-click="openPerson"
      >
        <Column header="Person">
          <template #body="{ data: a }: { data: AccountListItem }">
            <div class="person">
              <Avatar :label="initials(a.name)" shape="circle" :class="['avatar', { off: a.status === 'disabled' }]" />
              <div>
                <RouterLink :to="`/accounts/${a.id}`" class="name">{{ a.name }}</RouterLink>
                <div class="muted small">{{ a.email }}</div>
              </div>
            </div>
          </template>
        </Column>
        <Column header="Status">
          <template #body="{ data: a }: { data: AccountListItem }">
            <Tag :value="a.status[0].toUpperCase() + a.status.slice(1)" :severity="statusSeverity(a.status)" />
          </template>
        </Column>
        <Column header="Other roles">
          <template #body="{ data: a }: { data: AccountListItem }">
            <div class="chips">
              <Chip v-for="r in a.roles.filter((x) => x.id !== role!.id)" :key="r.id" :label="r.name" />
              <span v-if="a.roles.length <= 1" class="muted">None</span>
            </div>
          </template>
        </Column>
        <Column v-if="canAssign" header="" header-style="width: 4rem">
          <template #body="{ data: a }: { data: AccountListItem }">
            <div class="row-actions">
              <span v-tooltip.top="isSelf(a) && role!.id === ADMINISTRATOR_ROLE_ID ? 'You can’t remove your own Administrator role' : 'Remove from role'">
                <Button
                  icon="pi pi-times"
                  size="small"
                  text
                  rounded
                  severity="danger"
                  aria-label="Remove from role"
                  :disabled="isSelf(a) && role!.id === ADMINISTRATOR_ROLE_ID"
                  @click="askRemove(a)"
                />
              </span>
            </div>
          </template>
        </Column>
        <template #empty>Nobody has this role yet.</template>
      </DataTable>
    </template>

    <Dialog v-model:visible="editing" modal header="Edit role" :style="{ width: '32rem' }">
      <form class="form" @submit.prevent="saveDetails">
        <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
        <div class="field">
          <label for="role-name">Name</label>
          <InputText id="role-name" v-model="name" :disabled="role.is_system" required maxlength="100" />
          <small v-if="role.is_system">Built-in roles keep their name.</small>
          <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
        </div>
        <div class="field">
          <label for="role-desc">Description</label>
          <Textarea id="role-desc" v-model="description" rows="3" />
        </div>
        <div class="actions">
          <Button type="submit" label="Save" :loading="updateRole.isPending.value" />
          <Button label="Cancel" severity="secondary" text @click="editing = false" />
        </div>
      </form>
    </Dialog>

    <Dialog v-model:visible="adding" modal :header="`Add people to ${role.name}`" :style="{ width: '30rem' }">
      <form class="form" @submit.prevent="addPerson">
        <div class="field">
          <label for="add-person">Account</label>
          <Select
            v-model="addId"
            input-id="add-person"
            :options="candidates"
            option-label="name"
            option-value="id"
            filter
            :filter-fields="['name', 'email']"
            placeholder="Choose someone"
            empty-message="Everyone already has this role."
          >
            <template #option="{ option }">
              <div>
                <div>{{ option.name }}</div>
                <small class="muted">{{ option.email }}</small>
              </div>
            </template>
          </Select>
        </div>
        <div class="actions">
          <Button type="submit" label="Add" :disabled="!addId" :loading="assign.isPending.value" />
          <Button label="Cancel" severity="secondary" text @click="adding = false" />
        </div>
      </form>
    </Dialog>
  </section>
</template>

<style scoped>
.head-card {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 1rem;
  padding: 1rem 1.1rem;
  margin-bottom: 1rem;
  border: 1px solid var(--app-line);
  border-radius: 12px;
  background: var(--p-content-background);
}
.mark {
  flex: none;
  display: grid;
  place-items: center;
  width: 3rem;
  height: 3rem;
  border-radius: 10px;
  background: var(--app-soft);
  color: var(--app-accent);
  font-size: 1.2rem;
}
.who {
  flex: 1;
  min-width: 12rem;
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}
.line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}
.muted,
.meta,
.hint {
  color: var(--p-text-muted-color);
}
.meta,
.small {
  font-size: 0.85rem;
}
.section-tabs {
  margin-bottom: 1rem;
}
.tab-count {
  font: 0.75rem var(--app-mono);
  color: var(--p-text-muted-color);
}
.block-msg {
  margin-bottom: 0.75rem;
}
.area {
  font-weight: 600;
}
.hint {
  font-size: 0.8rem;
}
.hint.after {
  margin: 0.6rem 0 0;
}
:deep(.center) {
  text-align: center;
}
:deep(.center .p-datatable-column-header-content) {
  justify-content: center;
}
.via {
  font-size: 0.8rem;
  color: var(--p-text-muted-color);
}
.via i {
  font-size: 0.75rem;
}
/* dòng có thay đổi chưa lưu: vạch nhấn bên trái */
:deep(tr.changed > td:first-child) {
  box-shadow: var(--app-bar-left);
}
.end {
  margin-left: auto;
}
.person {
  display: flex;
  align-items: center;
  gap: 0.65rem;
}
.avatar {
  flex: none;
  background: var(--p-highlight-background);
  color: var(--p-highlight-color);
  font-weight: 700;
  font-size: 0.8rem;
}
.avatar.off {
  background: var(--app-soft);
  color: var(--p-text-muted-color);
}
.name {
  font-weight: 600;
  color: var(--p-text-color);
  text-decoration: none;
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
.savebar {
  position: sticky;
  bottom: 0.75rem;
  margin-top: 1rem;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.6rem;
  padding: 0.6rem 0.75rem 0.6rem 1rem;
  border-radius: 10px;
  /* màu đảo theo theme: nổi trên nền sáng lẫn tối */
  background: var(--p-text-color);
  color: var(--p-content-background);
  box-shadow: 0 6px 20px rgb(0 0 0 / 0.18);
}
.savebar .grow {
  flex: 1;
  min-width: 8rem;
}
.savebar .dot {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 50%;
  background: var(--app-accent);
}
.savebar .on-dark {
  color: var(--p-content-background);
}
</style>
