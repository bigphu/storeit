<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { setNewTabHandler } from '@/lib/navigation'
import { isTyping } from '@/lib/pageKeys'
import { SHORTCUTS, type Shortcut } from '@/lib/shortcuts'
import EmptyState from '@/components/EmptyState.vue'
import NoAccessPage from '../pages/NoAccessPage.vue'
import { useTabs } from '../tabs/useTabs'
import AccountMenu from './AccountMenu.vue'
import SideDrawer from '@/components/SideDrawer.vue'
import AppSidebar from './AppSidebar.vue'
import TabBar from './TabBar.vue'
import TypeSwitcher from './TypeSwitcher.vue'

const session = useSession()
const route = useRoute()
const router = useRouter()
const tabs = useTabs()

const allowed = computed(() => !route.meta.perm || session.can(route.meta.perm))

// Màn hẹp (điện thoại): sidebar không nằm trên trang (đẩy nội dung xuống và xô lệch khi danh
// sách loại tải xong) mà mở bằng nút ☰ thành ngăn kéo bên trái; chọn trang xong thì đóng
const narrowQuery = typeof window !== 'undefined' ? window.matchMedia('(max-width: 760px)') : null
const narrow = ref(narrowQuery?.matches ?? false)
const onNarrowChange = (e: MediaQueryListEvent) => (narrow.value = e.matches)
narrowQuery?.addEventListener('change', onNarrowChange)
onBeforeUnmount(() => narrowQuery?.removeEventListener('change', onNarrowChange))
const navOpen = ref(false)
watch(
  () => route.fullPath,
  () => (navOpen.value = false),
)
watch(narrow, (n) => {
  if (!n) navOpen.value = false
})
function switchTypeFromNav() {
  navOpen.value = false
  switcherOpen.value = true
}

// Mỗi lần điều hướng: vị trí mới thuộc về tab đang mở
watch(
  () => route.fullPath,
  () => tabs.sync(route.fullPath, route.meta.title, route.meta.icon, route.meta.tab !== false),
  { immediate: true },
)
// /empty khi vẫn còn tab (tải lại trang, nút Back): về tab đang mở hay tab đầu
watch(
  [() => route.name, () => tabs.tabs.length],
  ([name, n]) => {
    if (name !== 'empty' || n === 0) return
    const t = tabs.active ?? tabs.tabs[0]
    if (tabs.activeId === t.id) void router.replace(t.path)
    else void tabs.activate(t.id)
  },
  { immediate: true },
)

// Chuyển tab: giữ vị trí cuộn của từng tab (trang được giữ sống bằng KeepAlive)
const content = ref<HTMLElement>()
const scrolls = new Map<string, number>()
async function switchTo(id: string) {
  if (tabs.activeId) scrolls.set(tabs.activeId, content.value?.scrollTop ?? 0)
  await tabs.activate(id)
  await nextTick()
  if (content.value) content.value.scrollTop = scrolls.get(id) ?? 0
}

// Ctrl/⌘-click hay bấm giữa vào liên kết trong app: mở tab trong app thay vì tab trình
// duyệt. Menu chuột phải của trình duyệt vẫn mở được cửa sổ mới.
function onLinkOpen(e: MouseEvent) {
  const newTab = e.type === 'auxclick' ? e.button === 1 : e.button === 0 && (e.ctrlKey || e.metaKey)
  if (!newTab) return
  const a = (e.target as Element | null)?.closest?.('a[href]') as HTMLAnchorElement | null
  if (!a || a.target === '_blank' || a.hasAttribute('download') || a.origin !== window.location.origin) return
  if (a.pathname.startsWith('/api/')) return
  e.preventDefault()
  e.stopPropagation()
  tabs.open(a.pathname + a.search + a.hash, { background: true })
}

// Phím: Ctrl K bộ chọn loại; Alt 1–9 sang tab thứ n; ? bảng phím tắt
const switcherOpen = ref(false)
const helpOpen = ref(false)
function onKey(e: KeyboardEvent) {
  if (e.key === '?' && !isTyping(e) && !e.ctrlKey && !e.metaKey && !e.altKey) {
    helpOpen.value = !helpOpen.value
    return
  }
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k' && session.can(Perm.AssetRead)) {
    e.preventDefault()
    switcherOpen.value = !switcherOpen.value
    return
  }
  if (e.altKey && !e.ctrlKey && !e.metaKey && /^Digit[1-9]$/.test(e.code)) {
    const t = tabs.tabs[Number(e.code.slice(5)) - 1]
    if (t) {
      e.preventDefault()
      switchTo(t.id)
    }
  }
}

onMounted(() => {
  setNewTabHandler((path) => tabs.open(path, { background: true }))
  window.addEventListener('keydown', onKey)
  document.addEventListener('click', onLinkOpen, true)
  document.addEventListener('auxclick', onLinkOpen, true)
})
onBeforeUnmount(() => {
  setNewTabHandler((path) => window.open(path, '_blank', 'noopener'))
  window.removeEventListener('keydown', onKey)
  document.removeEventListener('click', onLinkOpen, true)
  document.removeEventListener('auxclick', onLinkOpen, true)
})
</script>

<template>
  <div class="shell">
    <header class="topbar">
      <Button v-if="narrow" icon="pi pi-bars" text rounded class="nav-toggle" aria-label="Open navigation" @click="navOpen = true" />
      <RouterLink to="/assets" class="brand">StoreIt</RouterLink>
      <!-- màn hẹp: nút biểu tượng gọn thay cho ô "Go to asset type… Ctrl K" -->
      <Button
        v-if="session.can(Perm.AssetRead) && narrow"
        icon="pi pi-search"
        text
        rounded
        aria-label="Go to asset type"
        title="Go to asset type"
        @click="switcherOpen = true"
      />
      <Button
        v-else-if="session.can(Perm.AssetRead)"
        severity="secondary"
        outlined
        size="small"
        class="go-type"
        @click="switcherOpen = true"
      >
        <span>Go to asset type…</span>
        <kbd>Ctrl K</kbd>
      </Button>
      <Button icon="pi pi-question-circle" text rounded aria-label="Keyboard shortcuts (?)" title="Keyboard shortcuts (?)" @click="helpOpen = true" />
      <span class="spacer" />
      <AccountMenu />
    </header>
    <AppSidebar v-if="!narrow" @switch-type="switcherOpen = true" />
    <SideDrawer
      v-else
      v-model:visible="navOpen"
      position="left"
      width="min(18rem, 85vw)"
      header="StoreIt"
      root-class="nav-drawer"
      content-class="nav-drawer-content"
    >
      <AppSidebar @switch-type="switchTypeFromNav" />
    </SideDrawer>
    <div class="work">
      <TabBar @switch="switchTo" />
      <main ref="content" class="content">
        <!-- Mỗi tab giữ trang của nó (bộ lọc, cuộn, form đang nhập) khi chuyển tab -->
        <EmptyState v-if="!tabs.tabs.length" icon="pi pi-clone" text="No open tabs. Open a page from the sidebar or press Ctrl K." />
        <RouterView v-else v-slot="{ Component, route: r }">
          <KeepAlive :max="12">
            <component :is="Component" v-if="allowed" :key="`${tabs.routeTabId}:${r.matched.at(-1)?.path}`" />
          </KeepAlive>
          <NoAccessPage v-if="!allowed" />
        </RouterView>
      </main>
    </div>
    <TypeSwitcher v-model:visible="switcherOpen" />
    <Dialog v-model:visible="helpOpen" modal header="Keyboard shortcuts" :style="{ width: 'min(92vw, 36rem)' }">
      <DataTable :value="[...SHORTCUTS]" row-group-mode="subheader" group-rows-by="group" size="small" scrollable scroll-height="70vh" class="shortcut-table">
        <template #groupheader="{ data }: { data: Shortcut }">
          <span class="shortcut-group">{{ data.group }}</span>
        </template>
        <Column header="Keys" header-style="width: 11rem">
          <template #body="{ data }: { data: Shortcut }">
            <span class="shortcut-keys"><kbd v-for="k in data.keys" :key="k">{{ k }}</kbd></span>
          </template>
        </Column>
        <Column field="action" header="Action" />
      </DataTable>
    </Dialog>
  </div>
</template>

<style scoped>
.shortcut-group {
  font-weight: 700;
}
.shortcut-keys {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}
/* bảng tra cứu: không tô dòng khi rê chuột */
.shortcut-table :deep(.p-datatable-tbody > tr:hover) {
  background: inherit;
}
.shell {
  display: grid;
  grid-template-columns: 15.5rem minmax(0, 1fr);
  grid-template-rows: auto minmax(0, 1fr);
  height: 100vh;
}
.topbar {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.5rem 1rem;
  background: var(--p-content-background);
  border-bottom: 1px solid var(--app-line);
}
.brand {
  font: 800 1.1rem var(--app-display);
  letter-spacing: -0.01em;
  color: inherit;
  text-decoration: none;
}
/* ô "Go to asset type…" trông như ô tìm kiếm */
.go-type {
  gap: 2rem;
  justify-content: space-between;
  min-width: 15rem;
  background: var(--app-ground);
  border-color: var(--app-line);
  color: var(--p-text-muted-color);
  font-weight: 400;
}
.spacer {
  flex: 1;
}
.nav-toggle {
  flex: none;
  margin-left: -0.5rem;
}
.work {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}
.content {
  flex: 1;
  padding: 1.1rem 1.4rem 2.5rem;
  min-width: 0;
  overflow: auto;
  background: var(--p-content-background);
}
@media (max-width: 760px) {
  /* sidebar ở trong ngăn kéo: chỉ còn thanh trên và phần làm việc */
  .shell {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: auto minmax(0, 1fr);
    height: auto;
    min-height: 100vh;
  }
}
</style>
<style>
/* ngăn kéo điều hướng trên màn hẹp (dựng ngoài cây component): sidebar chiếm hết */
.nav-drawer .p-drawer-header {
  font: 800 1.1rem var(--app-display);
  letter-spacing: -0.01em;
}
.nav-drawer .nav-drawer-content {
  padding: 0;
  display: flex;
}
.nav-drawer .nav-drawer-content > .sidebar {
  flex: 1;
  border-right: 0;
}
</style>
