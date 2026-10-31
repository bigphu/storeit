<script setup lang="ts">
import Avatar from 'primevue/avatar'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Tag from 'primevue/tag'
import { computed, ref, watch } from 'vue'
import SettingsGroup from '@/components/SettingsGroup.vue'
import SettingsRow from '@/components/SettingsRow.vue'
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
  <form v-if="session.me" class="settings" @submit.prevent="save">
    <!-- thẻ người dùng như đầu trang Settings của iOS -->
    <div class="who">
      <Avatar :label="initials" shape="circle" size="xlarge" />
      <div class="who-text">
        <div class="who-name">{{ session.me.account.name }}</div>
        <div class="who-email">{{ session.me.account.email }}</div>
      </div>
      <div class="who-roles">
        <Tag v-for="r in session.me.roles" :key="r.id" :value="r.name" severity="secondary" />
      </div>
    </div>
    <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>

    <SettingsGroup title="Profile">
      <SettingsRow label="Display name" for="profile-name" stacked>
        <InputText id="profile-name" v-model="name" required maxlength="200" fluid />
        <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
      </SettingsRow>
      <SettingsRow label="Email" hint="Used to sign in. Ask an administrator to change it.">
        <span class="muted">{{ session.me.account.email }}</span>
      </SettingsRow>
      <SettingsRow label="Roles" hint="Assigned by an administrator.">
        <Tag v-for="r in session.me.roles" :key="r.id" :value="r.name" severity="secondary" />
        <span v-if="!session.me.roles.length" class="muted">No roles</span>
      </SettingsRow>
    </SettingsGroup>

    <div class="settings-actions">
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
  flex-wrap: wrap;
  gap: 1rem;
  padding: 1rem 1.1rem;
  margin-bottom: 1.6rem;
  border: 1px solid var(--app-hair);
  border-radius: 14px;
  background: var(--app-panel);
  box-shadow: var(--app-card-shadow);
}
.who :deep(.p-avatar) {
  background: linear-gradient(160deg, var(--app-brand), var(--app-brand-strong));
  color: #fff;
  font-weight: 700;
}
.who-text {
  flex: 1;
  min-width: 10rem;
}
.who-name {
  font: 700 1.15rem var(--app-display);
  letter-spacing: -0.02em;
}
.who-email {
  color: var(--p-text-muted-color);
}
.who-roles {
  display: flex;
  gap: 0.35rem;
  flex-wrap: wrap;
}
.muted {
  color: var(--p-text-muted-color);
}
</style>
