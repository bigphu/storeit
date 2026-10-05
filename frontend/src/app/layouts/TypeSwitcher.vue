<script setup lang="ts">
import Dialog from 'primevue/dialog'
import Listbox, { type ListboxChangeEvent } from 'primevue/listbox'
import { computed, nextTick, ref } from 'vue'
import { useRouter } from 'vue-router'
import { openLocation } from '@/lib/navigation'
import { usePreferences } from '@/lib/preferences'
import { useAssetTypes } from '@/features/asset-types/api'
import { useTypeNav } from '../useTypeNav'

// Bộ chọn loại (Ctrl K, nút ở sidebar): loại gần đây trước; Enter mở trong trang này,
// Ctrl+Enter / Ctrl-click mở tab mới
const visible = defineModel<boolean>('visible', { required: true })

const router = useRouter()
const prefs = usePreferences()
const nav = useTypeNav()
const { data: types } = useAssetTypes(false, true)

interface Option {
  id: string
  name: string
  count?: number
}
const groups = computed(() => {
  const all: Option[] = (types.value ?? []).map((t) => ({ id: t.id, name: t.name, count: t.asset_count }))
  const recent = prefs.prefs.recentTypes.flatMap((id) => all.filter((o) => o.id === id))
  const rest = all.filter((o) => !recent.includes(o))
  return [
    { label: 'Recent', items: recent },
    { label: 'All types', items: rest },
  ].filter((g) => g.items.length)
})

const picked = ref<Option | null>(null)
const wrap = ref<HTMLElement>()

function onShow() {
  picked.value = null
  nextTick(() => wrap.value?.querySelector('input')?.focus())
}

function selectOption(o: Option | null, event?: MouseEvent | KeyboardEvent) {
  if (!o) return
  visible.value = false
  picked.value = null
  openLocation(router, nav.locationFor(o.id), event as MouseEvent)
}

function pick(e: ListboxChangeEvent) {
  selectOption(e.value as Option | null, e.originalEvent as MouseEvent | KeyboardEvent)
}

function onEnterKey(e: KeyboardEvent) {
  if (!visible.value) return
  const focusedEl =
    wrap.value?.querySelector<HTMLElement>('[data-p-focused="true"]') ??
    wrap.value?.querySelector<HTMLElement>('.p-listbox-option')
  if (focusedEl) {
    e.preventDefault()
    focusedEl.click()
  }
}
</script>

<template>
  <Dialog
    v-model:visible="visible"
    modal
    dismissable-mask
    :show-header="false"
    :style="{ width: 'min(92vw, 30rem)', paddingTop: '1rem' }"
    @show="onShow"
  >
    <div ref="wrap" @keydown.enter="onEnterKey">
      <Listbox
        v-model="picked"
        :options="groups"
        option-label="name"
        option-group-label="label"
        option-group-children="items"
        filter
        auto-option-focus
        filter-placeholder="Go to asset type…"
        list-style="max-height: 50vh"
        class="switcher-list"
        @change="pick"
      >
        <template #option="{ option }">
          <span class="opt" @click.stop="selectOption(option, $event)">
            <span>{{ option.name }}</span>
            <span class="count">{{ option.count ?? '' }}</span>
          </span>
        </template>
        <template #footer>
          <small class="hint">Enter opens here · Ctrl+Enter or Ctrl-click opens a new tab</small>
        </template>
      </Listbox>
    </div>
  </Dialog>
</template>

<style scoped>
.switcher-list {
  border: 0;
  width: 100%;
}
.opt {
  display: flex;
  justify-content: space-between;
  width: 100%;
  gap: 1rem;
}
.count {
  color: var(--p-text-muted-color);
  font-variant-numeric: tabular-nums;
}
.hint {
  display: block;
  padding: 0.5rem 0.75rem;
  color: var(--p-text-muted-color);
}
</style>