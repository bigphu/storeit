<script setup lang="ts">
import Button from 'primevue/button'
import ContextMenu from 'primevue/contextmenu'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import type { MenuItem } from 'primevue/menuitem'
import Tab from 'primevue/tab'
import TabList from 'primevue/tablist'
import Tabs from 'primevue/tabs'
import { useConfirm } from 'primevue/useconfirm'
import { computed, ref } from 'vue'
import type { Tab as TabItem } from '../tabs/tabList'
import { useTabs } from '../tabs/useTabs'

// Thanh tab trong app: bấm để chuyển (giữ trạng thái trang), bấm giữa hay ✕ để đóng,
// chuột phải để ghim, đổi tên, nhân đôi
const emit = defineEmits<{ switch: [id: string] }>()

const tabs = useTabs()
const confirm = useConfirm()

const label = (t: TabItem) => t.name || t.title || 'Loading…'

// đóng tab còn thay đổi chưa lưu thì hỏi lại
function close(t: TabItem) {
  if (!tabs.dirty.has(t.id)) {
    tabs.close(t.id)
    return
  }
  confirm.require({
    header: 'Close tab?',
    message: `“${label(t)}” has unsaved changes. Closing the tab discards them.`,
    acceptLabel: 'Discard and close',
    rejectLabel: 'Keep editing',
    acceptProps: { severity: 'danger' },
    accept: () => tabs.close(t.id),
  })
}

function onAux(e: MouseEvent, t: TabItem) {
  if (e.button === 1 && !t.pinned) {
    e.preventDefault()
    close(t)
  }
}

// Menu chuột phải
const menu = ref<InstanceType<typeof ContextMenu>>()
const menuTab = ref<TabItem | null>(null)
const menuItems = computed<MenuItem[]>(() => {
  const t = menuTab.value
  if (!t) return []
  return [
    { label: t.pinned ? 'Unpin tab' : 'Pin tab', icon: 'pi pi-thumbtack', command: () => tabs.togglePin(t.id) },
    { label: 'Rename…', icon: 'pi pi-pencil', command: () => startRename(t) },
    { label: 'Duplicate', icon: 'pi pi-clone', command: () => tabs.duplicate(t.id) },
    { separator: true },
    { label: 'Close', icon: 'pi pi-times', disabled: t.pinned, command: () => close(t) },
    { label: 'Close other tabs', icon: 'pi pi-times-circle', command: () => tabs.closeOthers(t.id) },
  ]
})
function openMenu(e: MouseEvent, t: TabItem) {
  menuTab.value = t
  menu.value?.show(e)
}

// Đổi tên tab (để trống thì về tiêu đề của trang)
const renaming = ref<TabItem | null>(null)
const renameText = ref('')
const renameOpen = computed({
  get: () => renaming.value !== null,
  set: (v) => {
    if (!v) renaming.value = null
  },
})
function startRename(t: TabItem) {
  renaming.value = t
  renameText.value = label(t)
}
function commitRename() {
  if (renaming.value) tabs.rename(renaming.value.id, renameText.value)
  renaming.value = null
}
</script>

<template>
  <div class="tabbar">
    <Tabs :value="tabs.activeId ?? ''" scrollable class="tabs" @update:value="(id) => emit('switch', String(id))">
      <TabList>
        <Tab
          v-for="t in tabs.tabs"
          :key="t.id"
          :value="t.id"
          as="div"
          class="app-tab"
          :class="{ pinned: t.pinned, flash: tabs.flashId === t.id }"
          :title="label(t)"
          @auxclick="(e: MouseEvent) => onAux(e, t)"
          @mousedown.middle.prevent
          @contextmenu.prevent="(e: MouseEvent) => openMenu(e, t)"
          @dblclick="startRename(t)"
        >
          <i :class="t.pinned ? 'pi pi-thumbtack' : (t.icon ?? 'pi pi-file')" class="tab-icon" />
          <span class="tab-label">{{ label(t) }}</span>
          <span v-if="tabs.dirty.has(t.id)" class="dirty-dot" title="Unsaved changes" />
          <Button
            v-if="!t.pinned"
            icon="pi pi-times"
            text
            rounded
            size="small"
            class="tab-close"
            :aria-label="`Close ${label(t)}`"
            @click.stop="close(t)"
          />
        </Tab>
      </TabList>
    </Tabs>
    <Button icon="pi pi-plus" text rounded class="new-tab" aria-label="New tab" @click="tabs.open('/assets', { background: false })" />

    <ContextMenu ref="menu" :model="menuItems" @hide="menuTab = null" />
    <Dialog v-model:visible="renameOpen" modal header="Rename tab" :style="{ width: '24rem' }">
      <form class="form" @submit.prevent="commitRename">
        <InputText v-model="renameText" aria-label="Tab name" autofocus />
        <small>Leave empty to use the page's title.</small>
        <div class="actions">
          <Button type="submit" label="Rename" />
          <Button label="Cancel" severity="secondary" text @click="renaming = null" />
        </div>
      </form>
    </Dialog>
  </div>
</template>

<style scoped>
/* Thanh tab kiểu demo: dải nền phụ, tab như tab trình duyệt, tab đang mở nền trang và
   vạch hổ phách phía trên */
.tabbar {
  display: flex;
  align-items: flex-end;
  gap: 0.25rem;
  min-width: 0;
  padding: 0.4rem 0.5rem 0;
  background: var(--app-soft);
  border-bottom: 1px solid var(--app-line);
}
.tabs {
  flex: 1;
  min-width: 0;
}
.tabs :deep(.p-tablist),
.tabs :deep(.p-tablist-content),
.tabs :deep(.p-tablist-tab-list) {
  background: transparent;
  border: 0;
}
.tabs :deep(.p-tablist-active-bar) {
  display: none;
}
.app-tab {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  max-width: 15rem;
  padding: 0.45rem 0.35rem 0.45rem 0.7rem;
  margin-right: 2px;
  border: 1px solid transparent;
  border-bottom: 0;
  border-radius: 8px 8px 0 0;
  background: transparent;
  color: var(--p-text-muted-color);
  font-weight: 500;
}
.app-tab:hover {
  background: color-mix(in srgb, var(--p-content-background) 55%, transparent);
}
/* tab đang mở: khối màu chính đặc, chữ và biểu tượng màu tương phản */
.app-tab.p-tab-active {
  background: var(--p-primary-color);
  border-color: var(--p-primary-color);
  color: var(--p-primary-contrast-color);
  font-weight: 600;
}
.app-tab.p-tab-active .tab-icon,
.app-tab.p-tab-active.pinned .tab-icon {
  color: inherit;
  opacity: 1;
}
.app-tab.p-tab-active .tab-close {
  color: inherit;
}
.app-tab.p-tab-active .tab-close:hover {
  background: color-mix(in srgb, currentColor 18%, transparent);
}
.app-tab.p-tab-active .dirty-dot {
  background: currentColor;
}
.tab-icon {
  font-size: 0.75rem;
  opacity: 0.7;
}
.app-tab.pinned .tab-icon {
  color: var(--app-accent);
  opacity: 1;
}
.tab-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tab-close {
  width: 1.4rem;
  height: 1.4rem;
}
.dirty-dot {
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 50%;
  background: var(--app-accent);
  flex: none;
}
.new-tab {
  margin-bottom: 0.2rem;
}
.app-tab.flash {
  animation: tab-flash 0.9s ease;
}
@keyframes tab-flash {
  30% {
    background: var(--p-highlight-background);
  }
}
@media (prefers-reduced-motion: reduce) {
  .app-tab.flash {
    animation: none;
  }
}
</style>
