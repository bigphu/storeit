<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { ref, watch } from 'vue'
import type { Role } from '@/lib/api/types'
import { useSession } from '@/lib/auth/session'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { EMPLOYEE_ROLE_ID } from '@/features/roles/catalog'
import { useRoles } from '@/features/roles/api'
import { useCreateAccount } from '../api'

// Mời người dùng: account bắt đầu ở trạng thái invited, nhận link đặt mật khẩu qua email.
// Role giao ngay lúc mời; role có quyền mình không có thì không chọn được (API cũng từ chối).
const visible = defineModel<boolean>('visible', { required: true })

const session = useSession()
const errors = useFormErrors()
const roles = useRoles()
const create = useCreateAccount()

const email = ref('')
const name = ref('')
const roleIds = ref<string[]>([])

const grantable = (r: Role) => r.permissions.every((p) => session.can(p))

watch(visible, (open) => {
  if (open) {
    email.value = name.value = ''
    roleIds.value = [EMPLOYEE_ROLE_ID]
    errors.clear()
  }
})

async function submit() {
  errors.clear()
  try {
    const account = await create.mutateAsync({ email: email.value, name: name.value, role_ids: roleIds.value })
    notify.success(`Invitation sent to ${account.email}.`)
    visible.value = false
  } catch (err) {
    errors.set(err)
  }
}
</script>

<template>
  <Dialog v-model:visible="visible" modal header="Invite account" :style="{ width: '34rem' }">
    <form class="form" @submit.prevent="submit">
      <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
      <div class="field">
        <label for="new-name">Name</label>
        <InputText id="new-name" v-model="name" required maxlength="200" autofocus />
        <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
      </div>
      <div class="field">
        <label for="new-email">Email</label>
        <InputText id="new-email" v-model="email" type="email" required />
        <small v-if="errors.fields.value.email" class="field-error">{{ errors.fields.value.email }}</small>
      </div>
      <fieldset class="field roles">
        <legend>Roles</legend>
        <label v-for="r in roles.data.value ?? []" :key="r.id" class="role-option" :class="{ off: !grantable(r) }">
          <Checkbox v-model="roleIds" :value="r.id" :disabled="!grantable(r)" :input-id="`invite-${r.id}`" />
          <span>
            <strong>{{ r.name }}</strong>
            <small>{{ grantable(r) ? r.description : 'Has permissions you don’t hold yourself.' }}</small>
          </span>
        </label>
      </fieldset>
      <p class="hint">They get an email with a link to set their password. The link works for 72 hours.</p>
      <div class="actions">
        <Button type="submit" label="Send invite" icon="pi pi-envelope" :loading="create.isPending.value" />
        <Button label="Cancel" severity="secondary" text @click="visible = false" />
      </div>
    </form>
  </Dialog>
</template>

<style scoped>
.roles {
  border: 0;
  padding: 0;
  margin: 0;
  gap: 0.4rem;
}
.roles legend {
  padding: 0;
  margin-bottom: 0.3rem;
  font-size: 0.85rem;
  color: var(--p-text-muted-color);
}
.role-option {
  display: flex;
  align-items: flex-start;
  gap: 0.6rem;
  padding: 0.5rem 0.65rem;
  border: 1px solid var(--app-line);
  border-radius: 8px;
  cursor: pointer;
}
.role-option:hover {
  background: var(--p-list-option-focus-background);
}
.role-option.off {
  cursor: not-allowed;
  opacity: 0.6;
}
.role-option span {
  display: flex;
  flex-direction: column;
}
.role-option small {
  color: var(--p-text-muted-color);
}
.hint {
  margin: 0;
  color: var(--p-text-muted-color);
  font-size: 0.875rem;
}
</style>
