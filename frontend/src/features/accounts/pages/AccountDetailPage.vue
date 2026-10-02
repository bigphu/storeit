<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import MultiSelect from 'primevue/multiselect'
import Tag from 'primevue/tag'
import { useConfirm } from 'primevue/useconfirm'
import { computed, ref, watch } from 'vue'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { formatDateTime } from '@/lib/dates'
import { isApiError } from '@/lib/errors'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { useRoles } from '@/features/roles/api'
import {
  useAccount,
  useAssignRoles,
  useDisableAccount,
  useEnableAccount,
  useResendInvitation,
  useSendPasswordReset,
  useUpdateAccount,
} from '../api'
import { statusSeverity } from '../status'

const props = defineProps<{ id: string }>()

const session = useSession()
const confirm = useConfirm()
const canManage = computed(() => session.can(Perm.AccountManage))

const { data: account, refetch } = useAccount(() => props.id)
const roles = useRoles()

// Sửa tên: gửi version đã đọc; 409 là người khác vừa sửa
const name = ref('')
const nameErrors = useFormErrors()
const update = useUpdateAccount()
const roleIds = ref<string[]>([])
watch(
  account,
  (a) => {
    if (!a) return
    name.value = a.name
    roleIds.value = a.roles.map((r) => r.id)
  },
  { immediate: true },
)

async function saveName() {
  if (!account.value) return
  nameErrors.clear()
  try {
    await update.mutateAsync({ id: props.id, name: name.value, version: account.value.version })
    notify.success('Name saved.')
  } catch (err) {
    if (isApiError(err) && err.status === 409) {
      notify.info('Someone else changed this account. Reloaded the latest version.')
      await refetch()
    } else {
      nameErrors.set(err)
    }
  }
}

const assign = useAssignRoles()
async function saveRoles() {
  await assign.mutateAsync({ id: props.id, roleIds: roleIds.value })
  notify.success('Roles saved.')
}

const disable = useDisableAccount()
const enable = useEnableAccount()
const resend = useResendInvitation()
const reset = useSendPasswordReset()

// ask: hỏi lại trước thao tác, chạy xong báo bằng toast; lỗi đã có toast mặc định
function ask(message: string, action: () => Promise<unknown>, done: string) {
  confirm.require({
    message,
    header: 'Confirm',
    acceptLabel: 'Yes',
    rejectLabel: 'Cancel',
    // lỗi đã hiện bằng toast mặc định của mutation
    accept: () =>
      action()
        .then(() => notify.success(done))
        .catch(() => {}),
  })
}
</script>

<template>
  <section v-if="account">
    <div class="page-header">
      <h1>{{ account.name }}</h1>
      <Tag :value="account.status" :severity="statusSeverity(account.status)" />
    </div>
    <dl class="props">
      <dt>Email</dt>
      <dd>{{ account.email }}</dd>
      <dt>Created</dt>
      <dd>{{ formatDateTime(account.created_at) }}</dd>
      <dt>Updated</dt>
      <dd>{{ formatDateTime(account.updated_at) }}</dd>
    </dl>

    <section>
      <h2>Name</h2>
      <form class="form" @submit.prevent="saveName">
        <Message v-if="nameErrors.general.value" severity="error">{{ nameErrors.general.value }}</Message>
        <div class="actions">
          <InputText v-model="name" aria-label="Name" :disabled="!canManage" />
          <Button v-if="canManage" type="submit" label="Save" :loading="update.isPending.value" />
        </div>
        <small v-if="nameErrors.fields.value.name" class="field-error">{{ nameErrors.fields.value.name }}</small>
      </form>
    </section>

    <section>
      <h2>Roles</h2>
      <div class="actions">
        <MultiSelect
          v-model="roleIds"
          :options="roles.data.value ?? []"
          option-label="name"
          option-value="id"
          display="chip"
          placeholder="No roles"
          aria-label="Roles"
          :disabled="!canManage"
        />
        <Button v-if="canManage" label="Save roles" :loading="assign.isPending.value" @click="saveRoles" />
      </div>
    </section>

    <section v-if="canManage">
      <h2>Actions</h2>
      <div class="actions">
        <Button
          v-if="account.status === 'invited'"
          label="Resend invitation"
          severity="secondary"
          @click="ask(`Send a new invitation to ${account.email}? The previous link stops working.`, () => resend.mutateAsync(id), 'Invitation sent.')"
        />
        <Button
          v-if="account.status === 'active'"
          label="Send password reset link"
          severity="secondary"
          @click="ask(`Email a password reset link to ${account.email}?`, () => reset.mutateAsync(id), 'Reset link sent.')"
        />
        <Button
          v-if="account.active"
          label="Disable"
          severity="danger"
          @click="ask(`Disable ${account.name}? They are signed out everywhere.`, () => disable.mutateAsync(id), 'Account disabled.')"
        />
        <Button
          v-else
          label="Enable"
          @click="ask(`Enable ${account.name}?`, () => enable.mutateAsync(id), 'Account enabled.')"
        />
      </div>
    </section>
  </section>
</template>
