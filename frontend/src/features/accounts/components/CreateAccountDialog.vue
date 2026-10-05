<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import MultiSelect from 'primevue/multiselect'
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { useRoles } from '@/features/roles/api'
import { useCreateAccount } from '../api'

const visible = defineModel<boolean>('visible', { required: true })

const router = useRouter()
const errors = useFormErrors()
const roles = useRoles()
const create = useCreateAccount()

const email = ref('')
const name = ref('')
const roleIds = ref<string[]>([])

watch(visible, (open) => {
  if (open) {
    email.value = name.value = ''
    roleIds.value = []
    errors.clear()
  }
})

async function submit() {
  errors.clear()
  try {
    const account = await create.mutateAsync({ email: email.value, name: name.value, role_ids: roleIds.value })
    notify.success(`Invitation sent to ${account.email}.`)
    visible.value = false
    await router.push(`/accounts/${account.id}`)
  } catch (err) {
    errors.set(err)
  }
}
</script>

<template>
  <Dialog v-model:visible="visible" modal header="New account" :style="{ width: '32rem' }">
    <form class="form" @submit.prevent="submit">
      <p>The person gets an email with a link to set their password.</p>
      <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
      <div class="field">
        <label for="new-email">Email</label>
        <InputText id="new-email" v-model="email" type="email" required />
        <small v-if="errors.fields.value.email" class="field-error">{{ errors.fields.value.email }}</small>
      </div>
      <div class="field">
        <label for="new-name">Name</label>
        <InputText id="new-name" v-model="name" required />
        <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
      </div>
      <div class="field">
        <label for="new-roles">Roles</label>
        <MultiSelect
          v-model="roleIds"
          input-id="new-roles"
          :options="roles.data.value ?? []"
          option-label="name"
          option-value="id"
          placeholder="No roles"
          display="chip"
        />
      </div>
      <div class="actions">
        <Button type="submit" label="Create and invite" :loading="create.isPending.value" />
        <Button label="Cancel" severity="secondary" text @click="visible = false" />
      </div>
    </form>
  </Dialog>
</template>
