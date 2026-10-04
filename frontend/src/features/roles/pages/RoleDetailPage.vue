<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Tag from 'primevue/tag'
import Textarea from 'primevue/textarea'
import { useConfirm } from 'primevue/useconfirm'
import { useTabTitle } from '@/app/tabs/tabPage'
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { useDeleteRole, usePermissions, useRole, useSetRolePermissions, useUpdateRole } from '../api'

const props = defineProps<{ id: string }>()

const session = useSession()
const router = useRouter()
const confirm = useConfirm()
const canManage = computed(() => session.can(Perm.RoleManage))

const { data: role } = useRole(() => props.id)
useTabTitle(() => role.value?.name)
const { data: permissions } = usePermissions()

const name = ref('')
const description = ref('')
const selected = ref<string[]>([])
watch(
  role,
  (r) => {
    if (!r) return
    name.value = r.name
    description.value = r.description
    selected.value = [...r.permissions]
  },
  { immediate: true },
)

// Nhóm quyền theo module: "identity.account.read" -> "identity"
const groups = computed(() => {
  const by = new Map<string, { code: string; description: string }[]>()
  for (const p of permissions.value ?? []) {
    const module = p.code.split('.')[0]
    by.set(module, [...(by.get(module) ?? []), p])
  }
  return [...by.entries()].sort(([a], [b]) => a.localeCompare(b))
})

const errors = useFormErrors()
const update = useUpdateRole()
async function saveDetails() {
  errors.clear()
  try {
    // vai trò hệ thống không đổi tên được
    const body = role.value?.is_system ? { description: description.value } : { name: name.value, description: description.value }
    await update.mutateAsync({ id: props.id, ...body })
    notify.success('Role saved.')
  } catch (err) {
    errors.set(err)
  }
}

const setPerms = useSetRolePermissions()
async function savePermissions() {
  await setPerms.mutateAsync({ id: props.id, permissions: selected.value })
  notify.success('Permissions saved.')
}

const remove = useDeleteRole()
function askDelete() {
  confirm.require({
    message: `Delete the role ${role.value?.name}?`,
    header: 'Confirm',
    acceptLabel: 'Delete',
    rejectLabel: 'Cancel',
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
</script>

<template>
  <section v-if="role">
    <div class="page-header">
      <h1>{{ role.name }}</h1>
      <Tag v-if="role.is_system" value="system" severity="secondary" />
    </div>

    <form class="form" @submit.prevent="saveDetails">
      <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
      <div class="field">
        <label for="role-name">Name</label>
        <InputText id="role-name" v-model="name" :disabled="!canManage || role.is_system" />
        <small v-if="role.is_system">Built-in roles can't be renamed.</small>
        <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
      </div>
      <div class="field">
        <label for="role-desc">Description</label>
        <Textarea id="role-desc" v-model="description" rows="3" :disabled="!canManage" />
      </div>
      <div v-if="canManage" class="actions">
        <Button type="submit" label="Save" :loading="update.isPending.value" />
        <Button v-if="!role.is_system" label="Delete role" severity="danger" text @click="askDelete" />
      </div>
    </form>

    <section>
      <h2>Permissions</h2>
      <fieldset v-for="[module, perms] in groups" :key="module" class="perm-group">
        <legend>{{ module }}</legend>
        <div v-for="p in perms" :key="p.code" class="perm">
          <Checkbox v-model="selected" :input-id="p.code" :value="p.code" :disabled="!canManage" />
          <label :for="p.code">
            <code>{{ p.code }}</code>
            — {{ p.description }}
          </label>
        </div>
      </fieldset>
      <div v-if="canManage" class="actions">
        <Button label="Save permissions" :loading="setPerms.isPending.value" @click="savePermissions" />
      </div>
    </section>
  </section>
</template>

<style scoped>
.perm-group {
  margin-bottom: 1rem;
}
.perm {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  margin: 0.25rem 0;
}
</style>
