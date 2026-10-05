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
    items.push({
      key: 'all',
      label: 'All assets',
      icon: 'pi pi-box',
      route: nav.listFor(undefined),
      count: total,
      toggle: true,
      active: name.value === 'assets',
    })
    if (allOpen.value) {
      for (const t of types.value ?? []) {
        items.push({
          key: t.id,
          label: t.name,
          route: nav.listFor(t.id),
          count: t.asset_count,
          sub: true,
        })
      }
    }
    items.push({
      label: 'Configuration',
      items: [
        { label: 'Asset types', icon: 'pi pi-sitemap', route: '/types', active: name.value === 'types' },
        { label: 'Statuses', icon: 'pi pi-tag', route: '/statuses', active: name.value === 'statuses' },
      ],
    })
  }
  const admin: NavItem[] = []
  if (session.can(Perm.AccountRead)) {
    admin.push({
      label: 'Accounts',
      icon: 'pi pi-users',
      route: '/accounts',
      active: name.value.startsWith('account') && name.value !== 'account-settings',
    })
  }
  if (session.can(Perm.RoleRead)) {
    admin.push({
      label: 'Roles',
      icon: 'pi pi-shield',
      route: '/roles',
      active: name.value.startsWith('role'),
    })
  }
  if (admin.length) items.push({ label: 'Administration', items: admin })
  return items
})

const scoped = computed<NavItem[]>(() => {
  const id = nav.scopeTypeId.value!
  const count = types.value?.find((t) => t.id === id)?.asset_count
  return [
    { label: 'Assets', icon: 'pi pi-database', route: nav.listFor(id), count, active: name.value !== 'type-settings' },
    { label: 'Settings', icon: 'pi pi-cog', route: `/types/${id}/settings`, active: name.value === 'type-settings' },
    { label: 'Activity', icon: 'pi pi-history', disabled: true, later: true },
    { label: 'Reports', icon: 'pi pi-chart-bar', disabled: true, later: true },
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
          <i v-if="item.icon" :class="[item.icon, 'nav-icon']" aria-hidden="true" />
          <span class="nav-label">{{ item.label }}</span>
          <span class="nav-count">later</span>
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
            <i v-if="item.icon" :class="[item.icon, 'nav-icon']" aria-hidden="true" />
            <span class="nav-label">{{ item.label }}</span>
            <span v-if="item.count !== undefined" class="nav-count">{{ item.count }}</span>
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
  gap: 0.25rem;
  padding: 0.75rem 0.6rem;
  background: var(--app-ground);
  border-right: 1px solid var(--app-line);
  overflow-y: auto;
}
.nav-menu {
  border: 0;
  background: transparent;
  width: 100%;
  min-width: 0;
  padding: 0;
}
/* nhãn nhóm (Configuration, Administration) kiểu demo */
.nav-menu :deep(.p-menu-submenu-label) {
  font: 500 0.66rem var(--app-mono);
  letter-spacing: 0.09em;
  text-transform: uppercase;
  color: var(--p-text-muted-color);
  padding: 1rem 0.6rem 0.3rem;
  background: transparent;
}
.nav-menu :deep(.p-menu-item-content) {
  border-radius: 7px;
}
.back {
  padding: 0.25rem 0.6rem 0.6rem;
  text-decoration: none;
  font-size: 0.9rem;
}
.switcher {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 0.1rem;
  text-align: left;
  margin-bottom: 0.5rem;
  background: var(--app-ground);
  border-color: var(--app-line);
  border-radius: 9px;
  color: var(--p-text-color);
}
.switcher-label {
  font: 500 0.66rem var(--app-mono);
  letter-spacing: 0.09em;
  text-transform: uppercase;
  color: var(--p-text-muted-color);
}
.switcher-value {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font: 600 1rem var(--app-display);
}
.switcher-value i {
  font-size: 0.75rem;
  color: var(--p-text-muted-color);
}
.nav-link {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  width: 100%;
  border-radius: 7px;
  color: var(--p-text-color);
  text-decoration: none;
}
.nav-icon {
  width: 1.15rem;
  font-size: 0.95rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: var(--p-text-muted-color);
  transition: color 0.15s ease;
}
.nav-link:hover .nav-icon {
  color: var(--p-text-color);
}
/* loại dưới "All assets": thụt vào, có đường dẫn bên trái căn giữa theo icon cha */
.nav-link.is-sub {
  margin-left: 1.15rem;
  padding-left: 0.85rem;
  width: calc(100% - 1.15rem);
  border-left: 1px solid var(--app-line);
  border-radius: 0 7px 7px 0;
  padding-top: 0.35rem;
  padding-bottom: 0.35rem;
}
/* mục đang mở: nền xanh nhạt phẳng, chữ đậm màu thường, icon xanh; không bóng, không vạch */
.nav-link.is-active {
  font-weight: 600;
  background: var(--app-selected);
  color: var(--p-text-color);
}
.nav-link.is-active .nav-icon {
  color: var(--app-accent);
}
.nav-link.is-later {
  color: var(--p-text-muted-color);
  cursor: default;
}
.nav-link.is-later .nav-icon {
  opacity: 0.55;
}
.nav-label {
  flex: 1;
  min-width: 0;
}
.nav-count {
  margin-left: auto;
  font: 0.75rem var(--app-mono);
  color: var(--p-text-muted-color);
  font-variant-numeric: tabular-nums;
}
.nav-link.is-later .nav-count {
  font-family: var(--app-body);
  font-style: italic;
}
.toggle {
  width: 1.4rem;
  height: 1.4rem;
  margin: -0.25rem -0.25rem -0.25rem 0;
  color: var(--p-text-muted-color);
}
</style>