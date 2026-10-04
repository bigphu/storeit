<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import ContextMenu from 'primevue/contextmenu'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Tag from 'primevue/tag'
import Textarea from 'primevue/textarea'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import type { Role } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { useFormErrors } from '@/lib/forms'
import { openLocation } from '@/lib/navigation'
import { onRowClick, useRowMenu } from '@/lib/tableRows'
import { useCreateRole, useRoles } from '../api'

const session = useSession()
const router = useRouter()
const { data: roles, isFetching } = useRoles()

const creating = ref(false)
const name = ref('')
const description = ref('')
const errors = useFormErrors()
const create = useCreateRole()

function openCreate() {
  name.value = description.value = ''
  errors.clear()
  creating.value = true
}

async function submit() {
  errors.clear()
  try {
    const role = await create.mutateAsync({ name: name.value, description: description.value })
    creating.value = false
    await router.push(`/roles/${role.id}`)
  } catch (err) {
    errors.set(err)
  }
}

// Bấm dòng mở vai trò (Ctrl/⌘ mở tab mới); chuột phải có menu mở
function openRole(r: Role, e?: MouseEvent, newTab?: boolean) {
  openLocation(router, `/roles/${r.id}`, e, newTab)
}
const rowClick = onRowClick(openRole)
const menu = ref<InstanceType<typeof ContextMenu>>()
const { items: menuItems, show: showMenu, clear: clearMenu } = useRowMenu<Role>(menu, (r) => [
  { label: 'Open', icon: 'pi pi-arrow-right', command: () => openRole(r) },
  { label: 'Open in new tab', icon: 'pi pi-external-link', command: () => openRole(r, undefined, true) },
])
</script>

<template>
  <section>
    <div class="page-header">
      <h1>Roles</h1>
      <Button v-if="session.can(Perm.RoleManage)" label="New role" icon="pi pi-plus" @click="openCreate" />
    </div>
    <ContextMenu ref="menu" :model="menuItems" @hide="clearMenu" />
    <DataTable
      :value="roles ?? []"
      :loading="isFetching"
      data-key="id"
      removable-sort
      row-hover
      :row-class="() => 'clickable-row'"
      @row-click="rowClick"
      @row-contextmenu="showMenu"
    >
      <Column header="Name" sort-field="name" sortable>
        <template #body="{ data: r }: { data: Role }">
          <RouterLink :to="`/roles/${r.id}`">{{ r.name }}</RouterLink>
          <Tag v-if="r.is_system" value="system" severity="secondary" class="ml" />
        </template>
      </Column>
      <Column field="description" header="Description" sortable />
      <Column header="Permissions" sort-field="permissions.length" sortable>
        <template #body="{ data: r }: { data: Role }">{{ r.permissions.length }}</template>
      </Column>
      <template #empty>No roles yet.</template>
    </DataTable>

    <Dialog v-model:visible="creating" modal header="New role" :style="{ width: '32rem' }">
      <form class="form" @submit.prevent="submit">
        <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
        <div class="field">
          <label for="role-name">Name</label>
          <InputText id="role-name" v-model="name" required />
          <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
        </div>
        <div class="field">
          <label for="role-desc">Description</label>
          <Textarea id="role-desc" v-model="description" rows="3" />
        </div>
        <div class="actions">
          <Button type="submit" label="Create" :loading="create.isPending.value" />
          <Button label="Cancel" severity="secondary" text @click="creating = false" />
        </div>
      </form>
    </Dialog>
  </section>
</template>

<style scoped>
.ml {
  margin-left: 0.5rem;
}
</style>
