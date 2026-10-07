<script setup lang="ts">
import Button from 'primevue/button'
import ContextMenu from 'primevue/contextmenu'
import InputText from 'primevue/inputtext'
import type { MenuItem } from 'primevue/menuitem'
import Tab from 'primevue/tab'
import TabList from 'primevue/tablist'
import Tabs from 'primevue/tabs'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import FormDialog from '@/components/FormDialog.vue'
import { runAction } from '@/lib/actions'
import { confirmAction } from '@/lib/confirm'
import { useDirty } from '@/lib/forms'
import type { Tab as TabItem } from '../tabs/tabList'
import { useTabs } from '../tabs/useTabs'

// Thanh tab trong app: bấm để chuyển (giữ trạng thái trang), bấm giữa hay ✕ để đóng,
// chuột phải để ghim, đổi tên, nhân đôi
const emit = defineEmits<{ switch: [id: string] }>()

const tabs = useTabs()

// tab chưa có tiêu đề (mở nền, lưu từ bản cũ): tiêu đề của route cho đến khi trang tải
const router = useRouter()
const label = (t: TabItem) => t.name || t.title || router.resolve(t.path).meta.title || 'Untitled'

// đóng tab: có Undo; tab còn thay đổi chưa lưu thì hỏi trước (bỏ thay đổi không hoàn tác được)
async function close(t: TabItem) {
  const name = label(t)
  if (tabs.dirty.has(t.id)) {
    const ok = await confirmAction({
      title: `Close ${name}?`,
      body: 'This tab has unsaved changes. Closing it discards them.',
      action: 'Discard and close',
      danger: true,
      icon: 'alert',
    })
    if (ok) await tabs.close(t.id)
    return
  }
  const index = tabs.tabs.findIndex((x) => x.id === t.id)
  const wasActive = tabs.activeId === t.id
  const snapshot = { ...t }
  await runAction({
    run: () => tabs.close(t.id),
    done: `${name} closed.`,
    undo: () => tabs.reopen(snapshot, index, wasActive),
    undone: `${name} reopened.`,
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
const renameForm = useDirty(() => renameText.value)
const renameOpen = computed({
  get: () => renaming.value !== null,
  set: (v) => {
    if (!v) renaming.value = null
  },
})
function startRename(t: TabItem) {
  renaming.value = t
  renameText.value = label(t)
  renameForm.reset()
}
function commitRename() {
  if (renaming.value) tabs.rename(renaming.value.id, renameText.value)
  renaming.value = null
}
</script>

<template>
  <div class="tabbar">
    <Tabs :value="tabs.activeId ?? ''" scrollable class="tabs app-tabbar" @update:value="(id) => emit('switch', String(id))">
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
    <FormDialog v-model:visible="renameOpen" size="s" icon="file" title="Rename tab" action="Rename" :dirty="renameForm.dirty.value" @submit="commitRename">
      <InputText v-model="renameText" aria-label="Tab name" autofocus />
      <small>Leave empty to use the page's title.</small>
    </FormDialog>
  </div>
</template>

<style scoped>
/* Thanh tab kiểu trình duyệt / VS Code: tab đang mở mang nền của trang và nối liền với
   trang bên dưới, chữ đậm (không vạch màu một bên); tab khác là chữ mờ, ngăn bằng vạch mảnh */
.tabbar {
  display: flex;
  align-items: flex-end;
  gap: 0.25rem;
  min-width: 0;
  padding: 0.35rem 0.5rem 0;
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
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.4rem;
  max-width: 15rem;
  padding: 0.45rem 0.35rem 0.45rem 0.7rem;
  border: 1px solid transparent;
  border-bottom: 0;
  border-radius: 7px 7px 0 0;
  background: transparent;
  color: var(--p-text-muted-color);
  font-weight: 500;
}
.app-tab:hover {
  background: color-mix(in srgb, var(--p-content-background) 55%, transparent);
  color: var(--p-text-color);
}
/* vạch ngăn giữa hai tab không mở */
.app-tab:not(.p-tab-active) + .app-tab:not(.p-tab-active)::before {
  content: '';
  position: absolute;
  left: -1px;
  top: 28%;
  bottom: 28%;
  width: 1px;
  background: var(--app-line);
}
.app-tab.p-tab-active {
  background: var(--p-content-background);
  border-color: var(--app-line);
  color: var(--p-text-color);
  font-weight: 600;
  /* đè lên viền dưới của thanh để nối liền với trang */
  margin-bottom: -1px;
  /* padding-bottom: 0.; */
}
.tab-icon {
  font-size: 0.75rem;
  opacity: 0.7;
}
.app-tab.pinned .tab-icon {
  color: var(--app-accent);
  opacity: 1;
}
.app-tab.pinned {
  padding-right: 0.7rem;
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
