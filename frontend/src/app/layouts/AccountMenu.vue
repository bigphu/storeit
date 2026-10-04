<script setup lang="ts">
import Avatar from 'primevue/avatar'
import Button from 'primevue/button'
import type { MenuItem } from 'primevue/menuitem'
import TieredMenu from 'primevue/tieredmenu'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSession } from '@/lib/auth/session'
import { type Theme, usePreferences } from '@/lib/preferences'

// Menu tài khoản ở góc trên: hồ sơ, mật khẩu, tuỳ chọn, theme nhanh, đăng xuất
const session = useSession()
const store = usePreferences()
const router = useRouter()
const menu = ref<InstanceType<typeof TieredMenu>>()

const initials = computed(() =>
  (session.me?.account.name ?? '')
    .split(/\s+/)
    .filter(Boolean)
    .map((w) => w[0])
    .slice(0, 2)
    .join('')
    .toUpperCase(),
)

const themeItem = (label: string, value: Theme): MenuItem => ({
  label,
  icon: store.prefs.theme === value ? 'pi pi-check' : 'pi pi-fw',
  command: () => (store.prefs.theme = value),
})

const items = computed<MenuItem[]>(() => [
  { label: 'Profile', icon: 'pi pi-user', command: () => router.push('/account/profile') },
  { label: 'Change password', icon: 'pi pi-lock', command: () => router.push('/account/password') },
  { label: 'Preferences', icon: 'pi pi-sliders-h', command: () => router.push('/account/preferences') },
  {
    label: 'Theme',
    icon: 'pi pi-palette',
    items: [themeItem('Match my device', 'system'), themeItem('Light', 'light'), themeItem('Dark', 'dark')],
  },
  { separator: true },
  {
    label: 'Sign out',
    icon: 'pi pi-sign-out',
    command: async () => {
      await session.logout()
      await router.push({ name: 'login' })
    },
  },
])
</script>

<template>
  <Button
    text
    class="account-btn"
    aria-haspopup="true"
    aria-controls="account-menu"
    :aria-label="`Account menu for ${session.me?.account.name ?? ''}`"
    @click="(e: MouseEvent) => menu?.toggle(e)"
  >
    <Avatar :label="initials" shape="circle" />
    <span class="account-name">{{ session.me?.account.name }}</span>
    <i class="pi pi-chevron-down" />
  </Button>
  <TieredMenu id="account-menu" ref="menu" :model="items" popup>
    <template #start>
      <div class="who">
        <b>{{ session.me?.account.name }}</b>
        <small>{{ session.me?.account.email }}</small>
      </div>
    </template>
  </TieredMenu>
</template>

<style scoped>
.account-btn {
  gap: 0.5rem;
}
.who {
  display: flex;
  flex-direction: column;
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--p-content-border-color);
}
.who small {
  color: var(--p-text-muted-color);
}
@media (max-width: 640px) {
  .account-name {
    display: none;
  }
}
</style>
