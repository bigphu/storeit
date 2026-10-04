<script setup lang="ts">
import Button from 'primevue/button'
import Card from 'primevue/card'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Select from 'primevue/select'
import SelectButton from 'primevue/selectbutton'
import Tag from 'primevue/tag'
import Textarea from 'primevue/textarea'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { Role } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { useFormErrors } from '@/lib/forms'
import { openLocation, wantsNewTab } from '@/lib/navigation'
import { isRowControl } from '@/lib/tableRows'
import { useUrlState } from '@/lib/urlState'
import { useCreateRole, useRoles } from '../api'
import { ALL_PERMS, label, moduleOf } from '../catalog'

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
const layoutOptions = [
  { label: 'Cards', value: 'cards', icon: 'pi pi-th-large' },
  { label: 'Compare', value: 'compare', icon: 'pi pi-table' },
]

const people = (r: Role) => r.member_count ?? 0
const peopleLabel = (n: number) => `${n} ${n === 1 ? 'person' : 'people'}`

function openRole(r: Role, e?: MouseEvent) {
  openLocation(router, `/roles/${r.id}`, e)
}
function onCardClick(r: Role, e: MouseEvent) {
  if (!isRowControl(e.target)) openRole(r, e)
}
function onCardAux(r: Role, e: MouseEvent) {
  if (wantsNewTab(e)) openRole(r, e)
}

// Bảng so sánh: mỗi dòng một quyền, nhóm theo module; mỗi cột một role
const matrix = computed(() => ALL_PERMS.map((code) => ({ code, label: label(code), module: moduleOf(code) })))

// Role mới: có thể chép quyền của role khác (chỉ role mà mình có đủ quyền)
const creating = ref(false)
const name = ref('')
const description = ref('')
const copyFrom = ref('')
const errors = useFormErrors()
const create = useCreateRole()
const copyOptions = computed(() => [
  { label: 'No permissions', value: '' },
  ...(roles.value ?? [])
    .filter((r) => r.permissions.every((p) => session.can(p)))
    .map((r) => ({ label: `Copy ${r.name}`, value: r.id })),
])
function openCreate() {
  name.value = description.value = copyFrom.value = ''
  errors.clear()
  creating.value = true
}
async function submit() {
  errors.clear()
  const from = roles.value?.find((r) => r.id === copyFrom.value)
  try {
    const role = await create.mutateAsync({ name: name.value, description: description.value, permissions: from?.permissions })
    creating.value = false
    await router.push(`/roles/${role.id}`)
  } catch (err) {
    errors.set(err)
  }
}
</script>

<template>
  <section>
    <div class="page-header">
      <div>
        <h1>Roles</h1>
        <p class="sub">A role is a set of permissions. People get everything their roles allow.</p>
      </div>
      <Button v-if="canManage" label="New role" icon="pi pi-plus" @click="openCreate" />
    </div>
    <div class="toolbar">
      <SelectButton
        :model-value="state.layout"
        :options="layoutOptions"
        option-value="value"
        :allow-empty="false"
        aria-label="Layout"
        @update:model-value="(v: Layout) => update({ layout: v })"
      >
        <template #option="{ option }">
          <i :class="option.icon" />
          <span>{{ option.label }}</span>
        </template>
      </SelectButton>
    </div>

    <div v-if="state.layout === 'cards'" class="role-grid">
      <Card
        v-for="r in roles ?? []"
        :key="r.id"
        class="role-card"
        tabindex="0"
        role="link"
        :aria-label="r.name"
        @click="(e: MouseEvent) => onCardClick(r, e)"
        @auxclick="(e: MouseEvent) => onCardAux(r, e)"
        @mousedown.middle.prevent
        @keydown.enter.self="openRole(r)"
      >
        <template #content>
          <div class="card-body">
            <div class="top">
              <h3>{{ r.name }}</h3>
              <i v-if="r.is_system" v-tooltip.top="'Built-in role'" class="pi pi-lock lock" aria-label="Built-in" />
              <Tag v-else value="Custom" severity="secondary" />
            </div>
            <p class="desc">{{ r.description || 'No description.' }}</p>
            <div>
              <div class="meter" :aria-label="`${r.permissions.length} of ${ALL_PERMS.length} permissions`">
                <span v-for="p in ALL_PERMS" :key="p" :class="{ on: r.permissions.includes(p) }" :title="label(p)" />
              </div>
              <div class="meta">{{ r.permissions.length }} of {{ ALL_PERMS.length }} permissions</div>
            </div>
            <div class="meta people"><i class="pi pi-users" /> {{ peopleLabel(people(r)) }}</div>
          </div>
        </template>
      </Card>
      <button v-if="canManage" type="button" class="new-card" @click="openCreate">
        <i class="pi pi-plus" />
        <span>New role</span>
      </button>
    </div>

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

    <Dialog v-model:visible="creating" modal header="New role" :style="{ width: '32rem' }">
      <form class="form" @submit.prevent="submit">
        <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
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
        <div class="actions">
          <Button type="submit" label="Create role" :loading="create.isPending.value" />
          <Button label="Cancel" severity="secondary" text @click="creating = false" />
        </div>
      </form>
    </Dialog>
  </section>
</template>

<style scoped>
.sub {
  margin: 0.2rem 0 0;
  color: var(--p-text-muted-color);
}
.role-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(16rem, 1fr));
  gap: 0.9rem;
}
.role-card {
  cursor: pointer;
  border: 1px solid var(--app-line);
  box-shadow: none;
  transition: box-shadow 0.15s ease;
}
.role-card:hover,
.role-card:focus-visible {
  box-shadow: 0 0 0 1px var(--app-accent);
}
.role-card :deep(.p-card-body) {
  padding: 0.95rem;
}
.card-body {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
}
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
.new-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  min-height: 11rem;
  border: 1px dashed var(--app-line);
  border-radius: 12px;
  background: transparent;
  color: var(--p-text-muted-color);
  font: inherit;
  cursor: pointer;
}
.new-card:hover {
  background: var(--app-soft);
  color: var(--p-text-color);
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
