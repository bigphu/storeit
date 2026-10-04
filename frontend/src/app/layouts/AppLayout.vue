<script setup lang="ts">
import Button from 'primevue/button'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import NoAccessPage from '../pages/NoAccessPage.vue'
import AccountMenu from './AccountMenu.vue'
import AppSidebar from './AppSidebar.vue'
import TypeSwitcher from './TypeSwitcher.vue'

const session = useSession()
const route = useRoute()

const allowed = computed(() => !route.meta.perm || session.can(route.meta.perm))

// Bộ chọn loại: nút trên thanh trên, nút ở sidebar, hoặc Ctrl K ở bất cứ đâu
const switcherOpen = ref(false)
function onKey(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k' && session.can(Perm.AssetRead)) {
    e.preventDefault()
    switcherOpen.value = !switcherOpen.value
  }
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <div class="shell">
    <header class="topbar">
      <RouterLink to="/assets" class="brand">StoreIt</RouterLink>
      <Button
        v-if="session.can(Perm.AssetRead)"
        severity="secondary"
        outlined
        size="small"
        class="go-type"
        @click="switcherOpen = true"
      >
        <span>Go to asset type…</span>
        <kbd>Ctrl K</kbd>
      </Button>
      <span class="spacer" />
      <AccountMenu />
    </header>
    <AppSidebar @switch-type="switcherOpen = true" />
    <main class="content">
      <RouterView v-if="allowed" />
      <NoAccessPage v-else />
    </main>
    <TypeSwitcher v-model:visible="switcherOpen" />
  </div>
</template>

<style scoped>
.shell {
  display: grid;
  grid-template-columns: 15rem minmax(0, 1fr);
  grid-template-rows: auto minmax(0, 1fr);
  height: 100vh;
}
.topbar {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.5rem 1rem;
  border-bottom: 1px solid var(--p-content-border-color);
}
.brand {
  font-weight: 700;
  font-size: 1.1rem;
  color: inherit;
  text-decoration: none;
}
.go-type {
  gap: 1.5rem;
}
.go-type kbd {
  font-size: 0.75rem;
  opacity: 0.7;
}
.spacer {
  flex: 1;
}
.content {
  padding: 1rem;
  min-width: 0;
  overflow: auto;
}
@media (max-width: 760px) {
  .shell {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: auto auto minmax(0, 1fr);
    height: auto;
    min-height: 100vh;
  }
  .go-type span {
    display: none;
  }
}
</style>
