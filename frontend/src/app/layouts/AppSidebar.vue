<script setup lang="ts">
import Button from 'primevue/button'
import Menu from 'primevue/menu'
import type { MenuItem } from 'primevue/menuitem'
import { computed, ref, watch } from 'vue'
import { useRoute, type RouteLocationRaw } from 'vue-router'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { usePreferences } from '@/lib/preferences'
import { useAssetType, useAssetTypes } from '@/features/asset-types/api'
import { useTypeNav } from '../useTypeNav'

// Sidebar hai cấp: cấp workspace (mọi tài sản, cấu hình, quản trị) và cấp loại
// (khi đang ở trong một loại: Assets, Settings, sau này Activity, Reports)
const emit = defineEmits<{ switchType: [] }>()

interface NavItem extends MenuItem {
  route?: RouteLocationRaw
  count?: number
  active?: boolean
  sub?: boolean
  toggle?: boolean
  later?: boolean
}

const session = useSession()
const route = useRoute()
const prefs = usePreferences()
const nav = useTypeNav()
const { data: types } = useAssetTypes(false, true)
const { data: scopeType } = useAssetType(nav.scopeTypeId)

// loại mở gần đây (cho bộ chọn loại)
watch(nav.scopeTypeId, (id) => id && prefs.touchType(id), { immediate: true })

const allOpen = ref(true)
const name = computed(() => String(route.name))

const workspace = computed<NavItem[]>(() => {
  const items: NavItem[] = []
  if (session.can(Perm.AssetRead)) {
    const total = (types.value ?? []).reduce((n, t) => n + (t.asset_count ?? 0), 0)
    items.push({ key: 'all', label: 'All assets', route: nav.listFor(undefined), count: total, toggle: true, active: name.value === 'assets' })
    if (allOpen.value) {
      for (const t of types.value ?? []) {
        items.push({ key: t.id, label: t.name, route: nav.listFor(t.id), count: t.asset_count, sub: true })
      }
    }
    items.push({
      label: 'Configuration',
      items: [
        { label: 'Asset types', route: '/types', active: name.value === 'types' },
        { label: 'Statuses', route: '/statuses', active: name.value === 'statuses' },
      ],
    })
  }
  const admin: NavItem[] = []
  if (session.can(Perm.AccountRead)) admin.push({ label: 'Accounts', route: '/accounts', active: name.value.startsWith('account') && name.value !== 'account-settings' })
  if (session.can(Perm.RoleRead)) admin.push({ label: 'Roles', route: '/roles', active: name.value.startsWith('role') })
  if (admin.length) items.push({ label: 'Administration', items: admin })
  return items
})

const scoped = computed<NavItem[]>(() => {
  const id = nav.scopeTypeId.value!
  const count = types.value?.find((t) => t.id === id)?.asset_count
  return [
    { label: 'Assets', route: nav.listFor(id), count, active: name.value !== 'type-settings' },
    { label: 'Settings', route: `/types/${id}/settings`, active: name.value === 'type-settings' },
    { label: 'Activity', disabled: true, later: true },
    { label: 'Reports', disabled: true, later: true },
  ]
})

const model = computed(() => (nav.scopeTypeId.value ? scoped.value : workspace.value))
</script>

<template>
  <nav class="sidebar" aria-label="Main">
    <template v-if="nav.scopeTypeId.value">
      <RouterLink :to="nav.listFor(undefined)" class="back">← All assets</RouterLink>
      <Button class="switcher" severity="secondary" outlined @click="emit('switchType')">
        <span class="switcher-label">Asset type</span>
        <span class="switcher-value">
          <span>{{ scopeType?.name ?? '…' }}</span>
          <i class="pi pi-chevron-down" />
        </span>
      </Button>
    </template>

    <Menu :model="model" class="nav-menu">
      <template #item="{ item, props }">
        <span v-if="item.later" class="nav-link is-later" v-bind="props.action">
          {{ item.label }} <span class="nav-count">later</span>
        </span>
        <RouterLink v-else-if="item.route" v-slot="{ href, navigate }" :to="item.route" custom>
          <a
            :href="href"
            v-bind="props.action"
            class="nav-link"
            :class="{ 'is-active': item.active, 'is-sub': item.sub }"
            :aria-current="item.active ? 'page' : undefined"
            @click="navigate"
          >
            <Button
              v-if="item.toggle"
              :icon="allOpen ? 'pi pi-chevron-down' : 'pi pi-chevron-right'"
              text
              rounded
              size="small"
              class="toggle"
              :aria-label="allOpen ? 'Hide asset types' : 'Show asset types'"
              :aria-expanded="allOpen"
              @click.stop.prevent="allOpen = !allOpen"
            />
            <span class="nav-label">{{ item.label }}</span>
            <span v-if="item.count !== undefined" class="nav-count">{{ item.count }}</span>
          </a>
        </RouterLink>
      </template>
    </Menu>
  </nav>
</template>

<style scoped>
.sidebar {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.75rem;
  border-right: 1px solid var(--p-content-border-color);
  overflow-y: auto;
}
.nav-menu {
  border: 0;
  background: transparent;
  width: 100%;
  min-width: 0;
}
.back {
  padding: 0.25rem 0.5rem;
}
.switcher {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 0.125rem;
  text-align: left;
}
.switcher-label {
  font-size: 0.75rem;
  color: var(--p-text-muted-color);
}
.switcher-value {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
}
.nav-link {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  width: 100%;
}
.nav-link.is-sub {
  padding-left: 2.25rem;
}
.nav-link.is-active {
  font-weight: 600;
  color: var(--p-primary-color);
}
.nav-link.is-later {
  color: var(--p-text-muted-color);
  cursor: default;
}
.nav-label {
  flex: 1;
  min-width: 0;
}
.nav-count {
  margin-left: auto;
  color: var(--p-text-muted-color);
  font-size: 0.85em;
  font-variant-numeric: tabular-nums;
}
.toggle {
  width: 1.5rem;
  height: 1.5rem;
  margin: -0.25rem 0 -0.25rem -0.25rem;
}
</style>
