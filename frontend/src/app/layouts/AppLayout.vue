<script setup lang="ts">
import Button from 'primevue/button'
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import NoAccessPage from '../pages/NoAccessPage.vue'

const session = useSession()
const route = useRoute()
const router = useRouter()

// Menu chỉ hiện trang người dùng có quyền xem
const menu = computed(() =>
  [
    { label: 'Assets', to: '/assets', perm: Perm.AssetRead },
    { label: 'Asset types', to: '/asset-types', perm: Perm.AssetRead },
    { label: 'Statuses', to: '/statuses', perm: Perm.AssetRead },
    { label: 'Accounts', to: '/accounts', perm: Perm.AccountRead },
    { label: 'Roles', to: '/roles', perm: Perm.RoleRead },
  ].filter((m) => session.can(m.perm)),
)

const allowed = computed(() => !route.meta.perm || session.can(route.meta.perm))

async function signOut() {
  await session.logout()
  await router.push({ name: 'login' })
}
</script>

<template>
  <div class="shell">
    <nav class="sidebar">
      <strong class="brand">StoreIt</strong>
      <RouterLink v-for="m in menu" :key="m.to" :to="m.to" class="nav-link">{{ m.label }}</RouterLink>
    </nav>
    <div class="main">
      <header class="topbar">
        <span>{{ session.me?.account.name }}</span>
        <RouterLink to="/account/password">Change password</RouterLink>
        <Button label="Sign out" size="small" text @click="signOut" />
      </header>
      <main class="content">
        <RouterView v-if="allowed" />
        <NoAccessPage v-else />
      </main>
    </div>
  </div>
</template>

<style scoped>
.shell {
  display: grid;
  grid-template-columns: 12rem 1fr;
  min-height: 100vh;
}
.sidebar {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 1rem;
  border-right: 1px solid var(--p-content-border-color);
}
.brand {
  margin-bottom: 1rem;
}
.nav-link.router-link-active {
  font-weight: 600;
}
.topbar {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 1rem;
  padding: 0.5rem 1rem;
  border-bottom: 1px solid var(--p-content-border-color);
}
.content {
  padding: 1rem;
  min-width: 0;
}
</style>
