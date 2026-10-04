<script setup lang="ts">
import Avatar from 'primevue/avatar'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Tag from 'primevue/tag'
import { computed, ref, watch } from 'vue'
import { useSession } from '@/lib/auth/session'
import { isApiError } from '@/lib/errors'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { useUpdateMe } from '../api'

// Hồ sơ của tôi: đổi tên hiển thị; email và vai trò do quản trị đổi
const session = useSession()
const errors = useFormErrors()
const update = useUpdateMe()

const name = ref('')
watch(
  () => session.me?.account.name,
  (n) => (name.value = n ?? ''),
  { immediate: true },
)

const initials = computed(() =>
  (session.me?.account.name ?? '')
    .split(/\s+/)
    .filter(Boolean)
    .map((w) => w[0])
    .slice(0, 2)
    .join('')
    .toUpperCase(),
)

async function save() {
  const me = session.me
  if (!me) return
  errors.clear()
  try {
    await update.mutateAsync({ name: name.value, version: me.account.version })
    notify.success('Name saved.')
  } catch (err) {
    if (isApiError(err) && err.status === 409) {
      // tài khoản vừa được sửa ở nơi khác (tab khác, quản trị): nạp lại
      await session.loadMe()
      notify.info('Your account was changed elsewhere. Reloaded it; try again.')
    } else {
      errors.set(err)
    }
  }
}
</script>

<template>
  <form v-if="session.me" class="form" @submit.prevent="save">
    <div class="who">
      <Avatar :label="initials" shape="circle" size="xlarge" />
      <div>
        <div class="who-name">{{ session.me.account.name }}</div>
        <div class="who-email">{{ session.me.account.email }}</div>
      </div>
    </div>
    <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
    <div class="field">
      <label for="profile-name">Display name</label>
      <InputText id="profile-name" v-model="name" required maxlength="200" />
      <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
    </div>
    <div class="field">
      <label for="profile-email">Email</label>
      <InputText id="profile-email" :model-value="session.me.account.email" disabled />
      <small>Used to sign in. Ask an administrator to change it.</small>
    </div>
    <div class="field">
      <span>Roles</span>
      <div class="actions">
        <Tag v-for="r in session.me.roles" :key="r.id" :value="r.name" severity="secondary" />
        <span v-if="!session.me.roles.length">No roles</span>
      </div>
      <small>Roles are assigned by an administrator.</small>
    </div>
    <div class="actions">
      <Button
        type="submit"
        label="Save"
        :loading="update.isPending.value"
        :disabled="!name.trim() || name.trim() === session.me.account.name"
      />
    </div>
  </form>
</template>

<style scoped>
.who {
  display: flex;
  align-items: center;
  gap: 0.9rem;
}
.who :deep(.p-avatar) {
  background: var(--p-highlight-background);
  color: var(--p-highlight-color);
  font-weight: 600;
}
.who-name {
  font-weight: 600;
  font-size: 1.05rem;
}
.who-email {
  color: var(--p-text-muted-color);
}
</style>
