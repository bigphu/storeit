<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import ContextMenu from 'primevue/contextmenu'
import DataTable, { type DataTableRowContextMenuEvent } from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import Menu from 'primevue/menu'
import type { MenuItem } from 'primevue/menuitem'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import { computed, reactive, ref, shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'
import AddCard from '@/components/AddCard.vue'
import CardGrid from '@/components/CardGrid.vue'
import EntityCard from '@/components/EntityCard.vue'
import FormDialog from '@/components/FormDialog.vue'
import IconAction from '@/components/IconAction.vue'
import InlineCell from '@/components/InlineCell.vue'
import OverviewFields, { type FieldDef } from '@/components/OverviewFields.vue'
import QuickEditDrawer from '@/components/QuickEditDrawer.vue'
import PageHeader from '@/components/PageHeader.vue'
import SegmentedFilter, { type SegmentOption } from '@/components/SegmentedFilter.vue'
import type { Role } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { mayClose } from '@/lib/confirm'
import { changesOf, clearTab, emptyDraft, isDirty } from '@/lib/detailDraft'
import { useDirty, useFormErrors } from '@/lib/forms'
import { openLocation } from '@/lib/navigation'
import { useRowMenu } from '@/lib/tableRows'
import { useUrlState } from '@/lib/urlState'
import { useCreateRole, useRoles } from '../api'
import { ALL_PERMS, EMPLOYEE_ROLE_ID, label, moduleOf } from '../catalog'
import { useRoleDelete, useRoleOverviewSave } from '../overviewSave'

// Vai trò: thẻ (đồng hồ quyền, số người) hoặc bảng so sánh mọi role với mọi quyền
const session = useSession()
const router = useRouter()
const canManage = computed(() => session.can(Perm.RoleManage))
const { data: roles } = useRoles()

type Layout = 'cards' | 'compare'
const { state, update } = useUrlState(
  (q) => ({ layout: (q.layout === 'compare' ? 'compare' : 'cards') as Layout }),
  (s) => ({ layout: s.layout === 'compare' ? s.layout : undefined }),
)
const layout = computed({ get: () => state.value.layout, set: (v: Layout) => update({ layout: v }) })
const layoutOptions: SegmentOption<Layout>[] = [
  { label: 'Cards', value: 'cards', icon: 'pi pi-th-large' },
  { label: 'Compare', value: 'compare', icon: 'pi pi-table' },
]

const people = (r: Role) => r.member_count ?? 0
const peopleLabel = (n: number) => `${n} ${n === 1 ? 'person' : 'people'}`


// Bảng so sánh: mỗi dòng một quyền, nhóm theo module; mỗi cột một role
const matrix = computed(() => ALL_PERMS.map((code) => ({ code, label: label(code), module: moduleOf(code) })))

// Role mới: có thể chép quyền của role khác (chỉ role mà mình có đủ quyền)
const creating = ref(false)
const name = ref('')
const description = ref('')
const copyFrom = ref('')
const errors = useFormErrors()
const create = useCreateRole()
const form = useDirty(() => ({ n: name.value.trim(), d: description.value.trim(), c: copyFrom.value }))
const copyOptions = computed(() => [
  { label: 'No permissions', value: '' },
  ...(roles.value ?? [])
    .filter((r) => r.permissions.every((p) => session.can(p)))
    .map((r) => ({ label: `Copy ${r.name}`, value: r.id })),
])
function openCreate() {
  name.value = description.value = ''
  // bắt đầu từ Employee khi được phép chép quyền của nó (ít bấm nhất cho role mới thường gặp)
  copyFrom.value = copyOptions.value.some((o) => o.value === EMPLOYEE_ROLE_ID) ? EMPLOYEE_ROLE_ID : ''
  errors.clear()
  form.reset()
  creating.value = true
}
async function submit() {
  errors.clear()
  const from = roles.value?.find((r) => r.id === copyFrom.value)
  try {
    const role = await create.mutateAsync({ name: name.value, description: description.value, permissions: from?.permissions })
    creating.value = false
    // mở ở Permissions để chỉnh phần vừa chép
    await router.push({ path: `/roles/${role.id}`, query: { tab: 'permissions' } })
  } catch (err) {
    errors.set(err)
  }
}

// Sửa nhanh trên thẻ: bút chì cạnh tên để đổi tại chỗ (role tự tạo); "Quick edit" trong menu
// mở ngăn kéo; xoá role tự tạo không ai giữ
const saveRole = useRoleOverviewSave()
const deleteRole = useRoleDelete()
const rename = (r: Role, name: string) => saveRole(r, { name })
const list = computed(() => roles.value ?? [])
const quick = ref<Role | null>(null)
const quickDraft = reactive(emptyDraft())
const quickSaving = ref(false)
const quickFields = computed<FieldDef[]>(() => [
  { key: 'name', label: 'Name', maxlength: 100, lock: quick.value?.is_system ? 'Built-in roles keep their name.' : undefined },
  { key: 'description', label: 'Description', kind: 'textarea' },
])
const quickSaved = computed(() => ({ name: quick.value?.name ?? '', description: quick.value?.description ?? '' }))
const quickOpen = computed({
  get: () => quick.value !== null,
  set: (v) => {
    if (!v) {
      quick.value = null
      clearTab(quickDraft, 'overview')
    }
  },
})
function openQuick(r: Role) {
  clearTab(quickDraft, 'overview')
  quick.value = r
}
const quickIndex = computed(() => (quick.value ? list.value.findIndex((x) => x.id === quick.value!.id) : -1))
async function moveQuick(step: number) {
  const next = list.value[quickIndex.value + step]
  if (!next || !(await mayClose(isDirty(quickDraft)))) return
  openQuick(next)
}
async function saveQuick() {
  const r = quick.value
  if (!r) return
  quickSaving.value = true
  try {
    if (await saveRole(r, changesOf(quickDraft, 'overview'))) clearTab(quickDraft, 'overview')
  } finally {
    quickSaving.value = false
  }
}
watch(roles, (l) => {
  if (quick.value) quick.value = l?.find((x) => x.id === quick.value!.id) ?? null
})

// Hành động trên thẻ: nút menu (☰) ở đầu thẻ và chuột phải, cùng một danh sách. Xoá chỉ cho
// role tự tạo; còn người giữ thì tắt, nhãn nói lý do
const openRole = (r: Role, newTab?: boolean) => openLocation(router, `/roles/${r.id}`, undefined, newTab)
const roleMenu = (r: Role): MenuItem[] => [
  { label: 'Open', icon: 'pi pi-arrow-right', command: () => openRole(r) },
  { label: 'Open in new tab', icon: 'pi pi-external-link', command: () => openRole(r, true) },
  { separator: true, visible: canManage.value },
  { label: 'Quick edit', icon: 'pi pi-pencil', visible: canManage.value, command: () => openQuick(r) },
  {
    label: people(r) > 0 ? `Delete (held by ${peopleLabel(people(r))})` : 'Delete',
    icon: 'pi pi-trash',
    visible: canManage.value && !r.is_system,
    disabled: people(r) > 0,
    command: () => deleteRole(r),
  },
]
const menu = ref<InstanceType<typeof ContextMenu>>()
const { items: menuItems, show: showMenu, clear: clearMenu } = useRowMenu<Role>(menu, roleMenu)
function onCardMenu(r: Role, e: MouseEvent) {
  e.preventDefault()
  showMenu({ originalEvent: e, data: r, index: 0 } as DataTableRowContextMenuEvent)
}
// không xoá role đang chọn khi menu đóng: Menu báo đóng sau hiệu ứng, lúc đó có thể đã mở
// cho thẻ khác
const cardMenu = ref<InstanceType<typeof Menu>>()
const cardMenuRole = shallowRef<Role | null>(null)
const cardMenuItems = computed(() => (cardMenuRole.value ? roleMenu(cardMenuRole.value) : []))
function toggleCardMenu(r: Role, e: MouseEvent) {
  cardMenuRole.value = r
  cardMenu.value?.toggle(e)
}
</script>

<template>
  <section>
    <PageHeader title="Roles" subtitle="A role is a set of permissions. People get everything their roles allow.">
      <Button v-if="canManage" label="New role" icon="pi pi-plus" @click="openCreate" />
    </PageHeader>
    <div class="toolbar">
      <SegmentedFilter v-model="layout" :options="layoutOptions" label="Layout" />
    </div>

    <ContextMenu ref="menu" :model="menuItems" @hide="clearMenu" />
    <Menu ref="cardMenu" :model="cardMenuItems" popup />

    <CardGrid v-if="state.layout === 'cards'">
      <EntityCard v-for="r in roles ?? []" :key="r.id" :to="`/roles/${r.id}`" :label="r.name" @menu="(e) => onCardMenu(r, e)">
        <div class="top">
          <h3>
            <InlineCell :value="r.name" label="name" :editable="canManage && !r.is_system" @save="(v) => rename(r, v)">{{ r.name }}</InlineCell>
          </h3>
          <i v-if="r.is_system" v-tooltip.top="'Built-in role'" class="pi pi-lock lock" aria-label="Built-in" />
          <IconAction icon="pi pi-bars" :label="`Actions for ${r.name}`" aria-haspopup="menu" @click="(e) => toggleCardMenu(r, e)" />
        </div>
        <p class="desc">{{ r.description || 'No description.' }}</p>
        <div>
          <div class="meter" :aria-label="`${r.permissions.length} of ${ALL_PERMS.length} permissions`">
            <span v-for="p in ALL_PERMS" :key="p" :class="{ on: r.permissions.includes(p) }" :title="label(p)" />
          </div>
          <div class="meta">{{ r.permissions.length }} of {{ ALL_PERMS.length }} permissions</div>
        </div>
        <div class="meta people"><i class="pi pi-users" /> {{ peopleLabel(people(r)) }}</div>
      </EntityCard>
      <AddCard v-if="canManage" label="New role" @click="openCreate" />
    </CardGrid>

    <DataTable
      v-else
      :value="matrix"
      data-key="code"
      row-group-mode="subheader"
      group-rows-by="module"
      row-hover
      scrollable
      class="matrix"
    >
      <template #groupheader="{ data: row }">
        <span class="eyebrow">{{ row.module }}</span>
      </template>
      <Column header="Permission" frozen header-style="min-width: 14rem">
        <template #body="{ data: row }">
          <div class="perm-name">{{ row.label }}</div>
          <code class="perm-code">{{ row.code }}</code>
        </template>
      </Column>
      <Column v-for="r in roles ?? []" :key="r.id" header-class="role-col" body-class="cell-c">
        <template #header>
          <div class="role-head">
            <RouterLink :to="`/roles/${r.id}`">{{ r.name }}</RouterLink>
            <span class="meta">{{ peopleLabel(people(r)) }}</span>
          </div>
        </template>
        <template #body="{ data: row }">
          <i v-if="r.permissions.includes(row.code)" class="pi pi-check yes" aria-label="Yes" />
          <span v-else class="no" aria-label="No">—</span>
        </template>
      </Column>
    </DataTable>

    <QuickEditDrawer
      v-if="quick"
      v-model:visible="quickOpen"
      :title="quick.name"
      icon="shield"
      :dirty="isDirty(quickDraft)"
      :busy="quickSaving"
      :can-prev="quickIndex > 0"
      :can-next="quickIndex >= 0 && quickIndex < list.length - 1"
      @save="saveQuick"
      @prev="moveQuick(-1)"
      @next="moveQuick(1)"
      @open-page="router.push(`/roles/${quick.id}`)"
    >
      <OverviewFields :fields="quickFields" :saved="quickSaved" :draft="quickDraft" stacked />
    </QuickEditDrawer>

    <FormDialog
      v-model:visible="creating"
      icon="shield"
      title="New role"
      action="Create role"
      :busy="create.isPending.value"
      :error="errors.general.value"
      :dirty="form.dirty.value"
      @submit="submit"
    >
      <div class="field">
        <label for="role-name">Name</label>
        <InputText id="role-name" v-model="name" required maxlength="100" autofocus />
        <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
      </div>
      <div class="field">
        <label for="role-desc">Description</label>
        <Textarea id="role-desc" v-model="description" rows="3" />
      </div>
      <div class="field">
        <label for="role-copy">Start from</label>
        <Select v-model="copyFrom" input-id="role-copy" :options="copyOptions" option-label="label" option-value="value" />
        <small>You can change the permissions on the next page.</small>
      </div>
    </FormDialog>
  </section>
</template>

<style scoped>
.top {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.top h3 {
  flex: 1;
  font-size: 1rem;
}
.lock {
  color: var(--p-text-muted-color);
  font-size: 0.8rem;
}
.desc {
  margin: 0;
  min-height: 2.6em;
  color: var(--p-text-muted-color);
  font-size: 0.875rem;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
/* đồng hồ quyền: một ô cho mỗi quyền, ô có màu là role có quyền đó */
.meter {
  display: flex;
  gap: 3px;
}
.meter span {
  flex: 1;
  height: 6px;
  border-radius: 3px;
  background: var(--app-soft);
}
.meter span.on {
  background: var(--app-accent);
}
.meta {
  margin-top: 0.3rem;
  font-size: 0.8rem;
  color: var(--p-text-muted-color);
}
.people i {
  font-size: 0.8rem;
  margin-right: 0.2rem;
}
.matrix :deep(.role-col) {
  text-align: center;
}
.matrix :deep(.role-col .p-datatable-column-header-content) {
  justify-content: center;
}
.matrix :deep(td.cell-c) {
  text-align: center;
}
.role-head {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.1rem;
  white-space: nowrap;
}
.role-head a {
  font-weight: 600;
  text-decoration: none;
}
.role-head .meta {
  margin: 0;
  font-weight: 400;
}
.perm-name {
  font-weight: 600;
}
.perm-code {
  font-size: 0.75rem;
  color: var(--p-text-muted-color);
}
.yes {
  color: var(--app-accent);
}
.no {
  color: var(--p-text-muted-color);
  opacity: 0.5;
}
</style>
